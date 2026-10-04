package httptransport_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const boundaryBrokenJSON = "{broken"
const boundaryRefresh = "refresh"
const boundaryParent = "enterprises/e"

func TestSDKFilteredDiscoveryRetainsDeviceAndFilter(t *testing.T) {
	t.Parallel()

	filter := "type = synthetic & shared"

	client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(
		request *http.Request,
	) (*http.Response, error) {
		if request.URL.Query().Get("filter") != filter {
			t.Error("filter changed")
		}

		return sdkResponse(http.StatusOK, `{"devices":[{"name":"enterprises/e/devices/d"}]}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListDevices(context.Background(), sdm.ListDevicesRequest{
		Auth: sdm.AuthContext{AccessToken: testAccountToken}, Parent: boundaryParent, Filter: &filter,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Devices) != 1 || result.Devices[0].Name != testShortDeviceName {
		t.Fatalf("lost device: %#v", result)
	}
}

func TestSDKMalformedCommandResultsRemainTyped(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
		return sdkResponse(http.StatusOK, `{"results":{"url":123,"token":false}}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GenerateImage(context.Background(), sdm.GenerateImageRequest{
		Auth: sdm.AuthContext{AccessToken: testAccountToken}, DeviceName: testShortDeviceName,
		Params: sdm.CameraEventImageGenerateImageParams{EventId: testEventID},
	})

	var failure *sdm.Error

	if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse || failure.Cause == nil {
		t.Fatalf("unexpected failure: %v", err)
	}
}

func TestLowLevelCommandRejectsMalformedCallerJSON(t *testing.T) {
	t.Parallel()

	client, err := transport.New(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
		t.Error("malformed command sent")

		return nil, context.Canceled
	})))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.ExecuteCommand(context.Background(), testAccountToken, testShortDeviceName, wire.ExecuteCommandRequest{
		Command: wire.CommandName(sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat), Params: json.RawMessage(`{`),
	})

	var failure *transport.Error

	if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidRequest || failure.Cause == nil {
		t.Fatalf("unexpected failure: %v", err)
	}
}

func TestOAuthRejectsEmptySuccessfulCredentialsAndMalformedError(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		`{"access_token":"","expires_in":3600,"token_type":"Bearer"}`,
		`{"access_token":"access","expires_in":0,"token_type":"Bearer"}`,
		`{"access_token":"access","expires_in":3600,"token_type":""}`,
		boundaryBrokenJSON,
	} {
		client, err := transport.New(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
			status := http.StatusOK
			if payload == boundaryBrokenJSON {
				status = http.StatusBadRequest
			}

			return sdkResponse(status, payload), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		refresh := boundaryRefresh
		input := new(wire.OAuthTokenRequest)
		input.ClientId = "client"
		input.ClientSecret = "secret"
		input.RefreshToken = &refresh
		_, err = client.OAuthRefresh(context.Background(), *input)

		var failure *transport.Error

		if !errors.As(err, &failure) {
			t.Fatalf("untyped failure %v", err)
		}

		expected := transport.ErrorInvalidResponse
		if payload == boundaryBrokenJSON {
			expected = transport.ErrorRejected
		}

		if failure.Kind != expected {
			t.Fatalf("kind = %s; expected %s", failure.Kind, expected)
		}
	}
}

func TestSDKRejectsMalformedAndNullDiscoveryLists(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{`{"structures":true,"rooms":true}`, `{"structures":null,"rooms":null}`} {
		client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
			return sdkResponse(http.StatusOK, payload), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		auth := sdm.AuthContext{AccessToken: testAccountToken}
		_, structureError := client.ListStructures(context.Background(), sdm.ListStructuresRequest{
			Auth: auth, Parent: boundaryParent,
		})

		_, roomError := client.ListRooms(context.Background(), sdm.ListRoomsRequest{
			Auth: auth, Parent: "enterprises/e/structures/s",
		})

		for _, err := range []error{structureError, roomError} {
			var failure *sdm.Error
			if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse {
				t.Fatalf("invalid list accepted: %v", err)
			}
		}
	}
}
