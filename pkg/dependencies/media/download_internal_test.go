package media

import (
	"errors"
	"net/url"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestDownloadRejectsInvalidConstruction(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatal(err)
	}

	endpoint, err := url.Parse("https://media.example.invalid/clip")
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.download(t.Context(), "DownloadClipPreview", "invalid method", endpoint, "")
	if response != nil && response.Body != nil {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}

	var failure *sdm.Error
	if response != nil || !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidRequest || failure.Cause == nil {
		t.Fatalf("invalid method was not rejected with construction cause: %v", err)
	}
}

func TestDownloadRejectsMissingContext(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatal(err)
	}

	endpoint, err := url.Parse("https://media.example.invalid/clip")
	if err != nil {
		t.Fatal(err)
	}

	//nolint:staticcheck // Deliberately verify the defensive nil-context error rather than issuing a request.
	response, err := client.download(nil, "DownloadClipPreview", "GET", endpoint, "")
	if response != nil && response.Body != nil {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}

	var failure *sdm.Error
	if response != nil || !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidRequest {
		t.Fatalf("nil context did not produce an invalid-request error: %v", err)
	}
}
