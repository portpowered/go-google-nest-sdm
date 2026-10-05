package httptransport_test

import (
	"context"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestSDKAuthorizationSession(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	authorization, ok := client.(sdm.AuthorizationClient)
	if !ok {
		t.Fatal("HTTP client lacks authorization session support")
	}

	attempt, err := authorization.OpenAuthorizationSession(context.Background(), sdm.OpenAuthorizationSessionRequest{
		ProjectId: "project", ClientId: "authorization-client", ClientSecret: "synthetic-secret",
		RedirectUri: "http://localhost:8765/callback",
	})
	if err != nil {
		t.Fatal(err)
	}

	if attempt.AuthorizationURL() == "" {
		t.Fatal("missing consent URL")
	}

	err = attempt.Close()
	if err != nil {
		t.Fatal(err)
	}
}
