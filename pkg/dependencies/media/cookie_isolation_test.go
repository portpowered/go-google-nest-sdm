package media_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/media"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const cookieImageMIME = "image/jpeg"

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
	_, err = media.New(media.WithHTTPClient(&standard))

	var failure *sdm.Error

	if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidRequest ||
		!errors.Is(err, media.ErrCookieJarUnsupported) {
		t.Fatalf("missing typed cookie rejection: %v", err)
	}
}

func TestHTTPClientIsolatesTwoAccounts(t *testing.T) {
	t.Parallel()

	calls := 0
	expected := []string{"event-account-a", "event-account-b"}

	var standard http.Client

	standard.Transport = cookieRoundTripper(func(request *http.Request) (*http.Response, error) {
		if calls >= len(expected) {
			t.Fatal("unexpected extra exchange")
		}

		if request.Method != http.MethodGet ||
			request.URL.String() != snapshotURL ||
			request.Body != nil ||
			!reflect.DeepEqual(request.Header, http.Header{
				"Authorization": {"Basic " + expected[calls]},
			}) {
			t.Fatalf("unexpected account exchange %d: %s %s %v", calls, request.Method, request.URL, request.Header)
		}

		calls++

		var response http.Response

		response.StatusCode = http.StatusOK
		response.Header = http.Header{"Content-Type": {cookieImageMIME}, "Set-Cookie": {"session=account-a; Path=/; Secure"}}
		response.Body = io.NopCloser(strings.NewReader("synthetic-image"))

		return &response, nil
	})

	client, err := media.New(media.WithHTTPClient(&standard))
	if err != nil {
		t.Fatal(err)
	}

	standard.Jar, err = cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, account := range expected {
		result, callErr := client.DownloadImage(t.Context(), sdm.DownloadImageRequest{
			Auth: sdm.ImageAuthContext{EventToken: account}, URL: snapshotURL, Width: nil, Height: nil,
		})
		if callErr != nil {
			t.Fatal(callErr)
		}

		data, readErr := io.ReadAll(result.Body)

		closeErr := result.Body.Close()

		if readErr != nil || closeErr != nil || string(data) != "synthetic-image" || result.ContentType != cookieImageMIME {
			t.Fatalf("account response: %q %v %v", data, readErr, closeErr)
		}
	}

	if calls != len(expected) {
		t.Fatalf("consumed %d exchanges", calls)
	}
}
