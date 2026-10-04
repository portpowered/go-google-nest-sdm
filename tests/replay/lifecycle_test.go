package replay_test

import (
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestOAuthReplay(t *testing.T) {
	t.Parallel()
	client := replayClient(t, "oauth")
	code, redirect, refresh := "synthetic-code", "https://example.invalid/callback", "synthetic-refresh-token"
	input := wire.OAuthTokenRequest{ClientId: "synthetic-client",
		ClientSecret: "synthetic-secret",
		Code:         &code,
		RedirectUri:  &redirect,
		CodeVerifier: nil,
		GrantType:    "",
		RefreshToken: nil}

	credentials, err := client.OAuthExchange(t.Context(), input)
	if err != nil || credentials.AccessToken != accessToken {
		t.Fatalf("exchange: %#v %v", credentials, err)
	}

	input.Code = nil
	input.RedirectUri = nil
	input.RefreshToken = &refresh

	credentials, err = client.OAuthRefresh(t.Context(), input)
	if err != nil || credentials.AccessToken != "synthetic-access-token-next" {
		t.Fatalf("refresh: %#v %v", credentials, err)
	}
}

func TestPubSubLifecycleReplay(t *testing.T) {
	t.Parallel()
	client := replayClient(t, "pubsub-lifecycle")
	ctx := t.Context()

	const subscription = "projects/synthetic-project/subscriptions/synthetic-subscription"

	batch, err := client.Pull(ctx, accessToken, subscription, wire.PullRequest{MaxMessages: 1, ReturnImmediately: nil})
	if err != nil || len(*batch.ReceivedMessages) != 1 {
		t.Fatalf("pull: %#v %v", batch, err)
	}

	err = client.ModifyAckDeadline(ctx,
		accessToken,
		subscription,
		wire.ModifyAckDeadlineRequest{AckIds: []string{"synthetic-ack"},
			AckDeadlineSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}

	err = client.Acknowledge(ctx, accessToken, subscription, wire.AcknowledgeRequest{AckIds: []string{"synthetic-ack"}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublicPubSubLifecycleReplay(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(
		replayHTTPClient(loadTransport(t, "fixtures/synthetic/pubsub-lifecycle.json"))))
	if err != nil {
		t.Fatal(err)
	}

	events, ok := client.(sdm.EventClient)
	if !ok {
		t.Fatal("event client missing")
	}

	session, err := events.OpenEventSession(t.Context(), sdm.OpenEventSessionRequest{
		Auth:         sdm.AuthContext{AccessToken: accessToken},
		Subscription: "projects/synthetic-project/subscriptions/synthetic-subscription", MaxMessages: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := session.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	delivery, err := session.Next(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if delivery.Event().EventId != "synthetic-envelope-event" {
		t.Fatal("public event identity differs")
	}

	err = delivery.ModifyAckDeadline(t.Context(), sdm.AckDeadlineRequest{Seconds: 30})
	if err != nil {
		t.Fatal(err)
	}

	err = delivery.Acknowledge(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	err = session.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, err = session.Next(t.Context())
	if err == nil {
		t.Fatal("closed session returned delivery")
	}
}
