package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
)

const maxResponseBytes = 8 << 20

func (client *Client) exchange(
	ctx context.Context, operation, method, endpoint, token, contentType string,
	body io.Reader, result any,
) error {
	if ctx == nil {
		return fail(operation, ErrorInvalidRequest, nil)
	}

	if method != protocol.MethodOAuthToken || endpoint != client.oauthBaseURL+protocol.PathOAuthToken {
		if token == "" || strings.ContainsAny(token, "\r\n") {
			return fail(operation, ErrorInvalidRequest, nil)
		}
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fail(operation, ErrorInvalidRequest, err)
	}

	request.Header.Set(protocol.HeaderAccept, protocol.MIMEApplicationJSON)

	if token != "" {
		request.Header.Set(protocol.HeaderAuthorization, protocol.BearerPrefix+token)
	}

	if contentType != "" {
		request.Header.Set(protocol.HeaderContentType, contentType)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}

		return fail(operation, classifyCause(err), err)
	}

	if response == nil {
		return fail(operation, ErrorInvalidResponse, nil)
	}

	if response.Body != nil {
		defer func() { _ = response.Body.Close() }()
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return providerFailure(operation, endpoint == client.oauthBaseURL+protocol.PathOAuthToken, response)
	}

	if response.Body == nil {
		return fail(operation, ErrorInvalidResponse, nil)
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fail(operation, classifyCause(err), err)
	}

	if len(payload) > maxResponseBytes {
		return fail(operation, ErrorInvalidResponse, nil)
	}

	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fail(operation, ErrorInvalidResponse, nil)
	}

	err = validateDependencyResponse(payload, result)
	if err != nil {
		return fail(operation, ErrorInvalidResponse, err)
	}

	return decodeJSON(operation, payload, result)
}

func classifyCause(err error) ErrorKind {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTimeout
	}

	if errors.Is(err, context.Canceled) {
		return ErrorCanceled
	}

	return ErrorTransport
}

func statusKind(status int) ErrorKind {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrorUnauthorized
	case http.StatusNotFound:
		return ErrorNotFound
	case http.StatusTooManyRequests:
		return ErrorRateLimited
	case http.StatusBadRequest:
		return ErrorRejected
	default:
		if status >= http.StatusInternalServerError {
			return ErrorServer
		}

		return ErrorInvalidResponse
	}
}

//nolint:unparam // Each generated method stays paired with its schema route at the call site.
func (client *Client) exchangeJSON(
	ctx context.Context, operation, method, endpoint, token string, input, result any,
) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fail(operation, ErrorInvalidRequest, err)
	}

	return client.exchange(ctx, operation, method, endpoint, token,
		protocol.MIMEApplicationJSON, bytes.NewReader(body), result)
}

func decodeJSON(operation string, payload []byte, result any) error {
	err := json.Unmarshal(payload, result)
	if err != nil {
		return fail(operation, ErrorInvalidResponse, err)
	}

	return nil
}
