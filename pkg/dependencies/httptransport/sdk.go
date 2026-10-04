package httptransport

import (
	"errors"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

// sdkClient adapts dependency exchanges to transport-independent public models.
type sdkClient struct{ transport *Client }

// NewClient creates a stateless SDM client using the configured HTTP transport.
// Credentials remain on each request; the caller owns injected connections.
//
//nolint:ireturn // API-01 exposes the transport-independent client interface.
func NewClient(options ...Option) (sdm.Client, error) {
	transport, err := New(options...)
	if err != nil {
		return nil, publicError(err)
	}

	return &sdkClient{transport: transport}, nil
}

func publicError(err error) error {
	if err == nil {
		return nil
	}

	var publicFailure *sdm.Error
	if errors.As(err, &publicFailure) {
		return err
	}

	var failure *Error
	if errors.As(err, &failure) {
		return &sdm.Error{
			Operation: failure.Operation,
			Kind: sdm.ErrorKind(
				failure.Kind,
			),
			StatusCode: failure.StatusCode,
			Cause:      failure.Cause,
		}
	}

	return &sdm.Error{Operation: "", Kind: sdm.ErrorTransport, StatusCode: 0, Cause: err}
}
