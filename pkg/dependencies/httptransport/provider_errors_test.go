package httptransport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestOAuthProviderFailureClassification(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code string
		kind transport.ErrorKind
	}{
		{"invalid_grant", transport.ErrorUnauthorized}, {"invalid_client", transport.ErrorUnauthorized},
		{"unauthorized_client", transport.ErrorUnauthorized}, {"access_denied", transport.ErrorUnauthorized},
		{"invalid_request", transport.ErrorRejected}, {"future_rejection", transport.ErrorRejected},
	}
	for _, test := range cases {
		body := &trackedBody{
			Reader: strings.NewReader(`{"error":"` + test.code + `","error_description":"private-token"}`),
			closed: false,
		}

		client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusBadRequest, body), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		refresh := "private-token"
		_, err = client.OAuthRefresh(context.Background(), wire.OAuthTokenRequest{
			ClientId: "id", ClientSecret: "private-secret", Code: nil, CodeVerifier: nil,
			GrantType: "", RedirectUri: nil, RefreshToken: &refresh,
		})

		var (
			failure  *transport.Error
			provider *sdm.ProviderError
		)

		if !errors.As(err, &failure) || failure.Kind != test.kind ||
			!errors.As(err, &provider) || string(provider.Code) != test.code || provider.HTTPStatus != http.StatusBadRequest {
			t.Fatalf("code=%s err=%v provider=%v", test.code, err, provider)
		}

		if !body.closed || strings.Contains(err.Error(), "private") || strings.Contains(provider.Error(), "private") {
			t.Fatal("body not closed or sensitive provider text escaped")
		}
	}
}

func TestGoogleProviderFailureSurvivesPublicAdapter(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
		return testResponse(http.StatusForbidden, io.NopCloser(strings.NewReader(
			`{"error":{"code":403,"status":"PERMISSION_DENIED","message":"private-token","details":[{"secret":"private"}]}}`,
		))), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetDevice(context.Background(), sdm.GetDeviceRequest{
		Auth: sdm.AuthContext{AccessToken: "private-token"}, Name: testDeviceName,
	})

	var (
		failure  *sdm.Error
		provider *sdm.ProviderError
	)

	if !errors.As(err, &failure) || failure.Kind != sdm.ErrorUnauthorized ||
		!errors.As(err, &provider) || provider.Code != "PERMISSION_DENIED" || provider.HTTPStatus != http.StatusForbidden {
		t.Fatalf("err=%v provider=%v", err, provider)
	}

	if strings.Contains(err.Error(), "private") || strings.Contains(provider.Error(), "private") {
		t.Fatal("sensitive provider text escaped")
	}
}

func TestMalformedProviderFailureKeepsStatusWithoutBody(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{testNullJSON, `{}`, `{"error":"private-token"}`, `{"error":{"status":123}}`} {
		client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusForbidden, io.NopCloser(strings.NewReader(payload))), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.GetDevice(context.Background(), "private-token", testDeviceName)

		var (
			failure  *transport.Error
			provider *sdm.ProviderError
		)

		if !errors.As(err, &failure) || failure.StatusCode != http.StatusForbidden ||
			failure.Cause == nil || errors.As(err, &provider) {
			t.Fatalf("payload=%s err=%v", payload, err)
		}

		if strings.Contains(failure.Cause.Error(), "private") {
			t.Fatal("malformed response leaked body")
		}
	}
}

func TestProviderFailureResponseReadErrorPreservesCause(t *testing.T) {
	t.Parallel()

	body := &trackedBody{Reader: providerErrorReader{}, closed: false}

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
		return testResponse(http.StatusServiceUnavailable, body), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetDevice(context.Background(), "token", testDeviceName)

	var failure *transport.Error
	if !errors.As(err, &failure) || failure.Kind != transport.ErrorServer ||
		failure.StatusCode != http.StatusServiceUnavailable || !errors.Is(err, io.ErrUnexpectedEOF) || !body.closed {
		t.Fatalf("err=%v closed=%t", err, body.closed)
	}
}

type providerErrorReader struct{}

func (providerErrorReader) Read(_ []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestGoogleNumericProviderFailure(t *testing.T) {
	t.Parallel()

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
		return testResponse(http.StatusBadRequest, io.NopCloser(strings.NewReader(`{"error":{"code":400}}`))), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetDevice(context.Background(), "token", testDeviceName)

	var (
		failure  *transport.Error
		provider *sdm.ProviderError
	)

	if !errors.As(err, &failure) || failure.Kind != transport.ErrorRejected ||
		!errors.As(err, &provider) || provider.Code != "400" || provider.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("err=%v provider=%v", err, provider)
	}
}
