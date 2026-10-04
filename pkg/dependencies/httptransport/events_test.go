package httptransport_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

// testEvent is a synthetic resource-change notification used by transport tests.
const testEvent = `{"eventId":"event","timestamp":"2026-01-01T00:00:00Z","userId":"user",` +
	`"resourceUpdate":{"name":"enterprises/project/devices/device",` +
	`"traits":{"sdm.devices.traits.Connectivity":{"status":"ONLINE"}}}}`

type eventDoer func(*http.Request) (*http.Response, error)

func (doer eventDoer) Do(request *http.Request) (*http.Response, error) { return doer(request) }

var errEventConnectionLost = errors.New("connection lost")

//nolint:ireturn // The public factory deliberately exposes the session interface.
func openTestSession(t *testing.T, doer httptransport.HTTPDoer) sdm.EventSession {
	t.Helper()

	client, err := httptransport.NewClient(httptransport.WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}

	events, ok := client.(sdm.EventClient)
	if !ok {
		t.Fatal("client lacks events interface")
	}

	session, err := events.OpenEventSession(context.Background(), sdm.OpenEventSessionRequest{
		Auth:         sdm.AuthContext{AccessToken: "pubsub-token"},
		Subscription: "projects/project/subscriptions/events", MaxMessages: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = session.Close() })

	return session
}

func eventPullResponse() *http.Response {
	data := base64.StdEncoding.EncodeToString([]byte(testEvent))

	return eventResponse(`{"receivedMessages":[{"ackId":"ack","message":{"data":"` + data + `"}}]}`)
}

func eventResponse(body string) *http.Response {
	response := new(http.Response)
	response.StatusCode = http.StatusOK
	response.Body = io.NopCloser(strings.NewReader(body))

	return response
}

func TestEventDeliveryExplicitAcknowledgement(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	session := openTestSession(t, eventDoer(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer pubsub-token" {
			t.Error("wrong account token")
		}

		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, fmt.Errorf("read test request: %w", err)
		}

		switch calls.Add(1) {
		case 1:
			if !strings.HasSuffix(request.URL.Path, ":pull") || string(body) != `{"maxMessages":1}` {
				t.Errorf("pull %s %s", request.URL, body)
			}

			return eventPullResponse(), nil
		case 2:
			if !strings.HasSuffix(request.URL.Path, ":modifyAckDeadline") ||
				string(body) != `{"ackDeadlineSeconds":0,"ackIds":["ack"]}` {
				t.Errorf("deadline %s %s", request.URL, body)
			}
		case 3:
			if !strings.HasSuffix(request.URL.Path, ":acknowledge") || string(body) != `{"ackIds":["ack"]}` {
				t.Errorf("ack %s %s", request.URL, body)
			}
		default:
			t.Error("unexpected exchange")
		}

		return eventResponse(`{}`), nil
	}))

	delivery, err := session.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if delivery.Event().EventId != "event" || calls.Load() != 1 {
		t.Fatal("event or implicit acknowledgement")
	}

	if delivery.Event().ResourceUpdate == nil ||
		delivery.Event().ResourceUpdate.Name != "enterprises/project/devices/device" {
		t.Fatal("resource update was lost")
	}

	err = delivery.ModifyAckDeadline(context.Background(), sdm.AckDeadlineRequest{Seconds: 0})
	if err != nil {
		t.Fatal(err)
	}

	err = delivery.Acknowledge(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if calls.Load() != 3 {
		t.Fatal("exchanges not consumed")
	}
}

func TestSessionConcurrentCloseCancelsPullAndWaiters(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})

	var calls atomic.Int32

	session := openTestSession(t, eventDoer(func(request *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(started)
		}

		<-request.Context().Done()

		return nil, request.Context().Err()
	}))

	var waiting sync.WaitGroup
	for range 4 {
		waiting.Add(1)

		go func() {
			defer waiting.Done()

			_, err := session.Next(context.Background())
			if !errors.Is(err, context.Canceled) {
				t.Errorf("Next error %v", err)
			}
		}()
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("pull did not start")
	}

	for range 4 {
		waiting.Add(1)

		go func() {
			defer waiting.Done()

			err := session.Close()
			if err != nil {
				t.Error(err)
			}
		}()
	}

	finished := make(chan struct{})

	go func() { waiting.Wait(); close(finished) }()

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("close did not release owned work")
	}

	if calls.Load() != 1 {
		t.Fatalf("pull retried %d times", calls.Load())
	}
}

func TestEmptyPullRespectsCallerDeadline(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	session := openTestSession(t, eventDoer(func(*http.Request) (*http.Response, error) {
		calls.Add(1)

		return eventResponse(`{}`), nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	_, err := session.Next(ctx)

	var failure *sdm.Error

	if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &failure) || failure.Kind != sdm.ErrorTimeout {
		t.Fatalf("deadline %v", err)
	}

	if calls.Load() != 1 {
		t.Fatal("empty pull spins")
	}
}

func TestInvalidDeliveryIsNeverAcknowledged(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	session := openTestSession(t, eventDoer(func(*http.Request) (*http.Response, error) {
		calls.Add(1)

		return eventResponse(`{"receivedMessages":[{"ackId":"ack","message":{"data":"e30="}}]}`), nil
	}))
	_, err := session.Next(context.Background())

	var failure *sdm.Error

	if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse || calls.Load() != 1 {
		t.Fatalf("invalid delivery %v", err)
	}
}

func TestCloseCancelsAcknowledgement(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})

	var calls atomic.Int32

	session := openTestSession(t, eventDoer(func(request *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return eventPullResponse(), nil
		}

		close(started)
		<-request.Context().Done()

		return nil, request.Context().Err()
	}))

	delivery, err := session.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	finished := make(chan error, 1)

	go func() { finished <- delivery.Acknowledge(context.Background()) }()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("acknowledgement did not start")
	}

	err = session.Close()
	if err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("acknowledgement %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("acknowledgement survived close")
	}

	err = delivery.ModifyAckDeadline(context.Background(), sdm.AckDeadlineRequest{Seconds: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("closed deadline %v", err)
	}

	if calls.Load() != 2 {
		t.Fatal("closed delivery performed network work")
	}
}

func TestPullFailureIsNotRetried(t *testing.T) {
	t.Parallel()

	cause := errEventConnectionLost

	var calls atomic.Int32

	session := openTestSession(t, eventDoer(func(*http.Request) (*http.Response, error) {
		calls.Add(1)

		return nil, cause
	}))

	_, err := session.Next(context.Background())
	if !errors.Is(err, cause) || calls.Load() != 1 {
		t.Fatalf("pull failure %v calls %d", err, calls.Load())
	}
}
