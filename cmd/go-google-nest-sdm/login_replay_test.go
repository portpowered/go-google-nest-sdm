package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
)

// Synthetic paired replay: volatile state and PKCE are validated against the
// actual browser consent before normalizing the token request's verifier.
type loginReplayTransport struct {
	paired   *pairedTransport
	consent  *url.URL
	redirect string
}

func (transport *loginReplayTransport) Do(request *http.Request) (*http.Response, error) {
	if transport.paired.consumed == 0 {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, wrapError(err)
		}

		request.Body = io.NopCloser(bytes.NewReader(body))

		form, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, wrapError(err)
		}

		verifier := form.Get("code_verifier")

		decoded, err := base64.RawURLEncoding.DecodeString(verifier)
		if err != nil || len(decoded) != 32 {
			return nil, errPairedRequestMismatch
		}

		challenge := sha256.Sum256([]byte(verifier))
		if transport.consent == nil ||
			transport.consent.Query().Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) {
			return nil, errPairedRequestMismatch
		}

		expected := url.Values{"client_id": {syntheticClientID}, "client_secret": {syntheticClientSecret},
			"code": {syntheticCode}, "code_verifier": {verifier}, "grant_type": {"authorization_code"},
			"redirect_uri": {transport.redirect}}
		transport.paired.exchanges[0].body = expected.Encode()
	}

	return transport.paired.Do(request)
}

func TestBrowserLoginPairedReplay(t *testing.T) {
	t.Parallel()

	for _, export := range []bool{false, true} {
		t.Run(map[bool]string{false: "redacted", true: "explicit-export"}[export], func(t *testing.T) {
			t.Parallel()

			listener, err := listenLoopback(context.Background(), "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}

			redirect := "http://" + listener.Addr().String() + "/oauth/callback"

			pairs := append(loadPairs(t, "oauth", 0), loadPairs(t, "rest-resources", 0)...)
			paired := &pairedTransport{t: t, exchanges: pairs, consumed: 0}
			transport := &loginReplayTransport{paired: paired, consent: nil, redirect: redirect}

			client, err := httptransport.NewClient(httptransport.WithHTTPClient(transport))
			if err != nil {
				t.Fatal(err)
			}

			output := &bytes.Buffer{}
			progress := &bytes.Buffer{}
			lookup := func(key string) string {
				return map[string]string{"SDM_CLIENT_ID": syntheticClientID, "SDM_CLIENT_SECRET": syntheticClientSecret,
					"SDM_REDIRECT_URI": redirect, "SDM_PROJECT_ID": "synthetic-enterprise"}[key]
			}
			app := application{client: client, in: strings.NewReader(""), out: output, lookup: lookup, progress: progress,
				login: loginDependencies{listen: func(context.Context, string) (net.Listener, error) { return listener, nil },
					openBrowser: transport.openConsent}}

			args := []string{"auth", "login"}
			if export {
				args = append(args, "--export")
			}

			err = app.run(context.Background(), args)
			if err != nil {
				t.Fatal(err)
			}

			if paired.consumed != len(pairs) {
				t.Fatalf("consumed %d/%d", paired.consumed, len(pairs))
			}

			if strings.Contains(output.String(), syntheticAccessToken) != export ||
				strings.Contains(output.String(), syntheticRefreshToken) != export {
				t.Fatalf("credential output: %s", output)
			}

			if !export && !strings.Contains(output.String(), `"deviceCount":1`) {
				t.Fatal(output)
			}

			_, err = listener.Accept()
			if err == nil {
				t.Fatal("callback listener still open")
			}
		})
	}
}

func (transport *loginReplayTransport) openConsent(ctx context.Context, address string) error {
	consent, err := url.Parse(address)
	if err != nil {
		return wrapError(err)
	}

	transport.consent = consent
	query := consent.Query()

	state, err := base64.RawURLEncoding.DecodeString(query.Get("state"))

	if err != nil || len(state) != 32 || query.Get("code_challenge_method") != "S256" ||
		query.Get("redirect_uri") != transport.redirect ||
		query.Get("scope") != "https://www.googleapis.com/auth/sdm.service" {
		return errPairedRequestMismatch
	}

	callback := transport.redirect + "?" + url.Values{"state": {query.Get("state")}, "code": {syntheticCode}}.Encode()

	return replayBrowserCallback(ctx, callback)
}

func replayBrowserCallback(ctx context.Context, address string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return wrapError(err)
	}

	client := new(http.Client)
	client.Timeout = time.Second

	response, err := client.Do(request)
	if err != nil {
		return wrapError(err)
	}

	_, err = io.Copy(io.Discard, response.Body)

	_ = response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errPairedRequestMismatch
	}

	return wrapError(err)
}
