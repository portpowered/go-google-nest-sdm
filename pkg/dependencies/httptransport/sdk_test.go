package httptransport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

type sdkDoer func(*http.Request) (*http.Response, error)

func (do sdkDoer) Do(request *http.Request) (*http.Response, error) { return do(request) }

func TestSDKPreservesResourceFieldsAndRejectsNull(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, body string
		invalid    bool
	}{
		{
			"future fields",
			`{"name":"enterprises/e/devices/d","future":{"active":true},"traits":{"vendor.Future":{"x":1}}}`,
			false,
		},

		{"null resource", `null`, true},
		{"null name", `{"name":null}`, true},
		{"null known trait", `{"name":"enterprises/e/devices/d","traits":{"sdm.devices.traits.Temperature":null}}`, true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			calls := 0

			client,
				err := transport.NewClient(
				transport.WithHTTPClient(
					sdkDoer(
						func(
							request *http.Request,
						) (
							*http.Response,
							error,
						) {
							calls++

							if request.Method != http.MethodGet || request.URL.Path != "/v1/enterprises/e/devices/d" || request.Header.Get(
								"Authorization",
							) != "Bearer account" {
								t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
							}

							return sdkResponse(http.StatusOK, testCase.body), nil
						})))
			if err != nil {
				t.Fatal(err)
			}

			result,
				err := client.GetDevice(
				context.Background(),
				sdm.GetDeviceRequest{
					Auth: sdm.AuthContext{
						AccessToken: "account",
					},
					Name: "enterprises/e/devices/d",
				},
			)

			if testCase.invalid {
				var failure *sdm.Error
				if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse {
					t.Fatalf("expected invalid response, got %v", err)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if _, ok := result.Device.AdditionalProperties["future"]; !ok {
				t.Fatal("lost future resource field")
			}

			if result.Device.Traits == nil {
				t.Fatal("lost traits")
			}

			if calls != 1 {
				t.Fatalf("calls = %d", calls)
			}
		})
	}
}

func TestSDKMapsErrorsWithoutRetry(t *testing.T) {
	t.Parallel()

	cause := io.ErrUnexpectedEOF
	calls := 0

	client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
		calls++

		return nil, cause
	})))
	if err != nil {
		t.Fatal(err)
	}

	_,
		err = client.SetHeat(
		context.Background(),
		sdm.SetHeatRequest{
			Auth: sdm.AuthContext{
				AccessToken: "account",
			},
			DeviceName: "enterprises/e/devices/d",
			Params: sdm.ThermostatTemperatureSetpointSetHeatParams{
				HeatCelsius: 20,
			},
		},
	)

	var failure *sdm.Error

	if !errors.As(err, &failure) || failure.Kind != sdm.ErrorTransport || !errors.Is(err, cause) {
		t.Fatalf("unexpected error %v", err)
	}

	if calls != 1 {
		t.Fatalf("command retried: %d", calls)
	}
}

func TestSDKRejectsMissingCommandResults(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
		return sdkResponse(http.StatusOK, `{}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	_,
		err = client.GenerateImage(
		context.Background(),
		sdm.GenerateImageRequest{
			Auth: sdm.AuthContext{
				AccessToken: "account",
			},
			DeviceName: "enterprises/e/devices/d",
			Params: sdm.CameraEventImageGenerateImageParams{
				EventId: "event",
			},
		},
	)

	var failure *sdm.Error

	if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestSDKListRejectsNullEntries(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`null`, `{"devices":null}`, `{"devices":[null]}`, `{"devices":[{"name":null}]}`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
				return sdkResponse(http.StatusOK, body), nil
			})))
			if err != nil {
				t.Fatal(err)
			}

			_,
				err = client.ListDevices(
				context.Background(),
				sdm.ListDevicesRequest{
					Auth: sdm.AuthContext{
						AccessToken: "account",
					},
					Parent: "enterprises/e",
					Filter: nil,
				},
			)

			var failure *sdm.Error

			if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}

func TestSDKRetainsHTTPStatus(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
		return sdkResponse(http.StatusUnauthorized, `{}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	_,
		err = client.GetRoom(
		context.Background(),
		sdm.GetRoomRequest{
			Auth: sdm.AuthContext{
				AccessToken: "account",
			},
			Name: "enterprises/e/structures/s/rooms/r",
		},
	)

	var failure *sdm.Error

	if !errors.As(
		err,
		&failure,
	) || failure.Kind != sdm.ErrorUnauthorized || failure.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestSDKEmptyAcknowledgement(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`{}`, `{"results":{}}`, `{"results":null}`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			client, err := transport.NewClient(transport.WithHTTPClient(sdkDoer(func(*http.Request) (*http.Response, error) {
				return sdkResponse(http.StatusOK, body), nil
			})))
			if err != nil {
				t.Fatal(err)
			}

			result,
				err := client.SetHeat(
				context.Background(),
				sdm.SetHeatRequest{
					Auth: sdm.AuthContext{
						AccessToken: "account",
					},
					DeviceName: "enterprises/e/devices/d",
					Params: sdm.ThermostatTemperatureSetpointSetHeatParams{
						HeatCelsius: 20,
					},
				},
			)

			if body == `{"results":null}` {
				var failure *sdm.Error
				if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse {
					t.Fatalf("unexpected error %v", err)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if result.Results == nil {
				t.Fatal("expected empty acknowledgement result")
			}
		})
	}
}

func sdkResponse(status int, body string) *http.Response {
	response := new(http.Response)
	response.StatusCode = status
	response.Body = io.NopCloser(strings.NewReader(body))

	return response
}
