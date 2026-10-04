package main

import (
	"context"
	"errors"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

type observedClient struct {
	sdm.Client

	events sdm.EventClient
	closed *bool
}

type observedSession struct {
	sdm.EventSession

	closed *bool
}

//nolint:ireturn // Implements the public SDK lifecycle interface while observing actual transport cleanup.
func (client observedClient) OpenEventSession(
	ctx context.Context, request sdm.OpenEventSessionRequest,
) (sdm.EventSession, error) {
	session, err := client.events.OpenEventSession(ctx, request)
	if err != nil {
		return nil, wrapError(err)
	}

	return observedSession{EventSession: session, closed: client.closed}, nil
}

func (session observedSession) Close() error {
	*session.closed = true

	return wrapError(session.EventSession.Close())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errSyntheticOutputFailure }

func TestEventFailureClosesSessionWithoutAcknowledgement(t *testing.T) {
	t.Parallel()
	app, _ := newTestApplication(t, loadPairs(t, pubSubFixture, 0), "")
	closed := false

	events, ok := app.client.(sdm.EventClient)
	if !ok {
		t.Fatal("client does not implement EventClient")
	}

	app.client = observedClient{Client: app.client, events: events, closed: &closed}
	app.out = failingWriter{}

	err := app.run(context.Background(), []string{
		eventsGroup, pullOperation, resourceFlag, syntheticSubscription,
	})
	if err == nil || !closed {
		t.Fatalf("error %v, closed %v", err, closed)
	}
}

func TestEventSuccessClosesSession(t *testing.T) {
	t.Parallel()
	app, _ := newTestApplication(t, loadPairs(t, pubSubFixture, 0, 2), "")
	closed := false

	events, ok := app.client.(sdm.EventClient)
	if !ok {
		t.Fatal("client does not implement EventClient")
	}

	app.client = observedClient{Client: app.client, events: events, closed: &closed}

	err := app.run(context.Background(), []string{
		eventsGroup, pullOperation, resourceFlag, syntheticSubscription,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !closed {
		t.Fatal("session was not closed")
	}
}

func TestCanceledEventDoesNotUseTransport(t *testing.T) {
	t.Parallel()
	app, _ := newTestApplication(t, nil, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.run(ctx, []string{
		eventsGroup, pullOperation, resourceFlag, syntheticSubscription,
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
