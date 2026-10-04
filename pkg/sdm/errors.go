package sdm

import "fmt"

// ErrorKind is a stable failure class suitable for errors.As checks.
type ErrorKind string

const (
	// ErrorInvalidRequest means local validation rejected the input before transmission.
	ErrorInvalidRequest ErrorKind = "invalid_request"
	// ErrorInvalidResponse means a known response payload violated its contract.
	ErrorInvalidResponse ErrorKind = "invalid_response"
	// ErrorUnauthorized means the account lacks valid credentials or authorization.
	ErrorUnauthorized ErrorKind = "unauthorized"
	// ErrorNotFound means the requested resource does not exist or is inaccessible.
	ErrorNotFound ErrorKind = "not_found"
	// ErrorRateLimited means the service rejected the request due to quota or rate limits.
	ErrorRateLimited ErrorKind = "rate_limited"
	// ErrorServer means the service could not complete the request.
	ErrorServer ErrorKind = "server"
	// ErrorTransport means the connection failed; command completion may be uncertain.
	ErrorTransport ErrorKind = "transport"
	// ErrorTimeout means the operation exceeded its deadline.
	ErrorTimeout ErrorKind = "timeout"
	// ErrorCanceled means the caller canceled the operation.
	ErrorCanceled ErrorKind = "canceled"
	// ErrorUnsupported means the requested capability is absent in the supplied snapshot.
	ErrorUnsupported ErrorKind = "unsupported"
)

// Error describes a client failure and preserves its underlying cause.
type Error struct {
	// Kind identifies the failure class.
	Kind ErrorKind
	// Operation identifies the attempted client operation.
	Operation string
	// Cause retains the validation, connection, or service error.
	Cause error
	// StatusCode is the HTTP response status, or zero when no response was received.
	StatusCode int
}

// Error returns a diagnostic description without including account credentials.
func (e *Error) Error() string {
	return fmt.Sprintf("sdm %s: %s", e.Operation, e.Kind)
}

// Unwrap exposes the original cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Cause }

func invalidResponse(operation string, cause error) error {
	return &Error{Kind: ErrorInvalidResponse, Operation: operation, Cause: cause, StatusCode: 0}
}
