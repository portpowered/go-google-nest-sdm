//nolint:testpackage // Private entropy and secret-erasure checks avoid exported test hooks.
package authorization

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

var errSyntheticNetwork = errors.New("synthetic network failure")

const (
	failureCanceled   = "canceled"
	failureDeadline   = "deadline"
	failureExchange   = "exchange"
	failureDiscovery  = "discovery"
	failurePublic     = "public"
	failureLateCancel = "lateCancel"
)

func TestCompletionFailures(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{
		failureExchange, failureDiscovery, failurePublic, failureCanceled, failureDeadline, failureLateCancel,
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			calls := 0
			client := failureClient(kind, cancel, &calls)

			attempt, err := newSession(context.Background(), client, config(), entropy())
			if err != nil {
				t.Fatal(err)
			}

			t.Cleanup(func() { _ = attempt.Close() })

			_, err = attempt.Complete(ctx, sdm.CompleteAuthorizationRequest{CallbackURL: callback(attempt)})

			expected := sdm.ErrorTransport

			switch kind {
			case failurePublic:
				expected = sdm.ErrorRejected
			case failureCanceled, failureLateCancel:
				expected = sdm.ErrorCanceled
			case failureDeadline:
				expected = sdm.ErrorTimeout
			}

			assertKind(t, err, expected)

			var failure *sdm.Error
			if !errors.As(err, &failure) || failure.Operation != "AuthorizeAccount" {
				t.Fatal(err)
			}

			if kind == failurePublic && failure.StatusCode != http.StatusBadRequest {
				t.Fatal("lost provider status")
			}

			if kind == failureDiscovery || kind == failureLateCancel {
				if calls != 2 {
					t.Fatal(calls)
				}
			} else if calls != 1 {
				t.Fatal(calls)
			}

			if attempt.verifier != "" || attempt.request.ClientSecret != "" {
				t.Fatal("failure retained secrets")
			}
		})
	}
}

func TestCompletionContextAndClose(t *testing.T) {
	t.Parallel()

	attempt, err := newSession(context.Background(), stubClient{exchange: nil, list: nil}, config(), entropy())
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = attempt.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = attempt.Complete(ctx, sdm.CompleteAuthorizationRequest{CallbackURL: ""})
	assertKind(t, err, sdm.ErrorCanceled)

	deadline, deadlineCancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer deadlineCancel()

	_, err = attempt.Complete(deadline, sdm.CompleteAuthorizationRequest{CallbackURL: ""})
	assertKind(t, err, sdm.ErrorTimeout)

	parent, parentCancel := context.WithCancel(context.Background())

	other, err := newSession(parent, stubClient{exchange: nil, list: nil}, config(), entropy())
	if err != nil {
		t.Fatal(err)
	}

	parentCancel()

	_, err = other.Complete(context.Background(), sdm.CompleteAuthorizationRequest{CallbackURL: ""})
	assertKind(t, err, sdm.ErrorCanceled)

	err = other.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentCloseAndDuplicate(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	ended := make(chan error, 1)
	client := stubClient{exchange: func(ctx context.Context, _ sdm.ExchangeTokenRequest) (sdm.ExchangeTokenResult, error) {
		close(started)
		<-ctx.Done()

		return sdm.ExchangeTokenResult{}, ctx.Err()
	}, list: nil}

	attempt, err := newSession(context.Background(), client, config(), entropy())
	if err != nil {
		t.Fatal(err)
	}

	request := sdm.CompleteAuthorizationRequest{CallbackURL: callback(attempt)}

	go func() { _, completeErr := attempt.Complete(context.Background(), request); ended <- completeErr }()

	<-started

	_, err = attempt.Complete(context.Background(), request)
	assertKind(t, err, sdm.ErrorCanceled)

	err = attempt.Close()
	if err != nil {
		t.Fatal(err)
	}

	assertKind(t, <-ended, sdm.ErrorCanceled)

	err = attempt.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func failureClient(kind string, cancel context.CancelFunc, calls *int) stubClient {
	sentinel := errSyntheticNetwork

	return stubClient{
		exchange: func(_ context.Context, _ sdm.ExchangeTokenRequest) (sdm.ExchangeTokenResult, error) {
			*calls++

			switch kind {
			case failureExchange:
				return sdm.ExchangeTokenResult{}, sentinel
			case failurePublic:
				return sdm.ExchangeTokenResult{}, &sdm.Error{
					Kind: sdm.ErrorRejected, Operation: failureExchange, Cause: sentinel, StatusCode: http.StatusBadRequest,
				}
			case failureCanceled:
				return sdm.ExchangeTokenResult{}, context.Canceled
			case failureDeadline:
				return sdm.ExchangeTokenResult{}, context.DeadlineExceeded
			}

			return sdm.ExchangeTokenResult{Credentials: sdm.Credentials{
				AccessToken: "token", ExpiresIn: 0, RefreshToken: nil,
				RefreshTokenExpiresIn: nil, Scope: nil, TokenType: "",
			}}, nil
		},
		list: func(_ context.Context, _ sdm.ListDevicesRequest) (sdm.ListDevicesResult, error) {
			*calls++

			if kind == failureDiscovery {
				return sdm.ListDevicesResult{}, sentinel
			}

			cancel()

			return sdm.ListDevicesResult{Devices: nil}, nil
		}}
}
