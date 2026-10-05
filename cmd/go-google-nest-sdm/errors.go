package main

import (
	"errors"
	"fmt"
)

func wrapError(err error) error {
	if err == nil {
		return nil
	}

	var wrapped *commandError
	if errors.As(err, &wrapped) {
		return err
	}

	return &commandError{cause: err}
}

type commandError struct{ cause error }

func (failure *commandError) Error() string { return fmt.Sprintf("CLI operation: %v", failure.cause) }
func (failure *commandError) Unwrap() error { return failure.cause }

func sdkResult[T any](value T, err error) (any, error) {
	if err != nil {
		return nil, wrapError(err)
	}

	return value, nil
}

var (
	errUnexpectedExchange        = errors.New("unexpected exchange")
	errPairedRequestMismatch     = errors.New("paired request mismatch")
	errPairedHeaderMismatch      = errors.New("paired header mismatch")
	errAuthenticationUnsupported = errors.New("client does not support authentication")
	errAuthOperation             = errors.New("unknown authentication operation; use help")
	errCameraOperation           = errors.New("unknown camera operation; use help")
	errCredentialsObject         = errors.New("credentials must contain a JSON object with documented fields")
	errCredentialsTrailingData   = errors.New("credentials must contain exactly one JSON object")
	errEventOperation            = errors.New("unknown event operation; use help")
	errEventsUnsupported         = errors.New("client does not support events")
	errEventCredentials          = errors.New("events pull requires an access token and a subscription --resource")
	errSyntheticOutputFailure    = errors.New("synthetic output failure")
	errFanOperation              = errors.New("unknown fan operation; use help")
	errCommandRequired           = errors.New("expected a command group and operation; use help")
	errUnexpectedArgument        = errors.New("unexpected argument; use documented flags")
	errInvalidTimeout            = errors.New("timeout must be positive")
	errInvalidCount              = errors.New("count must be positive")
	errSharedStdin               = errors.New("credentials and parameters cannot both read stdin")
	errParamsRequired            = errors.New("this command requires --params FILE or --params -")
	errParamsType                = errors.New("parameters must match this command's documented JSON type")
	errParamsTrailingData        = errors.New("parameters must contain exactly one JSON value")
	errAccessTokenRequired       = errors.New("provide SDM_ACCESS_TOKEN or a credential input")
	errResourceRequired          = errors.New("this operation requires --resource")
	errUnknownCommand            = errors.New("unknown command; use help")
	errThermostatOperation       = errors.New("unknown thermostat operation; use help")
	errLoginProjectRequired      = errors.New("auth login requires --project or SDM_PROJECT_ID")
	errLoginRedirect             = errors.New("login requires a registered http loopback URI with a fixed port and path")
	errLoginBrowser              = errors.New("could not open consent in the browser; verify the system browser launcher")
)
