package sdm

import "io"

// DownloadImageResult transfers ownership of the image stream to the caller.
// Close Body after reading, and keep the request context alive until then.
type DownloadImageResult struct {
	// Body contains encoded image bytes; the caller owns its close operation.
	Body io.ReadCloser
	// ContentType is the provider's media type, which may be absent.
	ContentType string
	// ContentLength is the provider's byte count, or -1 when unknown.
	ContentLength int64
}

// DownloadClipPreviewResult transfers ownership of the encoded clip to the caller.
// Close Body after reading, and keep the request context alive until then.
type DownloadClipPreviewResult struct {
	// Body contains encoded video bytes; the caller owns its close operation.
	Body io.ReadCloser
	// ContentType is the provider's media type, which may be absent.
	ContentType string
	// ContentLength is the provider's byte count, or -1 when unknown.
	ContentLength int64
}
