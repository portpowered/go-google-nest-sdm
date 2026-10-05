package httptransport_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"reflect"
	"strings"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
)

type cookieRoundTripper func(*http.Request) (*http.Response, error)

func (send cookieRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return send(request)
}

func TestHTTPClientRejectsCookieJar(t *testing.T) {
	t.Parallel()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	var standard http.Client

	standard.Jar = jar
	_, err = transport.New(transport.WithHTTPClient(&standard))

	var failure *transport.Error

	if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidRequest ||
		!errors.Is(err, transport.ErrCookieJarUnsupported) {
		t.Fatalf("missing typed cookie rejection: %v", err)
	}

	var absent *http.Client

	_, err = transport.New(transport.WithHTTPClient(absent))
	if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidRequest {
		t.Fatalf("nil HTTP client: %v", err)
	}
}

func TestHTTPClientIsolatesTwoAccounts(t *testing.T) {
	t.Parallel()

	calls := 0
	expected := []string{"account-a", "account-b"}

	var standard http.Client

	standard.Transport = cookieRoundTripper(func(request *http.Request) (*http.Response, error) {
		if calls >= len(expected) {
			t.Fatal("unexpected extra exchange")
		}

		if request.Method != http.MethodGet ||
			request.URL.String() != "https://smartdevicemanagement.googleapis.com/v1/enterprises/e/devices/d" ||
			request.Body != nil ||
			!reflect.DeepEqual(request.Header, http.Header{
				"Authorization": {"Bearer " + expected[calls]}, "Accept": {"application/json"},
			}) {
			t.Fatalf("unexpected account exchange %d: %s %s %v", calls, request.Method, request.URL, request.Header)
		}

		calls++

		var response http.Response

		response.StatusCode = http.StatusOK
		response.Header = http.Header{
			"Content-Type": {"application/json"}, "Set-Cookie": {"session=account-a; Path=/; Secure"},
		}
		response.Body = io.NopCloser(strings.NewReader(`{"name":"enterprises/e/devices/d"}`))

		return &response, nil
	})

	client, err := transport.New(transport.WithHTTPClient(&standard))
	if err != nil {
		t.Fatal(err)
	}
	// Caller mutation must not add a cookie jar to the already-configured client.
	standard.Jar, err = cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, account := range expected {
		result, callErr := client.GetDevice(t.Context(), account, "enterprises/e/devices/d")
		if callErr != nil || result.Name != "enterprises/e/devices/d" {
			t.Fatalf("account exchange: %+v %v", result, callErr)
		}
	}

	if calls != len(expected) {
		t.Fatalf("consumed %d exchanges", calls)
	}
}
