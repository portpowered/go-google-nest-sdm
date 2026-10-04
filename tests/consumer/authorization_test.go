package consumer_test

import (
	"net/url"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/authorization"
	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestCallerOwnedAuthorization(t *testing.T) {
	t.Parallel()

	client, err := httptransport.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	if _, supported := client.(sdm.AuthorizationClient); !supported {
		t.Fatal("HTTP client does not expose authorization")
	}

	session, err := authorization.NewSession(t.Context(), client, sdm.OpenAuthorizationSessionRequest{
		ClientId: "synthetic-client", ClientSecret: "synthetic-secret", ProjectId: "synthetic-project",
		RedirectUri: "http://127.0.0.1:8080/oauth/callback",
	})
	if err != nil {
		t.Fatal(err)
	}

	consent, err := url.Parse(session.AuthorizationURL())
	if err != nil {
		t.Fatal(err)
	}

	if consent.Host != "nestservices.google.com" || consent.Query().Get("state") == "" ||
		consent.Query().Get("code_challenge") == "" {
		t.Fatalf("invalid public consent URL: %v", consent)
	}

	for range 2 {
		if closeErr := session.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}
