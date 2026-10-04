package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

// These paired exchanges are synthetic protocol examples, never device captures.
type pairedExchange struct {
	method          string
	url             string
	token           string
	contentType     string
	body            string
	status          int
	response        string
	query           map[string][]string
	headers         http.Header
	responseHeaders http.Header
}

type fixtureRequest struct {
	Method      string              `json:"method"`
	Origin      string              `json:"origin"`
	EscapedPath string              `json:"escapedPath"`
	Query       map[string][]string `json:"query"`
	Headers     http.Header         `json:"headers"`
	Body        string              `json:"body"`
}
type fixtureResponse struct {
	Status  int         `json:"status"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
}
type fixturePair struct {
	Request  fixtureRequest  `json:"request"`
	Response fixtureResponse `json:"response"`
}
type fixtureFile struct {
	Provenance string        `json:"provenance"`
	Exchanges  []fixturePair `json:"exchanges"`
}

func loadPairs(t *testing.T, name string, indices ...int) []pairedExchange {
	t.Helper()

	fixturePath := filepath.Join("..", "..", "tests", "replay", "fixtures", "synthetic", name+".json")

	data, err := os.ReadFile(fixturePath) // #nosec G304 -- Uses only checked-in synthetic test fixtures.
	if err != nil {
		t.Fatal(err)
	}

	var fixture fixtureFile

	err = json.Unmarshal(data, &fixture)
	if err != nil {
		t.Fatal(err)
	}

	if fixture.Provenance != "synthetic" {
		t.Fatal("expected synthetic provenance")
	}

	pairs := make([]pairedExchange, 0, len(indices))

	for _, index := range indices {
		pair := fixture.Exchanges[index]
		pairs = append(pairs, pairedExchange{
			method: pair.Request.Method, url: pair.Request.Origin + pair.Request.EscapedPath,
			token: pair.Request.Headers.Get("Authorization"), contentType: pair.Request.Headers.Get("Content-Type"),
			body: pair.Request.Body, status: pair.Response.Status, response: pair.Response.Body,
			query: pair.Request.Query, headers: pair.Request.Headers, responseHeaders: pair.Response.Headers,
		})
	}

	return pairs
}

type pairedTransport struct {
	t         *testing.T
	exchanges []pairedExchange
	consumed  int
}

func (transport *pairedTransport) Do(request *http.Request) (*http.Response, error) {
	transport.t.Helper()

	if transport.consumed >= len(transport.exchanges) {
		return nil, errUnexpectedExchange
	}

	expect := transport.exchanges[transport.consumed]

	var (
		body []byte
		err  error
	)

	if request.Body != nil {
		body, err = io.ReadAll(request.Body)
	}

	if err != nil {
		return nil, wrapError(err)
	}

	originPath := request.URL.Scheme + "://" + request.URL.Host + request.URL.EscapedPath()
	if request.Method != expect.method || originPath != expect.url ||
		!reflect.DeepEqual(map[string][]string(request.URL.Query()), expect.query) ||
		request.Header.Get("Authorization") != expect.token || request.Header.Get("Content-Type") != expect.contentType ||
		!matchBody(body, expect.body, expect.contentType) {
		transport.t.Errorf("paired request mismatch at exchange %d", transport.consumed)

		return nil, errPairedRequestMismatch
	}

	for key, values := range expect.headers {
		if !reflect.DeepEqual(request.Header.Values(key), values) {
			return nil, errPairedHeaderMismatch
		}
	}

	transport.consumed++

	return &http.Response{
		StatusCode: expect.status, Header: expect.responseHeaders,
		Body: io.NopCloser(strings.NewReader(expect.response)), Request: request,
		Status: "", Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		ContentLength: int64(len(expect.response)), TransferEncoding: nil,
		Close: false, Uncompressed: false, Trailer: nil, TLS: nil,
	}, nil
}

// JSON object member ordering has no protocol meaning; form data and other
// payloads are matched byte for byte. Every decoded JSON member is compared.
func matchBody(actual []byte, expected, contentType string) bool {
	if contentType != "application/json" {
		return string(actual) == expected
	}

	var left, right any
	if json.Unmarshal(actual, &left) != nil || json.Unmarshal([]byte(expected), &right) != nil {
		return false
	}

	return reflect.DeepEqual(left, right)
}

func newTestApplication(
	t *testing.T, exchanges []pairedExchange, input string,
) (application, *bytes.Buffer) {
	t.Helper()
	transport := &pairedTransport{t: t, exchanges: exchanges, consumed: 0}

	client, err := httptransport.NewClient(httptransport.WithHTTPClient(transport))
	if err != nil {
		t.Fatal(err)
	}

	output := &bytes.Buffer{}
	lookup := func(key string) string {
		return map[string]string{
			"SDM_ACCESS_TOKEN": syntheticAccessToken, "SDM_CLIENT_ID": "synthetic-client",
			"SDM_CLIENT_SECRET": "synthetic-secret", "SDM_AUTHORIZATION_CODE": "synthetic-code",
			"SDM_REFRESH_TOKEN": syntheticRefreshToken, "SDM_REDIRECT_URI": "https://example.invalid/callback",
		}[key]
	}

	t.Cleanup(func() {
		if transport.consumed != len(exchanges) {
			t.Errorf("consumed %d of %d exchanges", transport.consumed, len(exchanges))
		}
	})

	return application{client: client, in: strings.NewReader(input), out: output, lookup: lookup}, output
}

func TestListDevicesPairedReplay(t *testing.T) {
	t.Parallel()

	app, output := newTestApplication(t, loadPairs(t, "rest-resources", 0), "")

	err := app.run(context.Background(), []string{devicesGroup, listOperation, resourceFlag, syntheticEnterprise})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output.String(), "synthetic-device") || strings.Contains(output.String(), "synthetic-access") {
		t.Fatalf("unexpected result: %s", output)
	}
}

func TestExchangeOutputRedactsCredentials(t *testing.T) {
	t.Parallel()

	for _, export := range []bool{false, true} {
		args := []string{authGroup, "exchange"}
		if export {
			args = append(args, exportFlag)
		}

		app, output := newTestApplication(t, loadPairs(t, "oauth", 0), "")

		err := app.run(context.Background(), args)
		if err != nil {
			t.Fatal(err)
		}

		if strings.Contains(output.String(), syntheticAccessToken) != export ||
			strings.Contains(output.String(), syntheticRefreshToken) != export {
			t.Fatalf("credential export mismatch: %s", output)
		}
	}
}

func TestAuthenticationErrorDoesNotExposeResponse(t *testing.T) {
	t.Parallel()
	app, _ := newTestApplication(t, loadPairs(t, "provider-error", 0), "")

	err := app.run(context.Background(), []string{
		devicesGroup, getOperation, resourceFlag, syntheticDevice,
	})
	if err == nil || strings.Contains(err.Error(), "Synthetic permission denied") ||
		strings.Contains(err.Error(), "synthetic-access") {
		t.Fatalf("unsafe or missing error: %v", err)
	}
}

func TestTokenExchangeAuthenticationError(t *testing.T) {
	t.Parallel()
	app, _ := newTestApplication(t, loadPairs(t, "oauth-error", 0), "")
	err := app.run(context.Background(), []string{authGroup, "exchange"})

	var provider *sdm.Error

	if !errors.As(err, &provider) || provider.Kind != sdm.ErrorUnauthorized {
		t.Fatalf("authentication class: %v", err)
	}

	output := describeError(err)
	if output.Kind != sdm.ErrorUnauthorized || output.StatusCode != http.StatusBadRequest ||
		strings.Contains(output.Error, "synthetic secret") {
		t.Fatalf("unsafe or unclassified JSON error: %+v", output)
	}
}

func TestEventPullAcknowledgesAfterOutput(t *testing.T) {
	t.Parallel()

	app, output := newTestApplication(t, loadPairs(t, pubSubFixture, 0, 2), "")

	err := app.run(context.Background(), []string{
		eventsGroup, pullOperation, resourceFlag, syntheticSubscription,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output.String(), "synthetic-envelope-event") ||
		strings.Contains(output.String(), "synthetic-ack") {
		t.Fatalf("unexpected event output: %s", output)
	}
}
