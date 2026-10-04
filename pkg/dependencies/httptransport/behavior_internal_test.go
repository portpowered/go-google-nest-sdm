package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestResourceListEnvelopeValidation(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{`{broken`, `null`, `{"devices":true}`} {
		_, err := decodeResourceList([]byte(payload), protocol.KeyDevices, "ListDevices", sdm.DecodeDevice)

		var failure *sdm.Error

		if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse {
			t.Fatalf("accepted invalid envelope %s: %v", payload, err)
		}
	}
}

func TestPublicErrorPreservesExistingTypedErrorAndCause(t *testing.T) {
	t.Parallel()

	existing := &sdm.Error{Operation: "caller", Kind: sdm.ErrorCanceled, StatusCode: 0, Cause: context.Canceled}
	if !errors.Is(publicError(existing), existing) {
		t.Fatal("existing typed error was replaced")
	}

	result := publicError(io.ErrUnexpectedEOF)

	var failure *sdm.Error

	if !errors.As(result, &failure) || failure.Kind != sdm.ErrorTransport || !errors.Is(result, io.ErrUnexpectedEOF) {
		t.Fatalf("lost cause: %v", result)
	}
}

func TestExchangeRejectsInvalidContextAndEndpoint(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatal(err)
	}

	var payload json.RawMessage
	//nolint:staticcheck // Verify the explicit public invalid-context contract without sending.
	err = client.exchange(nil, "invalid", protocol.MethodGetDevice, client.sdmBaseURL, "token", "", nil, &payload)

	var failure *Error

	if !errors.As(err, &failure) || failure.Kind != ErrorInvalidRequest {
		t.Fatalf("nil context: %v", err)
	}

	err = client.exchange(context.Background(), "invalid", protocol.MethodGetDevice, "%", "token", "", nil, &payload)
	if !errors.As(err, &failure) || failure.Kind != ErrorInvalidRequest || failure.Cause == nil {
		t.Fatalf("invalid endpoint: %v", err)
	}
}
