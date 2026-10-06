//nolint:testpackage // Private entropy and secret-erasure checks avoid exported test hooks.
package authorization

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

type stubClient struct {
	exchange func(context.Context, sdm.ExchangeTokenRequest) (sdm.ExchangeTokenResult, error)
	list     func(context.Context, sdm.ListDevicesRequest) (sdm.ListDevicesResult, error)
}

func (client stubClient) ExchangeToken(
	ctx context.Context, request sdm.ExchangeTokenRequest,
) (sdm.ExchangeTokenResult, error) {
	return client.exchange(ctx, request)
}
func (client stubClient) ListDevices(
	ctx context.Context, request sdm.ListDevicesRequest,
) (sdm.ListDevicesResult, error) {
	return client.list(ctx, request)
}

const (
	syntheticClientID = "client"
	syntheticToken    = "synthetic-token"
)

func config() sdm.OpenAuthorizationSessionRequest {
	return sdm.OpenAuthorizationSessionRequest{
		ProjectId: "device-access-project", ClientId: syntheticClientID,
		ClientSecret: "synthetic-secret", RedirectUri: "http://127.0.0.1:8765/callback",
	}
}
func entropy() io.Reader { return bytes.NewReader(bytes.Repeat([]byte{1}, 2*entropyBytes)) }
func callback(attempt *session) string {
	return attempt.request.RedirectUri + "?state=" + attempt.state + "&code=synthetic-code"
}
func assertKind(t *testing.T, err error, kind sdm.ErrorKind) {
	t.Helper()

	var failure *sdm.Error
	if !errors.As(err, &failure) || failure.Kind != kind {
		t.Fatalf("failure %v expected %s", err, kind)
	}
}

func TestConsentAndCompletion(t *testing.T) {
	t.Parallel()

	cfg := config()

	var attempt *session

	calls := 0
	client := stubClient{
		exchange: func(_ context.Context, request sdm.ExchangeTokenRequest) (sdm.ExchangeTokenResult, error) {
			calls++

			if request.ClientId != cfg.ClientId || request.ClientSecret != cfg.ClientSecret ||
				request.RedirectUri != cfg.RedirectUri || request.Code != "synthetic-code" || request.CodeVerifier == nil {
				t.Fatal("incorrect token exchange")
			}

			consent, err := url.Parse(attempt.AuthorizationURL())
			if err != nil {
				t.Fatal(err)
			}

			sum := sha256.Sum256([]byte(*request.CodeVerifier))
			if consent.Query().Get(protocol.QueryParamCodeChallenge) != base64.RawURLEncoding.EncodeToString(sum[:]) {
				t.Fatal("PKCE mismatch")
			}

			return sdm.ExchangeTokenResult{Credentials: sdm.Credentials{
				AccessToken: syntheticToken, TokenType: "Bearer", ExpiresIn: 3600,
				RefreshToken: nil, RefreshTokenExpiresIn: nil, Scope: nil,
			}}, nil
		},
		list: func(_ context.Context, request sdm.ListDevicesRequest) (sdm.ListDevicesResult, error) {
			calls++

			if request.Parent != "enterprises/device-access-project" || request.Auth.AccessToken != syntheticToken ||
				request.Filter != nil {
				t.Fatal("incorrect initial discovery")
			}

			return sdm.ListDevicesResult{
				AccountIdentity: &sdm.AccountIdentity{UserId: "authorized-account"}, Devices: []sdm.Device{{
					Name: "enterprises/device-access-project/devices/one", ParentRelations: nil,
					Traits: nil, Type: nil, AdditionalProperties: nil,
				}}}, nil
		},
	}

	var err error

	attempt, err = newSession(context.Background(), client, cfg, entropy())
	if err != nil {
		t.Fatal(err)
	}

	assertConsent(t, attempt, cfg)

	result, err := attempt.Complete(context.Background(), sdm.CompleteAuthorizationRequest{CallbackURL: callback(attempt)})
	if err != nil || calls != 2 || len(result.Devices) != 1 || result.Credentials.AccessToken != syntheticToken {
		t.Fatalf("%+v %v calls=%d", result, err, calls)
	}

	if result.AccountIdentity == nil || result.AccountIdentity.UserId != "authorized-account" {
		t.Fatal("completion lost authenticated account identity")
	}

	if attempt.verifier != "" || attempt.request.ClientSecret != "" {
		t.Fatal("secrets retained")
	}

	_, err = attempt.Complete(context.Background(), sdm.CompleteAuthorizationRequest{CallbackURL: ""})
	assertKind(t, err, sdm.ErrorCanceled)

	err = attempt.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = attempt.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenValidation(t *testing.T) {
	t.Parallel()

	client := stubClient{exchange: nil, list: nil}

	for _, redirect := range []string{
		"https://customer.example/callback", "http://localhost:123/callback", "http://[::1]:123/callback",
	} {
		cfg := config()

		cfg.RedirectUri = redirect

		if !validRequest(cfg) {
			t.Fatal(redirect)
		}
	}

	for _, redirect := range []string{
		"%", "/callback", "https://u:p@example.com/callback", "https://example.com/?a=b",
		"https://example.com/?", "https://example.com/#fragment", "http://example.com/callback", "file:///callback",
	} {
		cfg := config()
		cfg.RedirectUri = redirect
		_, err := newSession(context.Background(), client, cfg, entropy())
		assertKind(t, err, sdm.ErrorInvalidRequest)
	}

	for _, field := range []string{syntheticClientID, "secret", "project", "unsafeProject"} {
		cfg := config()

		switch field {
		case syntheticClientID:
			cfg.ClientId = ""
		case "secret":
			cfg.ClientSecret = ""
		case "project":
			cfg.ProjectId = ""
		case "unsafeProject":
			cfg.ProjectId = "../project"
		}

		_, err := newSession(context.Background(), client, cfg, entropy())
		assertKind(t, err, sdm.ErrorInvalidRequest)
	}

	_, err := newSession(context.Background(), nil, config(), entropy())
	assertKind(t, err, sdm.ErrorInvalidRequest)

	for _, size := range []int{0, entropyBytes} {
		_, err = newSession(context.Background(), client, config(), bytes.NewReader(make([]byte, size)))
		assertKind(t, err, sdm.ErrorTransport)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = newSession(ctx, client, config(), entropy())
	assertKind(t, err, sdm.ErrorCanceled)

	attempt, err := NewSession(context.Background(), client, config())
	if err != nil {
		t.Fatal(err)
	}

	err = attempt.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestInvalidCallbackCanBeCorrected(t *testing.T) {
	t.Parallel()

	attempt, err := newSession(context.Background(), stubClient{exchange: nil, list: nil}, config(), entropy())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = attempt.Close() })

	base := attempt.request.RedirectUri
	for _, value := range []string{
		"%",
		strings.Replace(base,
			"127.0.0.1",
			"example.com",
			1) + "?state=" + attempt.state + "&code=x",
		strings.Replace(base,
			"callback",
			"other",
			1) + "?state=" + attempt.state + "&code=x",
		base + "?state=" + attempt.state + "&code=x#f",
		base + "?state=%zz",
		base + "?state=wrong&code=x",
		base + "?state=" + attempt.state + "&state=" + attempt.state + "&code=x",
		base + "?state=" + attempt.state + "&code=x&code=y",
		base + "?state=" + attempt.state + "&error=x&error=y",
		base + "?state=" + attempt.state + "&error_description=x&error_description=y",
		base + "?state=" + attempt.state,
		base + "?state=" + attempt.state + "&code=%20",
		base + "?state=" + attempt.state + "&code=x&error=access_denied",
	} {
		_, err = attempt.Complete(context.Background(), sdm.CompleteAuthorizationRequest{CallbackURL: value})
		assertKind(t, err, sdm.ErrorInvalidRequest)

		if attempt.claimed {
			t.Fatal("invalid callback consumed attempt")
		}
	}

	_, err = callbackCode("http://127.0.0.1/", "%", "x")
	assertKind(t, err, sdm.ErrorInvalidRequest)
	_, err = attempt.Complete(context.Background(), sdm.CompleteAuthorizationRequest{
		CallbackURL: base + "?state=" + attempt.state + "&error=access_denied",
	})
	assertKind(t, err, sdm.ErrorUnauthorized)

	if !attempt.claimed || attempt.verifier != "" || attempt.request.ClientSecret != "" {
		t.Fatal("denied attempt retained secrets")
	}
}

func assertConsent(t *testing.T, attempt *session, cfg sdm.OpenAuthorizationSessionRequest) {
	t.Helper()

	parsed, err := url.Parse(attempt.AuthorizationURL())
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Scheme != "https" || parsed.Host != "nestservices.google.com" ||
		parsed.Path != "/partnerconnections/device-access-project/auth" {
		t.Fatal(parsed)
	}

	expected := map[string]string{
		"client_id": cfg.ClientId, "redirect_uri": cfg.RedirectUri, "response_type": "code",
		"scope": "https://www.googleapis.com/auth/sdm.service", "access_type": "offline",
		"prompt": "consent", "state": attempt.state, "code_challenge_method": "S256",
	}
	for key, value := range expected {
		if parsed.Query().Get(key) != value {
			t.Fatalf("%s mismatch", key)
		}
	}

	if len(parsed.Query()) != 9 {
		t.Fatal("unexpected consent query")
	}
}
