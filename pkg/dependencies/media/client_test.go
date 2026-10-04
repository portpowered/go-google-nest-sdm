package media_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/media"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const testToken = "token"
const clipURL = "https://media.example.invalid/clip"
const snapshotURL = "https://media.example.invalid/sdm_event_snapshot/image"

type doerFunc func(*http.Request) (*http.Response, error)

func (doer doerFunc) Do(request *http.Request) (*http.Response, error) { return doer(request) }

type trackedBody struct {
	io.Reader

	closed bool
}

func testResponse(status int, body io.ReadCloser, contentType string, length int64) *http.Response {
	var response http.Response

	response.StatusCode = status
	response.Body = body
	response.Header = http.Header{"Content-Type": {contentType}}
	response.ContentLength = length

	return &response
}

func (body *trackedBody) Close() error {
	body.closed = true

	return nil
}

func assertKind(t *testing.T, err error, kind sdm.ErrorKind) {
	t.Helper()

	var failure *sdm.Error
	if !errors.As(err, &failure) || failure.Kind != kind {
		t.Fatalf("error = %v; want kind %s", err, kind)
	}
}

func TestImageOwnership(t *testing.T) {
	t.Parallel()

	body := &trackedBody{Reader: strings.NewReader("image"), closed: false}

	client, err := media.New(media.WithHTTPClient(doerFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != snapshotURL+"?height=360&width=480" ||
			request.Header.Get("Authorization") != "Basic event-token" {
			t.Fatalf("unexpected request: %s %s headers %v", request.Method, request.URL, request.Header)
		}

		return testResponse(http.StatusOK, body, "image/jpeg", 5), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	width, height := 480, 360

	result, err := client.DownloadImage(t.Context(), sdm.DownloadImageRequest{
		Auth: sdm.ImageAuthContext{EventToken: "event-token"},
		URL:  snapshotURL, Width: &width, Height: &height,
	})
	if err != nil {
		t.Fatal(err)
	}

	if body.closed || result.ContentType != "image/jpeg" || result.ContentLength != 5 {
		t.Fatal("result lost body ownership or metadata")
	}

	data, err := io.ReadAll(result.Body)
	if err != nil || string(data) != "image" {
		t.Fatalf("body: %q, %v", data, err)
	}

	err = result.Body.Close()

	if err != nil || !body.closed {
		t.Fatal("caller close did not reach body")
	}
}

func TestInputRejectedBeforeNetwork(t *testing.T) {
	t.Parallel()

	client, err := media.New(media.WithHTTPClient(doerFunc(func(_ *http.Request) (*http.Response, error) {
		t.Fatal("invalid request reached transport")

		return nil, io.ErrUnexpectedEOF
	})))
	if err != nil {
		t.Fatal(err)
	}

	for _, endpoint := range []string{
		"http://media.example.invalid/sdm_event_snapshot/image",
		"https://user:pass@media.example.invalid/sdm_event_snapshot/image",
		snapshotURL + "#fragment",
		"https://media.example.invalid/other/image",
		":invalid",
	} {
		_, err = client.DownloadImage(t.Context(), sdm.DownloadImageRequest{
			Auth: sdm.ImageAuthContext{EventToken: testToken},
			URL:  endpoint, Width: nil, Height: nil,
		})
		assertKind(t, err, sdm.ErrorInvalidRequest)
	}

	zero := 0
	_, err = client.DownloadImage(t.Context(), sdm.DownloadImageRequest{
		Auth: sdm.ImageAuthContext{EventToken: testToken},
		URL:  snapshotURL, Width: &zero, Height: nil,
	})
	assertKind(t, err, sdm.ErrorInvalidRequest)

	width := 480
	_, err = client.DownloadImage(t.Context(), sdm.DownloadImageRequest{
		Auth: sdm.ImageAuthContext{EventToken: testToken},
		URL:  snapshotURL + "?signature=%invalid", Width: &width, Height: nil,
	})
	assertKind(t, err, sdm.ErrorInvalidRequest)
	_, err = client.DownloadClipPreview(t.Context(), sdm.DownloadClipPreviewRequest{
		Auth: sdm.AuthContext{AccessToken: "token\r\n"},
		URL:  clipURL,
	})
	assertKind(t, err, sdm.ErrorInvalidRequest)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = client.DownloadClipPreview(ctx, sdm.DownloadClipPreviewRequest{
		Auth: sdm.AuthContext{AccessToken: testToken},
		URL:  clipURL,
	})
	assertKind(t, err, sdm.ErrorCanceled)
}

func TestStatusFailureClosesBody(t *testing.T) {
	t.Parallel()

	for _, status := range []int{302, 401, 404, 410, 429, 500} {
		body := &trackedBody{Reader: strings.NewReader("secret provider diagnostics"), closed: false}

		client, err := media.New(media.WithHTTPClient(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return testResponse(status, body, "", 0), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.DownloadClipPreview(t.Context(), sdm.DownloadClipPreviewRequest{
			Auth: sdm.AuthContext{AccessToken: "secret-token"},
			URL:  "https://media.example.invalid/clip?signature=secret",
		})
		if err == nil || !body.closed || strings.Contains(err.Error(), "secret") {
			t.Fatalf("status %d failed safety: %v", status, err)
		}
	}
}

func TestRedirectNeverCarriesCredentials(t *testing.T) {
	t.Parallel()

	var requests int

	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++

		if request.URL.Path != "/clip" {
			t.Error("redirect was followed")
		}

		http.Redirect(response, request, "/unintended", http.StatusFound)
	}))
	defer server.Close()

	client, err := media.New(media.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DownloadClipPreview(t.Context(), sdm.DownloadClipPreviewRequest{
		Auth: sdm.AuthContext{AccessToken: testToken},
		URL:  server.URL + "/clip",
	})
	assertKind(t, err, sdm.ErrorInvalidResponse)

	if requests != 1 {
		t.Fatalf("made %d requests", requests)
	}
}

func TestFailureCauses(t *testing.T) {
	t.Parallel()

	for _, cause := range []error{context.Canceled, context.DeadlineExceeded, io.ErrUnexpectedEOF} {
		body := &trackedBody{Reader: strings.NewReader(""), closed: false}

		client, err := media.New(media.WithHTTPClient(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return testResponse(0, body, "", 0), cause
		})))
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.DownloadClipPreview(t.Context(), sdm.DownloadClipPreviewRequest{
			Auth: sdm.AuthContext{AccessToken: testToken},
			URL:  clipURL,
		})
		if !errors.Is(err, cause) || !body.closed {
			t.Fatalf("lost cause/body: %v", err)
		}
	}

	for _, option := range []media.Option{nil, media.WithHTTPClient(nil), media.WithHTTPClient((*http.Client)(nil))} {
		_, err := media.New(option)
		assertKind(t, err, sdm.ErrorInvalidRequest)
	}
}
