package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

type syntheticAuthorizationSession struct {
	mu       sync.Mutex
	complete func(context.Context, sdm.CompleteAuthorizationRequest) (sdm.CompleteAuthorizationResult, error)
	closed   bool
}

func (*syntheticAuthorizationSession) AuthorizationURL() string {
	return "https://example.invalid/consent"
}
func (session *syntheticAuthorizationSession) Complete(
	ctx context.Context, request sdm.CompleteAuthorizationRequest,
) (sdm.CompleteAuthorizationResult, error) {
	return session.complete(ctx, request)
}
func (session *syntheticAuthorizationSession) Close() error {
	session.mu.Lock()
	defer session.mu.Unlock()

	session.closed = true

	return nil
}

func TestAuthorizationCallbackRejectsBadRequestsAndReplay(t *testing.T) {
	t.Parallel()

	redirect, err := url.Parse("http://127.0.0.1:8080/oauth/callback")
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	session := &syntheticAuthorizationSession{complete: func(
		_ context.Context, request sdm.CompleteAuthorizationRequest,
	) (sdm.CompleteAuthorizationResult, error) {
		calls++

		if strings.Contains(request.CallbackURL, "invalid") {
			return sdm.CompleteAuthorizationResult{}, &sdm.Error{Kind: sdm.ErrorInvalidRequest,
				Operation: "CompleteAuthorization", Cause: nil, StatusCode: 0}
		}

		if request.CallbackURL != redirect.String()+"?state=bound&code=synthetic-code" {
			t.Error("unbound callback")
		}

		var result sdm.CompleteAuthorizationResult

		return result, nil
	}, mu: sync.Mutex{}, closed: false}
	callback := &authorizationCallback{session: session, redirect: redirect,
		results: make(chan callbackOutcome, 1), mutex: sync.Mutex{}, completed: false}

	cases := []struct {
		method, address, host string
		status                int
	}{
		{http.MethodPost, redirect.String(), redirect.Host, http.StatusNotFound},
		{http.MethodGet, "http://127.0.0.1:8080/other", redirect.Host, http.StatusNotFound},
		{http.MethodGet, redirect.String(), "attacker.example", http.StatusNotFound},
		{http.MethodGet, redirect.String() + "?state=invalid&code=secret", redirect.Host, http.StatusBadRequest},
		{http.MethodGet, redirect.String() + "?state=bound&code=synthetic-code", redirect.Host, http.StatusOK},
		{http.MethodGet, redirect.String() + "?state=bound&code=synthetic-code", redirect.Host, http.StatusConflict},
	}
	for _, test := range cases {
		request := httptest.NewRequestWithContext(context.Background(), test.method, test.address, nil)
		request.Host = test.host
		recorder := httptest.NewRecorder()
		callback.ServeHTTP(recorder, request)

		if recorder.Code != test.status || strings.Contains(recorder.Body.String(), "secret") {
			t.Fatalf("callback status/body: %d %s", recorder.Code, recorder.Body)
		}
	}

	if calls != 2 || len(callback.results) != 1 {
		t.Fatalf("calls/outcomes: %d/%d", calls, len(callback.results))
	}
}

func TestAuthorizationCallbackDenialIsTerminal(t *testing.T) {
	t.Parallel()

	redirect, _ := url.Parse("http://127.0.0.1:8080/oauth/callback")
	session := &syntheticAuthorizationSession{complete: func(
		context.Context, sdm.CompleteAuthorizationRequest,
	) (sdm.CompleteAuthorizationResult, error) {
		return sdm.CompleteAuthorizationResult{}, &sdm.Error{Kind: sdm.ErrorUnauthorized,
			Operation: "CompleteAuthorization", Cause: nil, StatusCode: 0}
	}, mu: sync.Mutex{}, closed: false}
	callback := &authorizationCallback{session: session, redirect: redirect,
		results: make(chan callbackOutcome, 1), mutex: sync.Mutex{}, completed: false}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		redirect.String()+"?state=bound&error=access_denied", nil)
	callback.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest || !callback.completed || len(callback.results) != 1 {
		t.Fatal("denial did not terminate")
	}
}

func TestReceiveAuthorizationClosesOwnedListener(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"success", "cancel", "timeout", "browser-error"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			listener, err := (new(net.ListenConfig)).Listen(context.Background(), "tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}

			redirect, err := url.Parse("http://" + listener.Addr().String() + "/oauth/callback")
			if err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			session := &syntheticAuthorizationSession{complete: func(
				context.Context, sdm.CompleteAuthorizationRequest,
			) (sdm.CompleteAuthorizationResult, error) {
				var result sdm.CompleteAuthorizationResult

				return result, nil
			}, mu: sync.Mutex{}, closed: false}
			progress := &bytes.Buffer{}

			var app application

			app.progress = progress
			launch := syntheticBrowser(ctx, cancel, mode, redirect.String())

			_, err = app.receiveAuthorization(ctx, session, redirect, listener, launch)
			if (err != nil) != (mode != "success") {
				t.Fatalf("mode %s: %v", mode, err)
			}

			_, acceptErr := listener.Accept()
			if acceptErr == nil {
				t.Fatal("listener remains open")
			}

			if !strings.Contains(progress.String(), "Google consent") {
				t.Fatal("missing progress")
			}
		})
	}
}

func TestLoginRedirectRequiresRegisteredLoopback(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "https://127.0.0.1:8080/callback", "http://example.com:8080/callback",
		"http://127.0.0.1/callback", "http://127.0.0.1:0/callback", "http://user@127.0.0.1:8080/callback",
		"http://127.0.0.1:8080/callback?code=secret", "http://127.0.0.1:8080/callback#fragment"} {
		_, err := loginRedirect(value)
		if err == nil {
			t.Fatalf("accepted %s", value)
		}
	}

	for _, value := range []string{"http://localhost:8080/callback", "http://127.0.0.1:8080/callback",
		"http://[::1]:8080/callback"} {
		_, err := loginRedirect(value)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func syntheticBrowser(
	ctx context.Context, cancel context.CancelFunc, mode, redirect string,
) func(context.Context, string) error {
	return func(context.Context, string) error {
		switch mode {
		case "cancel":
			cancel()

			return nil
		case "timeout":
			return nil
		case "browser-error":
			return errSyntheticOutputFailure
		default:
			return replayBrowserCallback(ctx, redirect+"?state=bound&code=synthetic-code")
		}
	}
}

func TestReceiveAuthorizationCancellationEndsActiveCallback(t *testing.T) {
	t.Parallel()

	listener, err := listenLoopback(context.Background(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	redirect, err := url.Parse("http://" + listener.Addr().String() + "/oauth/callback")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	finished := make(chan struct{})
	session := &syntheticAuthorizationSession{mu: sync.Mutex{}, closed: false,
		complete: func(ctx context.Context, _ sdm.CompleteAuthorizationRequest) (sdm.CompleteAuthorizationResult, error) {
			close(started)
			<-ctx.Done()
			close(finished)

			return sdm.CompleteAuthorizationResult{}, wrapError(ctx.Err())
		}}
	requestDone := make(chan struct{})
	launch := func(context.Context, string) error {
		go func() {
			_ = replayBrowserCallback(context.WithoutCancel(ctx), redirect.String()+"?state=bound&code=synthetic-code")

			close(requestDone)
		}()

		<-started
		cancel()

		return nil
	}

	var app application

	_, err = app.receiveAuthorization(ctx, session, redirect, listener, launch)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	select {
	case <-finished:
	default:
		t.Fatal("active callback outlived the command")
	}

	<-requestDone

	if !session.closed {
		t.Fatal("session not closed")
	}
}
