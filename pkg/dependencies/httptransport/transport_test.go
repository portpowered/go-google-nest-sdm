package httptransport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
)

type testDoer func(*http.Request) (*http.Response, error)

func (doer testDoer) Do(request *http.Request) (*http.Response, error) { return doer(request) }

type trackedBody struct {
	io.Reader

	closed bool
}

func (body *trackedBody) Close() error {
	body.closed = true

	return nil
}

func testResponse(status int, body io.ReadCloser) *http.Response {
	return &http.Response{
		Status: "", StatusCode: status, Proto: "", ProtoMajor: 0, ProtoMinor: 0,
		Header: nil, Body: body, ContentLength: 0, TransferEncoding: nil,
		Close: false, Uncompressed: false, Trailer: nil, Request: nil, TLS: nil,
	}
}

func TestExchangeClosesBodiesAndPreservesCauses(t *testing.T) {
	t.Parallel()

	body := &trackedBody{Reader: strings.NewReader("{broken"), closed: false}

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
		return testResponse(http.StatusOK, body), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetDevice(context.Background(), "secret", "enterprises/project/devices/device")

	var failure *transport.Error

	if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidResponse || !body.closed {
		t.Fatalf("failure=%v closed=%v", err, body.closed)
	}

	if failure.Cause == nil {
		t.Fatal("missing JSON cause")
	}
}

func TestCancellationAndNoRetry(t *testing.T) {
	t.Parallel()

	calls := 0

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(request *http.Request) (*http.Response, error) {
		calls++

		<-request.Context().Done()

		return nil, request.Context().Err()
	})))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.GetDevice(ctx, "token", "enterprises/project/devices/device")

	var failure *transport.Error

	if !errors.As(err, &failure) || failure.Kind != transport.ErrorCanceled ||
		!errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestInvalidInputMakesNoNetworkCall(t *testing.T) {
	t.Parallel()

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected HTTP request")

		return nil, context.Canceled
	})))
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{
		"", "enterprises/p/devices/..", "enterprises/p/devices/d?query",
		"enterprises/p/devices/d%2Fextra", "enterprises/p/structures/d",
	} {
		_, err = client.GetDevice(context.Background(), "token", name)
		if err == nil {
			t.Fatalf("accepted %q", name)
		}
	}

	_, err = client.Pull(context.Background(), "cloud", "projects/p/subscriptions/s",
		wire.PullRequest{MaxMessages: 0, ReturnImmediately: nil})
	if err == nil {
		t.Fatal("accepted zero batch")
	}

	err = client.ModifyAckDeadline(context.Background(), "cloud", "projects/p/subscriptions/s",
		wire.ModifyAckDeadlineRequest{AckIds: []string{"id"}, AckDeadlineSeconds: 601})
	if err == nil {
		t.Fatal("accepted excessive deadline")
	}
}

func TestOAuthUsesFormAndCallerCredentials(t *testing.T) {
	t.Parallel()

	calls := 0

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(request *http.Request) (*http.Response, error) {
		calls++

		if request.Header.Get("Authorization") != "" {
			t.Fatal("unexpected OAuth bearer")
		}

		err := request.ParseForm()
		if err != nil {
			t.Fatal(err)
		}

		if request.Form.Get("client_secret") != "private & secret" ||
			request.Form.Get("refresh_token") != "refresh+token" || request.Form.Get("grant_type") != "refresh_token" {
			t.Fatalf("invalid form")
		}

		const response = `{"access_token":"new","expires_in":3600,"token_type":"Bearer"}`

		return testResponse(http.StatusOK, io.NopCloser(strings.NewReader(response))), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	token := "refresh+token"

	result, err := client.OAuthRefresh(context.Background(), wire.OAuthTokenRequest{
		ClientId: "id", ClientSecret: "private & secret", RefreshToken: &token,
		Code: nil, CodeVerifier: nil, GrantType: wire.RefreshToken, RedirectUri: nil,
	})
	if err != nil || result.AccessToken != "new" || calls != 1 {
		t.Fatalf("result=%v err=%v calls=%d", result, err, calls)
	}
}

func TestBaseURLValidation(t *testing.T) {
	t.Parallel()

	for _, base := range []string{
		"", "file:///tmp/api", "https://user:password@host", "https://host?secret=x", "https://host#fragment",
	} {
		_, err := transport.New(transport.WithSDMBaseURL(base))
		if err == nil {
			t.Fatalf("accepted %q", base)
		}
	}

	_, err := transport.New(nil)
	if err == nil {
		t.Fatal("accepted nil option")
	}

	_, err = transport.New(transport.WithHTTPClient(nil))
	if err == nil {
		t.Fatal("accepted nil HTTP client")
	}
}

func TestStatusClassificationAndCredentialSafety(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status int
		kind   transport.ErrorKind
	}{
		{http.StatusUnauthorized, transport.ErrorUnauthorized}, {http.StatusForbidden, transport.ErrorUnauthorized},
		{http.StatusNotFound, transport.ErrorNotFound}, {http.StatusTooManyRequests, transport.ErrorRateLimited},
		{http.StatusInternalServerError, transport.ErrorServer}, {http.StatusBadRequest, transport.ErrorInvalidResponse},
	}
	for _, test := range cases {
		client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
			return testResponse(test.status, io.NopCloser(strings.NewReader("private token details"))), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.GetDevice(context.Background(), "private-token", "enterprises/project/devices/device")

		var failure *transport.Error

		if !errors.As(err, &failure) || failure.Kind != test.kind || failure.StatusCode != test.status {
			t.Fatalf("status=%d err=%v", test.status, err)
		}

		if strings.Contains(err.Error(), "private") {
			t.Fatal("credential leaked through error")
		}
	}
}

func TestResponseLimitAndMissingBody(t *testing.T) {
	t.Parallel()

	const responseLimit = 8 << 20

	for _, body := range []io.ReadCloser{nil, io.NopCloser(strings.NewReader(strings.Repeat(" ", responseLimit+1)))} {
		client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusOK, body), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.ListDevices(context.Background(), "token", "enterprises/project")

		var failure *transport.Error

		if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidResponse {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestTimeoutCauseAndResponseCleanup(t *testing.T) {
	t.Parallel()

	body := &trackedBody{Reader: strings.NewReader("unused"), closed: false}

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
		return testResponse(0, body), context.DeadlineExceeded
	})))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.ListDevices(context.Background(), "token", "enterprises/project")

	var failure *transport.Error

	if !errors.As(err, &failure) || failure.Kind != transport.ErrorTimeout ||
		!errors.Is(err, context.DeadlineExceeded) || !body.closed {
		t.Fatalf("err=%v closed=%v", err, body.closed)
	}
}

func TestDefaultClientRejectsRedirects(t *testing.T) {
	t.Parallel()

	var targetCalls atomic.Int32

	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		targetCalls.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(target.Close)

	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(source.Close)

	client, err := transport.New(transport.WithSDMBaseURL(source.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetDevice(context.Background(), "private-bearer", "enterprises/project/devices/device")

	var failure *transport.Error

	if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidResponse ||
		failure.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("redirect classification: %v", err)
	}

	calls := targetCalls.Load()
	if calls != 0 {
		t.Fatalf("redirect target received %d requests", calls)
	}
}
