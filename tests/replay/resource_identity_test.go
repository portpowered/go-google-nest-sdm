package replay_test

import (
	"errors"
	"fmt"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestResourceIdentityPairedReplay(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(
		replayHTTPClient(loadTransport(t, "fixtures/synthetic/resource-account-identity.json"))))
	if err != nil {
		t.Fatal(err)
	}

	for _, operation := range []string{"GetDevice", "ListStructures"} {
		for index := range 3 {
			identity, err := replayResourceIdentity(t, client, operation)

			switch index {
			case 0:
				if err != nil || identity == nil || identity.UserId != "synthetic-authorized-user" {
					t.Fatal("paired authenticated resource identity missing")
				}
			case 1:
				if err != nil || identity != nil {
					t.Fatal("paired missing header retained previous identity")
				}
			case 2:
				var failure *sdm.Error
				if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse ||
					failure.Operation != operation || identity != nil {
					t.Fatal("paired ambiguous resource identity accepted")
				}
			}
		}
	}
}

func replayResourceIdentity(t *testing.T, client sdm.Client, operation string) (*sdm.AccountIdentity, error) {
	t.Helper()

	auth := sdm.AuthContext{AccessToken: accessToken}
	if operation == "GetDevice" {
		result, err := client.GetDevice(t.Context(), sdm.GetDeviceRequest{Auth: auth, Name: deviceName})
		if err != nil {
			return nil, fmt.Errorf("paired device identity: %w", err)
		}

		return result.AccountIdentity, nil
	}

	result, err := client.ListStructures(t.Context(), sdm.ListStructuresRequest{Auth: auth, Parent: enterprise})
	if err != nil {
		return nil, fmt.Errorf("paired structures identity: %w", err)
	}

	return result.AccountIdentity, nil
}
