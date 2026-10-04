package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const callbackHeaderTimeout = 5 * time.Second
const callbackShutdownTimeout = 3 * time.Second

type loginDependencies struct {
	listen      func(context.Context, string) (net.Listener, error)
	openBrowser func(context.Context, string) error
}

func defaultLoginDependencies() loginDependencies {
	return loginDependencies{listen: listenLoopback, openBrowser: openBrowser}
}

func listenLoopback(ctx context.Context, address string) (net.Listener, error) {
	config := new(net.ListenConfig)
	listener, err := config.Listen(ctx, "tcp", address)

	return listener, wrapError(err)
}

func openBrowser(ctx context.Context, address string) error {
	var command *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// #nosec G204 -- Fixed executable and argument list; no shell.
		command = exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", address)
	case "darwin":
		command = exec.CommandContext(ctx, "open", address) // #nosec G204 -- Fixed executable; no shell.
	default:
		command = exec.CommandContext(ctx, "xdg-open", address) // #nosec G204 -- Fixed executable; no shell.
	}

	// The executable and option arguments are fixed; the SDK supplies the consent URL.
	// No command shell interprets the URL, and launcher output cannot leak into JSON.
	return wrapError(command.Run())
}

func hasTimeoutFlag(args []string) bool {
	for _, argument := range args {
		if argument == "--timeout" || argument == "-timeout" ||
			strings.HasPrefix(argument, "--timeout=") || strings.HasPrefix(argument, "-timeout=") {
			return true
		}
	}

	return false
}

type loginOutput struct {
	ExpiresIn       int    `json:"expiresIn"`
	TokenType       string `json:"tokenType"`
	HasRefreshToken bool   `json:"hasRefreshToken"`
	DeviceCount     int    `json:"deviceCount"`
}

func (app application) loginAccount(ctx context.Context, options invocation, account credentials) (any, error) {
	project := options.project
	if project == "" {
		project = app.lookup("SDM_PROJECT_ID")
	}

	redirectURI := options.redirect
	if redirectURI == "" {
		redirectURI = account.RedirectURI
	}

	redirect, err := loginRedirect(redirectURI)
	if err != nil {
		return nil, err
	}

	if project == "" {
		return nil, errLoginProjectRequired
	}

	client, ok := app.client.(sdm.AuthorizationClient)
	if !ok {
		return nil, errAuthenticationUnsupported
	}

	session, err := client.OpenAuthorizationSession(ctx, sdm.OpenAuthorizationSessionRequest{
		ProjectId: project, ClientId: account.ClientID, ClientSecret: account.ClientSecret, RedirectUri: redirectURI,
	})
	if err != nil {
		return nil, wrapError(err)
	}

	defer func() { _ = session.Close() }()

	dependencies := app.login
	if dependencies.listen == nil || dependencies.openBrowser == nil {
		dependencies = defaultLoginDependencies()
	}

	address := redirect.Host
	if redirect.Hostname() == "localhost" {
		address = net.JoinHostPort("127.0.0.1", redirect.Port())
	}

	listener, err := dependencies.listen(ctx, address)
	if err != nil {
		return nil, wrapError(err)
	}

	result, err := app.receiveAuthorization(ctx, session, redirect, listener, dependencies.openBrowser)
	if err != nil {
		return nil, wrapError(err)
	}

	if options.export {
		account.AccessToken = result.Credentials.AccessToken
		if result.Credentials.RefreshToken != nil {
			account.RefreshToken = *result.Credentials.RefreshToken
		}

		account.RedirectURI = redirectURI
		account.AuthorizationCode = ""
		account.CodeVerifier = ""

		return account, nil
	}

	return loginOutput{ExpiresIn: result.Credentials.ExpiresIn, TokenType: result.Credentials.TokenType,
		HasRefreshToken: result.Credentials.RefreshToken != nil, DeviceCount: len(result.Devices)}, nil
}

func loginRedirect(value string) (*url.URL, error) {
	redirect, err := url.Parse(value)
	if err != nil || redirect.Scheme != "http" || redirect.User != nil || redirect.RawQuery != "" ||
		redirect.Fragment != "" || redirect.Port() == "" || redirect.Port() == "0" || redirect.Path == "" {
		return nil, errLoginRedirect
	}

	host := redirect.Hostname()
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil, errLoginRedirect
	}

	return redirect, nil
}

type callbackOutcome struct {
	result sdm.CompleteAuthorizationResult
	err    error
}

type authorizationCallback struct {
	session   sdm.AuthorizationSession
	redirect  *url.URL
	results   chan callbackOutcome
	mutex     sync.Mutex
	completed bool
}

func (callback *authorizationCallback) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet || request.URL.EscapedPath() != callback.redirect.EscapedPath() ||
		request.Host != callback.redirect.Host {
		http.Error(writer, "Unknown callback", http.StatusNotFound)

		return
	}

	callback.mutex.Lock()
	if callback.completed {
		callback.mutex.Unlock()
		http.Error(writer, "Authorization already completed", http.StatusConflict)

		return
	}

	callback.completed = true
	callback.mutex.Unlock()
	// Bind to the configured origin. Host and proxy headers never construct the callback URL.
	address := *callback.redirect
	address.RawQuery = request.URL.RawQuery
	result, err := callback.session.Complete(request.Context(),
		sdm.CompleteAuthorizationRequest{CallbackURL: address.String()})

	var failure *sdm.Error

	if errors.As(err, &failure) && failure.Kind == sdm.ErrorInvalidRequest &&
		failure.Operation == "CompleteAuthorization" {
		callback.mutex.Lock()
		callback.completed = false
		callback.mutex.Unlock()
		http.Error(writer, "Invalid authorization callback", http.StatusBadRequest)

		return
	}

	if err != nil {
		http.Error(writer, "Authorization failed. Return to the terminal.", http.StatusBadRequest)
	} else {
		_, _ = io.WriteString(writer, "Authorization completed. You can close this window.")
	}

	callback.results <- callbackOutcome{result: result, err: err}
}

func (app application) receiveAuthorization(ctx context.Context, session sdm.AuthorizationSession,
	redirect *url.URL, listener net.Listener, launch func(context.Context, string) error,
) (sdm.CompleteAuthorizationResult, error) {
	callbackContext, stopCallback := context.WithCancel(ctx)
	results := make(chan callbackOutcome, 1)
	callback := &authorizationCallback{session: session, redirect: redirect, results: results,
		mutex: sync.Mutex{}, completed: false}
	server := new(http.Server)
	server.Handler = callback
	server.ReadHeaderTimeout = callbackHeaderTimeout
	server.BaseContext = func(net.Listener) context.Context { return callbackContext }
	served := make(chan struct{})

	serveErrors := make(chan error, 1)

	go func() {
		serveErrors <- server.Serve(listener)

		close(served)
	}()

	defer func() {
		stopCallback()

		_ = session.Close()

		shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), callbackShutdownTimeout)
		defer cancel()

		_ = server.Shutdown(shutdownContext)
		_ = server.Close()
		// Consume Serve completion so the owned serving goroutine cannot outlive the command.
		<-served
	}()

	if app.progress != nil {
		_, err := fmt.Fprintln(app.progress, "Opening Google consent; waiting for the registered localhost callback.")
		if err != nil {
			return sdm.CompleteAuthorizationResult{}, wrapError(err)
		}
	}

	err := launch(ctx, session.AuthorizationURL())
	if err != nil {
		if ctx.Err() != nil {
			return sdm.CompleteAuthorizationResult{}, wrapError(ctx.Err())
		}

		return sdm.CompleteAuthorizationResult{}, errLoginBrowser
	}

	select {
	case outcome := <-results:
		return outcome.result, wrapError(outcome.err)
	case <-ctx.Done():
		return sdm.CompleteAuthorizationResult{}, wrapError(ctx.Err())
	case serveErr := <-serveErrors:
		return sdm.CompleteAuthorizationResult{}, wrapError(serveErr)
	}
}
