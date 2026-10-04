package replay_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type fixture struct {
	Provenance string     `json:"provenance"`
	Exchanges  []exchange `json:"exchanges"`
}

type exchange struct {
	Request  requestExpectation  `json:"request"`
	Response responseExpectation `json:"response"`
}

type requestExpectation struct {
	Method  string      `json:"method"`
	Origin  string      `json:"origin"`
	Path    string      `json:"escapedPath"`
	Query   url.Values  `json:"query"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
}

type responseExpectation struct {
	Status  int         `json:"status"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
}

// pairedTransport never supplies a response for a mismatched request. Failed
// matches remain unconsumed, so cleanup also reports an incomplete transcript.
type pairedTransport struct {
	mu        sync.Mutex
	exchanges []exchange
	next      int
}

func loadTransport(t *testing.T, path string) *pairedTransport {
	t.Helper()

	fixtureRoot, err := os.OpenRoot(".")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := fixtureRoot.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	file, err := fixtureRoot.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := file.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	var recorded fixture

	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()

	err = decoder.Decode(&recorded)
	if err != nil {
		t.Fatal(err)
	}

	if recorded.Provenance != "synthetic" || len(recorded.Exchanges) == 0 {
		t.Fatal("fixture requires explicit synthetic provenance and paired exchanges")
	}

	transport := newPairedTransport(recorded.Exchanges)

	t.Cleanup(func() {
		err := transport.complete()
		if err != nil {
			t.Error(err)
		}
	})

	return transport
}

func (transport *pairedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.mu.Lock()
	defer transport.mu.Unlock()

	if transport.next >= len(transport.exchanges) {
		return nil, fmt.Errorf("%w: unexpected or duplicate request: %s %s", errReplayMismatch, request.Method, request.URL)
	}

	paired := transport.exchanges[transport.next]

	err := matchRequest(request, paired.Request)
	if err != nil {
		return nil, err
	}

	transport.next++

	var response http.Response

	response.StatusCode = paired.Response.Status
	response.Header = paired.Response.Headers.Clone()
	response.Body = io.NopCloser(strings.NewReader(paired.Response.Body))
	response.Request = request

	return &response, nil
}

func (transport *pairedTransport) complete() error {
	transport.mu.Lock()
	defer transport.mu.Unlock()

	if transport.next != len(transport.exchanges) {
		return fmt.Errorf("%w: consumed %d of %d paired exchanges",
			errReplayMismatch, transport.next, len(transport.exchanges))
	}

	return nil
}

func matchRequest(request *http.Request, expected requestExpectation) error {
	if request.Method != expected.Method ||
		request.URL.Scheme+"://"+request.URL.Host != expected.Origin ||
		request.URL.EscapedPath() != expected.Path {
		return fmt.Errorf("%w: request target differs: %s %s", errReplayMismatch, request.Method, request.URL)
	}

	query := request.URL.Query()
	if len(query) != len(expected.Query) {
		return fmt.Errorf("%w: query differs: %v", errReplayMismatch, query)
	}

	for key, values := range expected.Query {
		if !reflect.DeepEqual(query[key], values) {
			return fmt.Errorf("%w: query %s differs: %v", errReplayMismatch, key, query[key])
		}
	}

	for key, values := range expected.Headers {
		if !reflect.DeepEqual(request.Header.Values(key), values) {
			return fmt.Errorf("%w: header %s differs: %v", errReplayMismatch, key, request.Header.Values(key))
		}
	}

	var (
		body []byte
		err  error
	)

	if request.Body != nil {
		body, err = io.ReadAll(request.Body)
		if err != nil {
			return fmt.Errorf("read or decode request: %w", err)
		}
	}

	if strings.Contains(request.Header.Get("Content-Type"), "application/json") && expected.Body != "" {
		var actualValue, expectedValue any

		err = json.Unmarshal(body, &actualValue)
		if err != nil {
			return fmt.Errorf("read or decode request: %w", err)
		}

		err = json.Unmarshal([]byte(expected.Body), &expectedValue)
		if err != nil {
			return fmt.Errorf("read or decode request: %w", err)
		}

		if !reflect.DeepEqual(actualValue, expectedValue) {
			return fmt.Errorf("%w: JSON request body differs: %s", errReplayMismatch, body)
		}
	} else if string(body) != expected.Body {
		return fmt.Errorf("%w: request body differs: %s", errReplayMismatch, body)
	}

	return nil
}

var errReplayMismatch = errors.New("replay mismatch")

func newPairedTransport(exchanges []exchange) *pairedTransport {
	var transport pairedTransport

	transport.exchanges = exchanges

	return &transport
}

func replayHTTPClient(transport http.RoundTripper) *http.Client {
	var client http.Client

	client.Transport = transport

	return &client
}
