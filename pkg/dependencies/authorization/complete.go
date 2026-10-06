package authorization

import (
	"context"
	"errors"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

// Complete validates the callback, exchanges its code with the bound PKCE
// verifier, and completes the mandatory initial devices.list. Invalid callbacks
// may be corrected; a valid callback consumes this attempt even on network failure.
//
//nolint:contextcheck // GO-09: session-owned cancellation additionally cancels the caller-derived completion context.
func (attempt *session) Complete(ctx context.Context,
	request sdm.CompleteAuthorizationRequest,
) (sdm.CompleteAuthorizationResult, error) {
	err := ctx.Err()
	if err != nil {
		return sdm.CompleteAuthorizationResult{}, contextFailure("CompleteAuthorization", err)
	}

	attempt.mu.Lock()
	if attempt.claimed || attempt.ctx.Err() != nil {
		attempt.mu.Unlock()

		return sdm.CompleteAuthorizationResult{}, failure("CompleteAuthorization", sdm.ErrorCanceled, context.Canceled)
	}

	code, err := callbackCode(request.CallbackURL, attempt.request.RedirectUri, attempt.state)

	var invalid *sdm.Error

	if errors.As(err, &invalid) && invalid.Kind == sdm.ErrorInvalidRequest {
		attempt.mu.Unlock()

		return sdm.CompleteAuthorizationResult{}, err
	}

	config, verifier := attempt.request, attempt.verifier
	attempt.claimed = true
	attempt.request.ClientSecret, attempt.verifier = "", ""

	attempt.mu.Unlock()

	defer attempt.cancel()

	if err != nil {
		return sdm.CompleteAuthorizationResult{}, err
	}

	ownedContext, cancel := context.WithCancel(ctx)

	stop := context.AfterFunc(attempt.ctx, cancel)

	defer cancel()
	defer stop()

	return attempt.completeExchange(ownedContext, config, code, verifier)
}

func (attempt *session) completeExchange(ctx context.Context, config sdm.OpenAuthorizationSessionRequest,
	code, verifier string,
) (sdm.CompleteAuthorizationResult, error) {
	token, err := attempt.client.ExchangeToken(ctx, sdm.ExchangeTokenRequest{
		ClientId: config.ClientId, ClientSecret: config.ClientSecret, Code: code,
		RedirectUri: config.RedirectUri, CodeVerifier: &verifier,
	})
	if err != nil {
		return sdm.CompleteAuthorizationResult{}, completionFailure(err)
	}

	devices, err := attempt.client.ListDevices(ctx, sdm.ListDevicesRequest{
		Auth:   sdm.AuthContext{AccessToken: token.Credentials.AccessToken},
		Parent: protocol.CollectionEnterprises + "/" + config.ProjectId, Filter: nil,
	})
	if err != nil {
		return sdm.CompleteAuthorizationResult{}, completionFailure(err)
	}

	err = ctx.Err()
	if err != nil {
		return sdm.CompleteAuthorizationResult{}, contextFailure("AuthorizeAccount", err)
	}

	return sdm.CompleteAuthorizationResult{
		Credentials: token.Credentials, Devices: devices.Devices, AccountIdentity: devices.AccountIdentity,
	}, nil
}

func failure(operation string, kind sdm.ErrorKind, cause error) error {
	return &sdm.Error{Operation: operation, Kind: kind, Cause: cause, StatusCode: 0}
}

func contextFailure(operation string, cause error) error {
	kind := sdm.ErrorCanceled
	if errors.Is(cause, context.DeadlineExceeded) {
		kind = sdm.ErrorTimeout
	}

	return failure(operation, kind, cause)
}

func completionFailure(cause error) error {
	var public *sdm.Error
	if errors.As(cause, &public) {
		return &sdm.Error{Operation: "AuthorizeAccount", Kind: public.Kind, Cause: cause, StatusCode: public.StatusCode}
	}

	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		return contextFailure("AuthorizeAccount", cause)
	}

	return failure("AuthorizeAccount", sdm.ErrorTransport, cause)
}
