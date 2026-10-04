package httptransport

import "fmt"

// ErrorKind identifies a transport failure without exposing account credentials.
type ErrorKind string

const (
	ErrorInvalidRequest  ErrorKind = "invalid_request"
	ErrorInvalidResponse ErrorKind = "invalid_response"
	ErrorUnauthorized    ErrorKind = "unauthorized"
	ErrorNotFound        ErrorKind = "not_found"
	ErrorRateLimited     ErrorKind = "rate_limited"
	ErrorServer          ErrorKind = "server"
	ErrorTransport       ErrorKind = "transport"
	ErrorTimeout         ErrorKind = "timeout"
	ErrorCanceled        ErrorKind = "canceled"
)

// Error preserves failure class, HTTP status, and the underlying cause.
// Error never includes request URLs, bodies, or authorization headers.
type Error struct {
	Operation  string
	Kind       ErrorKind
	StatusCode int
	Cause      error
}

// Error describes the operation and class without including sensitive data.
func (failure *Error) Error() string {
	return fmt.Sprintf("sdm %s: %s (HTTP %d)", failure.Operation, failure.Kind, failure.StatusCode)
}

// Unwrap allows errors.Is and errors.As to inspect the underlying cause.
func (failure *Error) Unwrap() error { return failure.Cause }

func fail(operation string, kind ErrorKind, cause error) *Error {
	return &Error{Operation: operation, Kind: kind, StatusCode: 0, Cause: cause}
}
