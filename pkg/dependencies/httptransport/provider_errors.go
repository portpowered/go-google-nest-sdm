package httptransport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

var errInvalidProviderError = errors.New("provider error response violated its contract")

func providerFailure(operation string, oauth bool, response *http.Response) error {
	failure := &Error{
		Operation: operation, Kind: statusKind(response.StatusCode),
		StatusCode: response.StatusCode, Cause: errInvalidProviderError,
	}
	if response.Body == nil {
		return failure
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		failure.Cause = err

		return failure
	}

	if len(payload) > maxResponseBytes {
		return failure
	}

	if oauth {
		provider, err := decodeOAuthFailure(payload, response.StatusCode)
		if err != nil {
			return failure
		}

		failure.Cause = provider
		if response.StatusCode == http.StatusBadRequest && unauthorizedOAuthCode(string(provider.Code)) {
			failure.Kind = ErrorUnauthorized
		}

		return failure
	}

	provider, err := decodeGoogleFailure(payload, response.StatusCode)
	if err == nil {
		failure.Cause = provider
	}

	return failure
}

func decodeOAuthFailure(payload []byte, status int) (*sdm.ProviderError, error) {
	err := contracts.Validate("oauth.openapi.yaml", "OAuthErrorResponse", payload)
	if err != nil {
		return nil, errInvalidProviderError
	}

	var result wire.OAuthErrorResponse

	err = json.Unmarshal(payload, &result)
	if err != nil {
		return nil, errInvalidProviderError
	}

	return &sdm.ProviderError{Code: sdm.ProviderErrorCode(result.Error), HTTPStatus: status}, nil
}

func decodeGoogleFailure(payload []byte, status int) (*sdm.ProviderError, error) {
	err := contracts.Validate("errors.openapi.yaml", "GoogleErrorResponse", payload)
	if err != nil {
		return nil, errInvalidProviderError
	}

	var result wire.GoogleErrorResponse

	err = json.Unmarshal(payload, &result)
	if err != nil {
		return nil, errInvalidProviderError
	}

	code := ""
	if result.Error.Status != nil {
		code = string(*result.Error.Status)
	} else if result.Error.Code != nil {
		code = strconv.Itoa(*result.Error.Code)
	}

	return &sdm.ProviderError{Code: sdm.ProviderErrorCode(code), HTTPStatus: status}, nil
}

func unauthorizedOAuthCode(code string) bool {
	switch code {
	case protocol.OAuthErrorInvalidGrant, protocol.OAuthErrorInvalidClient,
		protocol.OAuthErrorUnauthorizedClient, protocol.OAuthErrorAccessDenied:
		return true
	default:
		return false
	}
}
