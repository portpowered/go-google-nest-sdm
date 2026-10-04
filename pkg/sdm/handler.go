package sdm

import (
	"context"
	"errors"
)

// EventHandler processes one decoded event synchronously using the caller's context.
// A nil error permits the caller to acknowledge delivery; a returned error leaves
// acknowledgement and redelivery policy with the application.
type EventHandler func(context.Context, EventEnvelope) error

// HandleEvent decodes unwrapped SDM data and invokes the handler without storing
// state or starting a goroutine. Pass a Pub/Sub client's Message.Data directly.
// For wrapped HTTP push, decode the wrapper with Client.DecodePushEvent first.
// The application authenticates HTTP push identity and owns acknowledgement.
func HandleEvent(ctx context.Context, request HandleEventRequest, handler EventHandler) (HandleEventResult, error) {
	if err := ctx.Err(); err != nil {
		return HandleEventResult{}, err
	}
	if handler == nil {
		return HandleEventResult{}, &Error{Kind: ErrorInvalidRequest, Operation: "handle event", Cause: errors.New("event handler is required")}
	}
	event, err := DecodeEvent(request.Data)
	if err != nil {
		return HandleEventResult{}, err
	}
	result := HandleEventResult{Event: event}
	if err := handler(ctx, event); err != nil {
		return result, err
	}
	return result, nil
}
