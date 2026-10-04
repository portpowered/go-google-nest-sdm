package httptransport_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func requireEventError(t *testing.T, err error, kind sdm.ErrorKind) {
	t.Helper()

	var failure *sdm.Error
	if !errors.As(err, &failure) || failure.Kind != kind {
		t.Fatalf("expected %s, got %v", kind, err)
	}
}

func TestSessionRejectsInvalidSetup(t *testing.T) {
	t.Parallel()

	client := newPushClient(t)
	zero := 0

	cases := []sdm.OpenEventSessionRequest{
		{Auth: sdm.AuthContext{AccessToken: coveragePubSubToken}, Subscription: "invalid", MaxMessages: nil},
		{Auth: sdm.AuthContext{AccessToken: ""}, Subscription: coverageSubscription, MaxMessages: nil},
		{Auth: sdm.AuthContext{AccessToken: coveragePubSubToken}, Subscription: coverageSubscription, MaxMessages: &zero},
	}

	for _, request := range cases {
		_, err := client.OpenEventSession(t.Context(), request)
		requireEventError(t, err, sdm.ErrorInvalidRequest)
	}

	request := sdm.OpenEventSessionRequest{
		Auth: sdm.AuthContext{AccessToken: coveragePubSubToken}, Subscription: coverageSubscription, MaxMessages: nil,
	}

	//nolint:staticcheck // Verify the documented invalid-context boundary under GO-08.
	_, err := client.OpenEventSession(nil, request)
	requireEventError(t, err, sdm.ErrorInvalidRequest)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = client.OpenEventSession(ctx, request)
	requireEventError(t, err, sdm.ErrorCanceled)
}

func TestPushContextValidation(t *testing.T) {
	t.Parallel()

	client := newPushClient(t)
	request := sdm.DecodePushEventRequest{Body: []byte(testEvent), Unwrapped: true}

	//nolint:staticcheck // Verify the documented invalid-context boundary under GO-08.
	_, err := client.DecodePushEvent(nil, request)
	requireEventError(t, err, sdm.ErrorInvalidRequest)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = client.DecodePushEvent(ctx, request)
	requireEventError(t, err, sdm.ErrorCanceled)
}

func TestSessionRejectsIncompleteAndOverflowDeliveries(t *testing.T) {
	t.Parallel()

	encoded := base64.StdEncoding.EncodeToString([]byte(testEvent))

	cases := []string{
		`{"receivedMessages":[{}]}`,
		`{"receivedMessages":[{"ackId":""}]}`,
		`{"receivedMessages":[{"ackId":"synthetic-ack","message":{}}]}`,
		`{"receivedMessages":[{"ackId":"a","message":{"data":"` + encoded + `"}},{"ackId":"b"}]}`,
	}

	for _, body := range cases {
		var calls atomic.Int32

		session := openTestSession(t, eventDoer(func(*http.Request) (*http.Response, error) {
			calls.Add(1)

			return eventResponse(body), nil
		}))
		_, err := session.Next(t.Context())
		requireEventError(t, err, sdm.ErrorInvalidResponse)

		if calls.Load() != 1 {
			t.Fatal("invalid deliveries triggered another exchange")
		}
	}
}

func TestDeliveryCallerCancellationAndNilContext(t *testing.T) {
	t.Parallel()

	session := openTestSession(t, eventDoer(func(*http.Request) (*http.Response, error) {
		return eventPullResponse(), nil
	}))

	delivery, err := session.Next(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	//nolint:staticcheck // Verify the documented invalid-context boundary under GO-08.
	_, err = session.Next(nil)
	requireEventError(t, err, sdm.ErrorInvalidRequest)

	//nolint:staticcheck // Verify the documented invalid-context boundary under GO-08.
	err = delivery.Acknowledge(nil)
	requireEventError(t, err, sdm.ErrorInvalidRequest)

	//nolint:staticcheck // Verify the documented invalid-context boundary under GO-08.
	err = delivery.ModifyAckDeadline(nil, sdm.AckDeadlineRequest{Seconds: 1})
	requireEventError(t, err, sdm.ErrorInvalidRequest)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = delivery.Acknowledge(ctx)
	requireEventError(t, err, sdm.ErrorCanceled)
	err = delivery.ModifyAckDeadline(ctx, sdm.AckDeadlineRequest{Seconds: 1})
	requireEventError(t, err, sdm.ErrorCanceled)

	_, err = session.Next(ctx)
	requireEventError(t, err, sdm.ErrorCanceled)

	err = session.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, err = session.Next(t.Context())
	requireEventError(t, err, sdm.ErrorCanceled)

	err = delivery.Acknowledge(t.Context())
	requireEventError(t, err, sdm.ErrorCanceled)
}

func TestEmptyPullEventuallyDelivers(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	session := openTestSession(t, eventDoer(func(*http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return eventResponse(`{"receivedMessages":[]}`), nil
		}

		return eventPullResponse(), nil
	}))

	delivery, err := session.Next(t.Context())
	if err != nil || delivery.Event().EventId != testEventID || calls.Load() != 2 {
		t.Fatalf("idle poll result %v, calls %d", err, calls.Load())
	}
}

func TestCancellationDuringPullDoesNotDeliverPendingEvent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	session := openTestSession(t, eventDoer(func(*http.Request) (*http.Response, error) {
		cancel()

		return eventPullResponse(), nil
	}))
	_, err := session.Next(ctx)
	requireEventError(t, err, sdm.ErrorCanceled)
}

func TestCloseDuringPullDoesNotDeliverPendingEvent(t *testing.T) {
	t.Parallel()

	var session sdm.EventSession

	var calls atomic.Int32

	session = openTestSession(t, eventDoer(func(*http.Request) (*http.Response, error) {
		calls.Add(1)

		err := session.Close()
		if err != nil {
			t.Error(err)
		}

		return eventPullResponse(), nil
	}))

	delivery, err := session.Next(t.Context())
	requireEventError(t, err, sdm.ErrorCanceled)

	if !errors.Is(err, context.Canceled) || delivery != nil || calls.Load() != 1 {
		t.Fatalf("closed pull delivered or retried: delivery=%v err=%v calls=%d", delivery, err, calls.Load())
	}
}
