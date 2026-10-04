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

func TestOAuthSuccessValidatesOriginalResponse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		payload string
		invalid bool
	}{
		{`{"access_token":"token","expires_in":3600,"token_type":"Bearer",` +
			`"refresh_token_expires_in":86400,"future":{"ok":true}}`, false},
		{`{"access_token":"token","expires_in":3600,"token_type":"Bearer","refresh_token_expires_in":-1}`, true},
		{`{"access_token":"token","expires_in":3600,"token_type":"Bearer","refresh_token_expires_in":null}`, true},
		{`{"access_token":"token","expires_in":3600,"token_type":"Bearer","scope":null}`, true},
	}
	for _, test := range cases {
		client, err := transport.NewClient(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusOK, io.NopCloser(strings.NewReader(test.payload))), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		auth, ok := client.(sdm.AuthClient)
		if !ok {
			t.Fatal("missing auth interface")
		}

		result, err := auth.RefreshToken(context.Background(), sdm.RefreshTokenRequest{
			ClientId: "id", ClientSecret: "secret", RefreshToken: "refresh",
		})

		var failure *sdm.Error

		if test.invalid {
			if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse {
				t.Fatalf("response=%s err=%v", test.payload, err)
			}
		} else if err != nil || result.Credentials.RefreshTokenExpiresIn == nil ||
			*result.Credentials.RefreshTokenExpiresIn != 86400 {
			t.Fatalf("response=%s err=%v result=%v", test.payload, err, result)
		}
	}
}

func TestPullSuccessValidatesOriginalResponse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		payload string
		invalid bool
	}{
		{`{}`, false},
		{`{"receivedMessages":[],"future":{"ok":true}}`, false},
		{`{"receivedMessages":null}`, true},
		{`{"receivedMessages":[null]}`, true},
		{`{"receivedMessages":[{"message":{"attributes":{"key":null}}}]}`, true},
		{`{"receivedMessages":[{"deliveryAttempt":null}]}`, true},
	}
	for _, test := range cases {
		client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusOK, io.NopCloser(strings.NewReader(test.payload))), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.Pull(context.Background(), "cloud-token", "projects/project/subscriptions/events",
			wire.PullRequest{MaxMessages: 1, ReturnImmediately: nil})

		var failure *transport.Error

		if test.invalid && (!errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidResponse) {
			t.Fatalf("response=%s err=%v", test.payload, err)
		}

		if !test.invalid && err != nil {
			t.Fatalf("response=%s err=%v", test.payload, err)
		}
	}
}

func TestAcknowledgementRequiresObjectResponse(t *testing.T) {
	t.Parallel()

	const futureResponse = `{"future":true}`

	for _, payload := range []string{testNullJSON, `[]`, `"acknowledged"`, futureResponse} {
		client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusOK, io.NopCloser(strings.NewReader(payload))), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		err = client.Acknowledge(context.Background(), "cloud-token", "projects/project/subscriptions/events",
			wire.AcknowledgeRequest{AckIds: []string{"ack"}})
		if (err == nil) != (payload == futureResponse) {
			t.Fatalf("response=%s err=%v", payload, err)
		}

		err = client.ModifyAckDeadline(context.Background(), "cloud-token", "projects/project/subscriptions/events",
			wire.ModifyAckDeadlineRequest{AckIds: []string{"ack"}, AckDeadlineSeconds: 0})
		if (err == nil) != (payload == futureResponse) {
			t.Fatalf("response=%s err=%v", payload, err)
		}
	}
}
