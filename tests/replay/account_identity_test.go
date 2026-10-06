package replay_test

import (
	"errors"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestAccountIdentityPairedReplay(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(
		replayHTTPClient(loadTransport(t, "fixtures/synthetic/account-identity.json"))))
	if err != nil {
		t.Fatal(err)
	}

	request := sdm.ListDevicesRequest{Auth: sdm.AuthContext{AccessToken: accessToken}, Parent: enterprise, Filter: nil}

	result, err := client.ListDevices(t.Context(), request)

	if err != nil || result.AccountIdentity == nil || result.AccountIdentity.UserId != "synthetic-authorized-user" {
		t.Fatalf("authenticated identity missing: %v", err)
	}

	result, err = client.ListDevices(t.Context(), request)
	if err != nil || result.AccountIdentity != nil {
		t.Fatalf("absent header retained prior identity: %v", err)
	}

	result, err = client.ListDevices(t.Context(), request)

	var invalid *sdm.Error

	if !errors.As(err, &invalid) || invalid.Kind != sdm.ErrorInvalidResponse || result.AccountIdentity != nil {
		t.Fatalf("ambiguous identity accepted: %v", err)
	}
}
