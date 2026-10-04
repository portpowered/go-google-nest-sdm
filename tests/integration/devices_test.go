//go:build integration

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const integrationTimeout = 30 * time.Second

func TestLiveListDevices(t *testing.T) {
	t.Parallel()

	accessToken := os.Getenv("NEST_ACCESS_TOKEN")
	enterpriseID := os.Getenv("NEST_ENTERPRISE_ID")
	if accessToken == "" || enterpriseID == "" {
		t.Skip("set NEST_ACCESS_TOKEN and NEST_ENTERPRISE_ID to run against Google")
	}

	client, err := httptransport.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	_, err = client.ListDevices(ctx, sdm.ListDevicesRequest{
		Auth:   sdm.AuthContext{AccessToken: accessToken},
		Parent: "enterprises/" + enterpriseID,
		Filter: nil,
	})
	if err != nil {
		// Do not include account/device payloads in test output.
		t.Fatal("live ListDevices failed")
	}
}
