// Package authorization implements caller-owned Device Access consent sessions.
// It owns no HTTP listener or browser and never stores credentials implicitly.
package authorization

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sync"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const entropyBytes = 32

//nolint:containedctx // API-13: explicit session owns cancellation of account-linking completion.
type session struct {
	mu       sync.Mutex
	client   sdm.AccountAuthorizationClient
	request  sdm.OpenAuthorizationSessionRequest
	url      string
	state    string
	verifier string
	ctx      context.Context
	cancel   context.CancelFunc
	claimed  bool
}

// NewSession creates an account-linking attempt with cryptographic state and S256
// PKCE. Register the exact redirect URI with Google. The caller serves that
// callback endpoint and passes its absolute callback URI to Complete.
//
//nolint:ireturn // API-13 exposes an explicit lifecycle through the public session interface.
func NewSession(ctx context.Context, client sdm.AccountAuthorizationClient,
	request sdm.OpenAuthorizationSessionRequest,
) (sdm.AuthorizationSession, error) {
	return newSession(ctx, client, request, rand.Reader)
}

func newSession(ctx context.Context, client sdm.AccountAuthorizationClient,
	request sdm.OpenAuthorizationSessionRequest, entropy io.Reader,
) (*session, error) {
	err := ctx.Err()
	if err != nil {
		return nil, contextFailure("OpenAuthorizationSession", err)
	}

	if client == nil || !validRequest(request) {
		return nil, failure("OpenAuthorizationSession", sdm.ErrorInvalidRequest, nil)
	}

	state, err := randomValue(entropy)
	if err != nil {
		return nil, failure("OpenAuthorizationSession", sdm.ErrorTransport, err)
	}

	verifier, err := randomValue(entropy)
	if err != nil {
		return nil, failure("OpenAuthorizationSession", sdm.ErrorTransport, err)
	}

	ownedContext, cancel := context.WithCancel(ctx)

	return &session{mu: sync.Mutex{}, client: client, request: request,
		url: consentURL(request, state, verifier), state: state, verifier: verifier,
		ctx: ownedContext, cancel: cancel, claimed: false}, nil
}

func randomValue(entropy io.Reader) (string, error) {
	data := make([]byte, entropyBytes)

	_, err := io.ReadFull(entropy, data)
	if err != nil {
		return "", fmt.Errorf("read authorization entropy: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

// AuthorizationURL returns the browser destination. Its state value is public;
// the PKCE verifier and client secret remain private to this session.
func (attempt *session) AuthorizationURL() string { return attempt.url }

// Close cancels in-flight completion and forgets retained credentials. Repeated
// Close calls are safe. The caller remains responsible for its callback server.
func (attempt *session) Close() error {
	attempt.mu.Lock()
	attempt.claimed = true
	attempt.request.ClientSecret = ""
	attempt.verifier = ""
	attempt.mu.Unlock()
	attempt.cancel()

	return nil
}
