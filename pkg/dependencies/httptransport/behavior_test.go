package httptransport_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const (
	behaviorRoom               = "enterprises/e/structures/s/rooms/r"
	behaviorStructure          = "enterprises/e/structures/s"
	behaviorDiscoveryStructure = "enterprises/err/structures/s"
	behaviorDiscoveryRoom      = "enterprises/err/structures/s/rooms/r"
	behaviorCallback           = "https://example.test/callback"
	behaviorCode               = "code"
	behaviorSecret             = "secret"
)

func TestSDKCommandsRejectInvalidInputWithoutSending(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
		t.Error("invalid command reached transport")

		return nil, io.ErrUnexpectedEOF
	})))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"fan", func() error {
			var invalidRequest sdm.SetFanTimerRequest

			_, err := client.SetFanTimer(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"eco", func() error {
			var invalidRequest sdm.SetThermostatEcoModeRequest

			_, err := client.SetThermostatEcoMode(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"mode", func() error {
			var invalidRequest sdm.SetThermostatModeRequest

			_, err := client.SetThermostatMode(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"heat", func() error {
			_, err := client.SetHeat(context.Background(), sdm.SetHeatRequest{
				Auth: sdm.AuthContext{AccessToken: ""}, DeviceName: "",
				Params: sdm.ThermostatTemperatureSetpointSetHeatParams{HeatCelsius: math.NaN()},
			})

			return behaviorError(err)
		}},
		{"cool", func() error {
			_, err := client.SetCool(context.Background(), sdm.SetCoolRequest{
				Auth: sdm.AuthContext{AccessToken: ""}, DeviceName: "",
				Params: sdm.ThermostatTemperatureSetpointSetCoolParams{CoolCelsius: math.Inf(1)},
			})

			return behaviorError(err)
		}},
		{"range", func() error {
			var invalidRequest sdm.SetRangeRequest

			_, err := client.SetRange(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"image", func() error {
			var invalidRequest sdm.GenerateImageRequest

			_, err := client.GenerateImage(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"rtsp", func() error {
			var invalidRequest sdm.GenerateRtspStreamRequest

			_, err := client.GenerateRtspStream(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"extend rtsp", func() error {
			var invalidRequest sdm.ExtendRtspStreamRequest

			_, err := client.ExtendRtspStream(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"stop rtsp", func() error {
			var invalidRequest sdm.StopRtspStreamRequest

			_, err := client.StopRtspStream(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"webrtc", func() error {
			var invalidRequest sdm.GenerateWebRtcStreamRequest

			_, err := client.GenerateWebRtcStream(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"extend webrtc", func() error {
			var invalidRequest sdm.ExtendWebRtcStreamRequest

			_, err := client.ExtendWebRtcStream(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
		{"stop webrtc", func() error {
			var invalidRequest sdm.StopWebRtcStreamRequest

			_, err := client.StopWebRtcStream(context.Background(), invalidRequest)

			return behaviorError(err)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var failure *sdm.Error
			if !errors.As(test.call(), &failure) || failure.Kind != sdm.ErrorInvalidRequest {
				t.Fatalf("expected invalid request: %v", failure)
			}
		})
	}
}

func TestSDKDiscoveryConvertsResourcesAndRejectsMalformedLists(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, body string
		call       func(sdm.Client) error
	}{
		{"structures", `{"structures":[{"name":"enterprises/err/structures/s"}]}`, func(c sdm.Client) error {
			v, err := c.ListStructures(context.Background(), sdm.ListStructuresRequest{
				Auth:   sdm.AuthContext{AccessToken: testAccountToken},
				Parent: "enterprises/err",
			})
			if err == nil && (len(v.Structures) != 1 || v.Structures[0].Name != behaviorDiscoveryStructure) {
				t.Error("lost structure")
			}

			return behaviorError(err)
		}},
		{"rooms", `{"rooms":[{"name":"enterprises/err/structures/s/rooms/r"}]}`, func(c sdm.Client) error {
			v, err := c.ListRooms(context.Background(), sdm.ListRoomsRequest{
				Auth:   sdm.AuthContext{AccessToken: testAccountToken},
				Parent: behaviorDiscoveryStructure,
			})
			if err == nil && (len(v.Rooms) != 1 || v.Rooms[0].Name != behaviorDiscoveryRoom) {
				t.Error("lost room")
			}

			return behaviorError(err)
		}},
		{"structure", `{"name":"enterprises/err/structures/s"}`, func(c sdm.Client) error {
			v, err := c.GetStructure(context.Background(), sdm.GetStructureRequest{
				Auth: sdm.AuthContext{AccessToken: testAccountToken},
				Name: behaviorDiscoveryStructure,
			})
			if err == nil && v.Structure.Name != behaviorDiscoveryStructure {
				t.Error("lost structure")
			}

			return behaviorError(err)
		}},
		{"room", `{"name":"enterprises/err/structures/s/rooms/r"}`, func(c sdm.Client) error {
			v, err := c.GetRoom(context.Background(), sdm.GetRoomRequest{
				Auth: sdm.AuthContext{AccessToken: testAccountToken},
				Name: behaviorDiscoveryRoom,
			})
			if err == nil && v.Room.Name != behaviorDiscoveryRoom {
				t.Error("lost room")
			}

			return behaviorError(err)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			checkDiscoveryBodies(t, test.body, test.call)
		})
	}
}

func TestSDKDiscoveryRejectsResourceNamesWithoutSending(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
		t.Error("invalid name sent")

		return nil, io.ErrUnexpectedEOF
	})))
	if err != nil {
		t.Fatal(err)
	}

	calls := []func() error{
		func() error {
			var invalidRequest sdm.ListDevicesRequest

			_, err := client.ListDevices(context.Background(), invalidRequest)

			return behaviorError(err)
		},
		func() error {
			var invalidRequest sdm.GetDeviceRequest

			_, err := client.GetDevice(context.Background(), invalidRequest)

			return behaviorError(err)
		},
		func() error {
			var invalidRequest sdm.ListStructuresRequest

			_, err := client.ListStructures(context.Background(), invalidRequest)

			return behaviorError(err)
		},
		func() error {
			var invalidRequest sdm.GetStructureRequest

			_, err := client.GetStructure(context.Background(), invalidRequest)

			return behaviorError(err)
		},
		func() error {
			var invalidRequest sdm.ListRoomsRequest

			_, err := client.ListRooms(context.Background(), invalidRequest)

			return behaviorError(err)
		},
		func() error {
			var invalidRequest sdm.GetRoomRequest

			_, err := client.GetRoom(context.Background(), invalidRequest)

			return behaviorError(err)
		},
	}
	for _, call := range calls {
		var failure *sdm.Error
		if !errors.As(call(), &failure) || failure.Kind != sdm.ErrorInvalidRequest {
			t.Fatalf("unexpected failure %v", failure)
		}
	}
}

func TestSDKExchangeTokenReturnsCallerOwnedCredentials(t *testing.T) {
	t.Parallel()

	verifier := "synthetic-verifier"

	client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(
		func(request *http.Request) (*http.Response, error) {
			err := request.ParseForm()
			if err != nil {
				t.Fatal(err)
			}

			if request.Form.Get(behaviorCode) != behaviorCode || request.Form.Get("code_verifier") != verifier ||
				request.Form.Get("redirect_uri") != behaviorCallback {
				t.Error("exchange inputs changed")
			}

			return sdkResponse(http.StatusOK, `{"access_token":"access","token_type":"Bearer","expires_in":3600,`+
				`"refresh_token":"refresh","refresh_token_expires_in":7200,"scope":"scope"}`), nil
		},
	)))
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ExchangeToken(context.Background(), sdm.ExchangeTokenRequest{
		ClientId: "client", ClientSecret: behaviorSecret,
		Code:        behaviorCode,
		RedirectUri: behaviorCallback, CodeVerifier: &verifier,
	})
	if err != nil {
		t.Fatal(err)
	}

	v := result.Credentials
	if v.AccessToken != "access" || v.RefreshToken == nil || *v.RefreshToken != "refresh" ||
		v.RefreshTokenExpiresIn == nil || *v.RefreshTokenExpiresIn != 7200 ||
		v.Scope == nil || *v.Scope != "scope" {
		t.Fatalf("credentials incomplete: %#v", v)
	}

	var invalidRequest sdm.ExchangeTokenRequest

	_, err = client.ExchangeToken(context.Background(), invalidRequest)

	var failure *sdm.Error

	if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidRequest {
		t.Fatalf("invalid credentials accepted: %v", err)
	}
}

func TestLowLevelResourceNameValidation(t *testing.T) {
	t.Parallel()

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
		t.Error("invalid input sent")

		return nil, io.ErrUnexpectedEOF
	})))
	if err != nil {
		t.Fatal(err)
	}

	calls := []func() error{func() error {
		_, err := client.ListDevices(context.Background(), "token", "")

		return behaviorError(err)
	}, func() error {
		_, err := client.ListStructures(context.Background(), "token", "")

		return behaviorError(err)
	}, func() error {
		_, err := client.GetStructure(context.Background(), "token", "")

		return behaviorError(err)
	}, func() error {
		_, err := client.ListRooms(context.Background(), "token", "")

		return behaviorError(err)
	}, func() error {
		_, err := client.GetRoom(context.Background(), "token", "")

		return behaviorError(err)
	}, func() error {
		var invalidRequest wire.ExecuteCommandRequest

		_, err := client.ExecuteCommand(context.Background(), "token", "", invalidRequest)

		return behaviorError(err)
	}}
	for _, call := range calls {
		var failure *transport.Error
		if !errors.As(call(), &failure) || failure.Kind != transport.ErrorInvalidRequest {
			t.Fatalf("unexpected failure %v", failure)
		}
	}
}

func TestHTTPMissingResponseReadFailureAndInvalidAuthorization(t *testing.T) {
	t.Parallel()

	response := testResponse(http.StatusOK, &trackedBody{Reader: providerErrorReader{}, closed: false})

	t.Cleanup(func() {
		err := response.Body.Close()
		if err != nil {
			t.Error(err)
		}
	})

	responses := []*http.Response{nil, response}

	for _, response := range responses {
		client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
			return response, nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.GetDevice(context.Background(), "token", testDeviceName)

		var failure *transport.Error

		if !errors.As(err, &failure) {
			t.Fatalf("untyped failure %v", err)
		}

		if response == nil && failure.Kind != transport.ErrorInvalidResponse {
			t.Fatal(err)
		}

		tracked, ok := responseBody(response)
		if response != nil && (!errors.Is(err, io.ErrUnexpectedEOF) || !ok || !tracked.closed) {
			t.Fatal("read cause or cleanup lost")
		}
	}

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
		t.Error("invalid request sent")

		return nil, io.ErrUnexpectedEOF
	})))
	if err != nil {
		t.Fatal(err)
	}

	for _, token := range []string{"", "token\r\nX: secret"} {
		_, err = client.GetDevice(context.Background(), token, testDeviceName)

		var failure *transport.Error

		if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidRequest {
			t.Fatal(err)
		}
	}
}

func TestOAuthInputValidation(t *testing.T) {
	t.Parallel()

	client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
		t.Error("invalid oauth sent")

		return nil, io.ErrUnexpectedEOF
	})))
	if err != nil {
		t.Fatal(err)
	}

	empty := ""
	code := behaviorCode

	redirect := behaviorCallback

	for _, input := range []wire.OAuthTokenRequest{
		{ClientId: "id", ClientSecret: behaviorSecret,
			Code: nil, CodeVerifier: nil,
			GrantType: "", RedirectUri: nil, RefreshToken: nil},
		{ClientId: "id", ClientSecret: behaviorSecret,
			Code: &code, RedirectUri: &redirect, CodeVerifier: &empty,
			GrantType: "", RefreshToken: nil},
	} {
		_, err = client.OAuthExchange(context.Background(), input)

		var failure *transport.Error

		if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidRequest {
			t.Fatal(err)
		}
	}

	_, err = client.OAuthRefresh(context.Background(), wire.OAuthTokenRequest{ClientId: "id", ClientSecret: behaviorSecret,
		Code: nil, CodeVerifier: nil,
		GrantType: "", RedirectUri: nil, RefreshToken: nil})

	var failure *transport.Error

	if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidRequest {
		t.Fatal(err)
	}
}

func TestOtherBaseOriginsAndProviderErrorLimits(t *testing.T) {
	t.Parallel()

	for _, body := range []io.ReadCloser{nil, io.NopCloser(strings.NewReader(strings.Repeat("x", (8<<20)+1)))} {
		client, err := transport.New(transport.WithHTTPClient(testDoer(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusBadRequest, body), nil
		})))
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.GetDevice(context.Background(), "token", testDeviceName)

		var failure *transport.Error

		if !errors.As(err, &failure) || failure.Kind != transport.ErrorRejected {
			t.Fatal(err)
		}
	}

	_, err := transport.NewClient(nil)

	var failure *sdm.Error

	if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidRequest {
		t.Fatal(err)
	}
}

func responseBody(response *http.Response) (*trackedBody, bool) {
	if response == nil {
		return nil, false
	}

	body, ok := response.Body.(*trackedBody)

	return body, ok
}

func behaviorError(err error) error {
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}

	return nil
}

func checkDiscoveryBodies(t *testing.T, validBody string, call func(sdm.Client) error) {
	t.Helper()

	for _, invalid := range []bool{false, true} {
		body := validBody
		if invalid {
			body = `{"name":null,"rooms":false,"structures":[null]}`
		}

		client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(
			func(*http.Request) (*http.Response, error) {
				return sdkResponse(http.StatusOK, body), nil
			},
		)))
		if err != nil {
			t.Fatal(err)
		}

		err = call(client)

		if invalid {
			var failure *sdm.Error
			if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse {
				t.Fatalf("malformed resource accepted: %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}

func TestConfiguredServiceOrigins(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		option            transport.Option
		valid, pubSub     bool
		endpoint, payload string
	}{
		{"pubsub valid", transport.WithPubSubBaseURL("https://example.test/v1/"), true, true,
			"https://example.test/v1/projects/p/subscriptions/s:pull", `{}`},
		{"oauth valid", transport.WithOAuthBaseURL("https://example.test/"), true, false,
			"https://example.test/token", `{"access_token":"access","expires_in":3600,"token_type":"Bearer"}`},
		{"pubsub invalid", transport.WithPubSubBaseURL("%"), false, true, "", ""},
		{"oauth invalid", transport.WithOAuthBaseURL("%"), false, false, "", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			client, err := transport.New(test.option, transport.WithHTTPClient(testDoer(
				func(request *http.Request) (*http.Response, error) {
					calls++

					if request.URL.String() != test.endpoint || request.Method != http.MethodPost {
						t.Errorf("request = %s %s; expected POST %s", request.Method, request.URL, test.endpoint)
					}

					return sdkResponse(http.StatusOK, test.payload), nil
				},
			)))

			if !test.valid {
				var failure *transport.Error

				if !errors.As(err, &failure) || failure.Kind != transport.ErrorInvalidRequest || client != nil || calls != 0 {
					t.Fatalf("invalid origin accepted: client=%v err=%v calls=%d", client, err, calls)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if test.pubSub {
				_, err = client.Pull(context.Background(), testAccountToken, "projects/p/subscriptions/s",
					wire.PullRequest{MaxMessages: 1, ReturnImmediately: nil})
			} else {
				input := new(wire.OAuthTokenRequest)
				refresh := "synthetic-refresh"
				input.ClientId = "synthetic-client"
				input.ClientSecret = "synthetic-secret"
				input.RefreshToken = &refresh

				result, oauthError := client.OAuthRefresh(context.Background(), *input)
				err = oauthError

				if err == nil && result.AccessToken != "access" {
					t.Error("lost exchanged access token")
				}
			}

			if err != nil || calls != 1 {
				t.Fatalf("valid origin exchange: err=%v calls=%d", err, calls)
			}
		})
	}
}
