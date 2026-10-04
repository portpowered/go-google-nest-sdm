package sdm

import (
	"context"
	"errors"
	"fmt"
)

var (
	errHandlerRequired = errors.New("event handler is required")
	errContextRequired = errors.New("event context is required")
)

const operationHandleEvent = "handle event"

// EventHandler processes one decoded event synchronously using the caller's context.
// A nil error permits the caller to acknowledge delivery; a returned error leaves
// acknowledgement and redelivery policy with the application.
type EventHandler func(context.Context, EventEnvelope) error

// HandleEvent decodes unwrapped SDM data and invokes the handler without storing
// state or starting a goroutine. Pass a Pub/Sub client's Message.Data directly.
// For wrapped HTTP push, decode the wrapper with Client.DecodePushEvent first.
// The application authenticates HTTP push identity and owns acknowledgement.
func HandleEvent(ctx context.Context, request HandleEventRequest, handler EventHandler) (HandleEventResult, error) {
	if ctx == nil {
		return HandleEventResult{}, &Error{
			Kind: ErrorInvalidRequest, Operation: operationHandleEvent, Cause: errContextRequired, StatusCode: 0,
		}
	}

	contextErr := ctx.Err()
	if contextErr != nil {
		kind := ErrorCanceled
		if errors.Is(contextErr, context.DeadlineExceeded) {
			kind = ErrorTimeout
		}

		return HandleEventResult{}, &Error{Kind: kind, Operation: operationHandleEvent, Cause: contextErr, StatusCode: 0}
	}

	if handler == nil {
		return HandleEventResult{}, &Error{
			Kind: ErrorInvalidRequest, Operation: operationHandleEvent, Cause: errHandlerRequired, StatusCode: 0,
		}
	}

	event, err := DecodeEvent(request.Data)
	if err != nil {
		return HandleEventResult{}, err
	}

	result := HandleEventResult{Event: event}

	handlerErr := handler(ctx, event)
	if handlerErr != nil {
		return result, fmt.Errorf("SDM event handler: %w", handlerErr)
	}

	return result, nil
}
