package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestControlCommandsUsePublicTypedSDK(t *testing.T) {
	t.Parallel()

	commands := []string{
		"timer", "eco", "mode", "heat", "cool", "range", "image", "rtsp-start", "rtsp-extend", "rtsp-stop",
		"webrtc-start", "webrtc-extend", "webrtc-stop",
	}
	groups := []string{
		"fan", thermostatGroup, thermostatGroup, thermostatGroup, thermostatGroup, thermostatGroup,
		cameraGroup, cameraGroup, cameraGroup, cameraGroup, cameraGroup, cameraGroup, cameraGroup,
	}

	for index, command := range commands {
		pair := loadPairs(t, "commands", index)

		var payload commandEnvelope

		err := json.Unmarshal([]byte(pair[0].body), &payload)
		if err != nil {
			t.Fatal(err)
		}

		app, output := newTestApplication(t, pair, string(payload.Params))

		err = app.run(context.Background(), []string{
			groups[index], command, resourceFlag, syntheticDevice, paramsFlag, "-",
		})
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}

		if groups[index] == cameraGroup && !strings.Contains(output.String(), `"acknowledged":true`) {
			t.Fatalf("media output: %s", output)
		}
	}
}

type commandEnvelope struct {
	Params json.RawMessage `json:"params"`
}

func TestRequiredCommandFieldsCannotBecomeZeroValues(t *testing.T) {
	t.Parallel()

	for _, input := range []string{`{}`, `null`, `{"heatCelsius":null}`, `{"heatCelsius":20,"unexpected":true}`} {
		_, err := readCommandParams[sdm.ThermostatTemperatureSetpointSetHeatParams](
			"-", strings.NewReader(input), sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat,
		)
		if err == nil {
			t.Fatalf("accepted invalid params: %s", input)
		}
	}
}

func TestExplicitRefreshExportRetainsRefreshCredential(t *testing.T) {
	t.Parallel()

	app, output := newTestApplication(t, loadPairs(t, "oauth", 1), "")

	err := app.run(context.Background(), []string{authGroup, "refresh", exportFlag})
	if err != nil {
		t.Fatal(err)
	}

	var account credentials

	err = json.Unmarshal(output.Bytes(), &account)
	if err != nil {
		t.Fatal(err)
	}

	if account.RefreshToken != syntheticRefreshToken || account.AccessToken != "synthetic-access-token-next" {
		t.Fatalf("renewed account: %+v", account)
	}
}
