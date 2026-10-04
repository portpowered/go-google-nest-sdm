package media_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/media"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestImageTransportFailurePreservesCause(t *testing.T) {
	t.Parallel()

	client, err := media.New(media.WithHTTPClient(doerFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})))
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.DownloadImage(t.Context(), sdm.DownloadImageRequest{
		Auth: sdm.ImageAuthContext{EventToken: testToken}, URL: snapshotURL, Width: nil, Height: nil,
	})
	assertKind(t, err, sdm.ErrorTransport)

	if !errors.Is(err, io.ErrUnexpectedEOF) || result.Body != nil {
		t.Fatalf("failure lost transport cause or returned a body: %v", err)
	}
}

func TestClipOwnership(t *testing.T) {
	t.Parallel()

	body := &trackedBody{Reader: strings.NewReader("clip"), closed: false}

	client, err := media.New(media.WithHTTPClient(doerFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != clipURL ||
			request.Header.Get("Authorization") != "Bearer "+testToken {
			t.Fatalf("unexpected clip request: %s %s", request.Method, request.URL)
		}

		return testResponse(http.StatusOK, body, "video/mp4", 4), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.DownloadClipPreview(t.Context(), sdm.DownloadClipPreviewRequest{
		Auth: sdm.AuthContext{AccessToken: testToken}, URL: clipURL,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Body != body || body.closed || result.ContentType != "video/mp4" || result.ContentLength != 4 {
		t.Fatal("clip ownership or metadata was lost")
	}

	data, err := io.ReadAll(result.Body)
	if err != nil || string(data) != "clip" {
		t.Fatalf("clip data = %q, error = %v", data, err)
	}

	err = result.Body.Close()
	if err != nil || !body.closed {
		t.Fatalf("caller close did not release the clip: %v", err)
	}
}

func TestMissingResponseRejected(t *testing.T) {
	t.Parallel()

	for _, absentResponse := range []bool{true, false} {
		client, err := media.New(media.WithHTTPClient(doerFunc(func(_ *http.Request) (*http.Response, error) {
			if absentResponse {
				//nolint:nilnil // The injected broken transport deliberately violates HTTPDoer's result contract.
				return nil, nil
			}

			var response http.Response

			response.StatusCode = http.StatusOK

			return &response, nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		result, err := client.DownloadClipPreview(t.Context(), sdm.DownloadClipPreviewRequest{
			Auth: sdm.AuthContext{AccessToken: testToken}, URL: clipURL,
		})
		assertKind(t, err, sdm.ErrorInvalidResponse)

		if result.Body != nil {
			t.Fatal("invalid response transferred body ownership")
		}
	}
}
