package replay_test

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/media"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestMediaDownloadReplay(t *testing.T) {
	t.Parallel()

	httpClient := replayHTTPClient(loadTransport(t, "fixtures/synthetic/media-downloads.json"))

	client, err := media.New(media.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	width := 480

	image,
		err := client.DownloadImage(t.Context(),
		sdm.DownloadImageRequest{Auth: sdm.ImageAuthContext{EventToken: "synthetic-event-token"},
			URL:    "https://media.example.invalid/sdm_event_snapshot/synthetic-image",
			Width:  &width,
			Height: nil})
	if err != nil {
		t.Fatal(err)
	}

	assertMediaBody(t, image.Body, image.ContentType, "image/jpeg", "\x00synthetic-image\x01")

	clip,
		err := client.DownloadClipPreview(t.Context(),
		sdm.DownloadClipPreviewRequest{Auth: sdm.AuthContext{AccessToken: accessToken},
			URL: "https://clips.example.invalid/opaque/clip%2Fid?signature=synthetic&item=a&item=b"})
	if err != nil {
		t.Fatal(err)
	}

	assertMediaBody(t, clip.Body, clip.ContentType, "video/mp4", "\x00\x00synthetic-mp4\x01")
	_,
		err = client.DownloadClipPreview(t.Context(),
		sdm.DownloadClipPreviewRequest{Auth: sdm.AuthContext{AccessToken: accessToken},
			URL: "https://clips.example.invalid/expired"})

	var failure *sdm.Error

	if !errors.As(err, &failure) || failure.Kind != sdm.ErrorUnauthorized || failure.StatusCode != http.StatusForbidden {
		t.Fatalf("clip failure: %v", err)
	}
}

func assertMediaBody(t *testing.T, body io.ReadCloser, actualType, expectedType, expectedBytes string) {
	t.Helper()

	actual, err := io.ReadAll(body)

	closeErr := body.Close()

	if err != nil || closeErr != nil || string(actual) != expectedBytes || actualType != expectedType {
		t.Fatalf("media body %q (%s), read %v close %v", actual, actualType, err, closeErr)
	}
}
