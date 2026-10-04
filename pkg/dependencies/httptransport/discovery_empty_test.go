package httptransport_test

import (
	"context"
	"net/http"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestSDKEmptyDiscoveryReturnsNonNilEmptyLists(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
		return sdkResponse(http.StatusOK, `{}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListDevices(context.Background(), sdm.ListDevicesRequest{
		Auth: sdm.AuthContext{AccessToken: testAccountToken}, Parent: "enterprises/e", Filter: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Devices == nil || len(result.Devices) != 0 {
		t.Fatalf("expected empty discovery list: %#v", result.Devices)
	}
}
