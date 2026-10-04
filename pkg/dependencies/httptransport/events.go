package httptransport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

// Empty successful pulls are polled with a delay to avoid a busy loop.
const idlePullDelay = 100 * time.Millisecond

var (
	errPullOverflow = errors.New("pull exceeded requested message count")
	errDeliveryData = errors.New("missing acknowledgement or event data")
)

//nolint:containedctx // An explicit session owns cancellation under GO-08 and GO-09.
type eventSession struct {
	transport    *Client
	token        string
	subscription string
	maxMessages  int
	ctx          context.Context
	cancel       context.CancelFunc
	next         chan struct{}
	pending      []wire.ReceivedMessage // Protected by the next semaphore.
}

//nolint:ireturn // Implements the transport-independent public session factory.
func (client *sdkClient) OpenEventSession(
	ctx context.Context, request sdm.OpenEventSessionRequest,
) (sdm.EventSession, error) {
	if ctx == nil {
		return nil, publicError(fail("OpenEventSession", ErrorInvalidRequest, nil))
	}

	_, err := resourcePath(request.Subscription, protocol.PathPull,
		protocol.CollectionProjects, protocol.CollectionSubscriptions)
	if err != nil {
		return nil, publicError(err)
	}

	maxMessages := 1
	if request.MaxMessages != nil {
		maxMessages = *request.MaxMessages
	}

	if request.Auth.AccessToken == "" || maxMessages < 1 {
		return nil, publicError(fail("OpenEventSession", ErrorInvalidRequest, nil))
	}

	err = ctx.Err()
	if err != nil {
		return nil, sessionContextError("OpenEventSession", err)
	}

	owned, cancel := context.WithCancel(ctx)

	return &eventSession{transport: client.transport, token: request.Auth.AccessToken,
		subscription: request.Subscription, maxMessages: maxMessages, ctx: owned,
		cancel: cancel, next: make(chan struct{}, 1), pending: nil}, nil
}

func (session *eventSession) Close() error {
	session.cancel()

	return nil
}

//nolint:ireturn // Implements the public session's transport-independent delivery method.
func (session *eventSession) Next(ctx context.Context) (sdm.EventDelivery, error) {
	if ctx == nil {
		return nil, publicError(fail("Next", ErrorInvalidRequest, nil))
	}

	merged, cancel := session.operationContext(ctx)
	defer cancel()

	select {
	case session.next <- struct{}{}:
		defer func() { <-session.next }()
	case <-merged.Done():
		return nil, sessionContextError("Next", merged.Err())
	}

	for {
		err := session.ctx.Err()
		if err != nil {
			return nil, sessionContextError("Next", err)
		}

		err = merged.Err()
		if err != nil {
			return nil, sessionContextError("Next", err)
		}

		if len(session.pending) > 0 {
			return session.takeDelivery()
		}

		response, err := session.transport.Pull(merged, session.token, session.subscription,
			wire.PullRequest{MaxMessages: session.maxMessages, ReturnImmediately: nil})
		if err != nil {
			return nil, publicError(err)
		}

		if response.ReceivedMessages != nil {
			if len(*response.ReceivedMessages) > session.maxMessages {
				return nil, publicError(fail("Next", ErrorInvalidResponse, errPullOverflow))
			}

			session.pending = *response.ReceivedMessages
		}

		if len(session.pending) > 0 {
			continue
		}

		err = waitForPull(merged)
		if err != nil {
			return nil, sessionContextError("Next", err)
		}
	}
}

// operationContext stops network work when either its caller or session closes.
func (session *eventSession) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	merged, cancel := context.WithCancel(ctx)

	stop := context.AfterFunc(session.ctx, cancel)

	if session.ctx.Err() != nil {
		cancel()
	}

	return merged, func() { stop(); cancel() }
}

func waitForPull(ctx context.Context) error {
	timer := time.NewTimer(idlePullDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for pull: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func (session *eventSession) takeDelivery() (*eventDelivery, error) {
	message := session.pending[0]

	var empty wire.ReceivedMessage

	session.pending[0] = empty
	session.pending = session.pending[1:]

	if message.AckId == nil || *message.AckId == "" || message.Message == nil || message.Message.Data == nil {
		return nil, publicError(fail("Next", ErrorInvalidResponse, errDeliveryData))
	}

	event, err := sdm.DecodeEvent(*message.Message.Data)
	if err != nil {
		return nil, publicError(fail("Next", ErrorInvalidResponse, err))
	}

	return &eventDelivery{session: session, ackID: *message.AckId, event: event}, nil
}

func sessionContextError(operation string, cause error) error {
	kind := sdm.ErrorCanceled
	if errors.Is(cause, context.DeadlineExceeded) {
		kind = sdm.ErrorTimeout
	}

	return &sdm.Error{Operation: operation, Kind: kind, Cause: cause, StatusCode: 0}
}
