package replay_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestTypedSyntheticPresenceAndUnknownFields(t *testing.T) {
	t.Parallel()

	input := `{"name":"enterprises/synthetic/devices/synthetic","futureOuter":{"keep":true},` +
		`"traits":{"sdm.devices.traits.CameraMotion":{},` +
		`"sdm.devices.traits.ThermostatTemperatureSetpoint":{"heatCelsius":0},` +
		`"sdm.devices.traits.Connectivity":{"status":"FUTURE_STATUS"},` +
		`"future.traits.Capability":{"keep":true}}}`

	device, err := sdm.DecodeDevice([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(device)
	if err != nil {
		t.Fatal(err)
	}

	var original, roundTrip map[string]any

	err = json.Unmarshal([]byte(input), &original)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(encoded, &roundTrip)
	if err != nil {
		t.Fatal(err)
	}

	traits, traitsPresent := roundTrip["traits"].(map[string]any)
	if !traitsPresent {
		t.Fatal("traits object missing")
	}

	if traits["sdm.devices.traits.CameraMotion"] == nil ||
		traits["future.traits.Capability"] == nil ||
		roundTrip["futureOuter"] == nil {
		t.Fatal("presence or unknown fields lost")
	}

	setpoint, setpointPresent := traits["sdm.devices.traits.ThermostatTemperatureSetpoint"].(map[string]any)
	if !setpointPresent {
		t.Fatal("setpoint object missing")
	}

	if heat, present := setpoint["heatCelsius"]; !present || heat != float64(0) {
		t.Fatal("present zero setpoint lost")
	}

	if _, present := setpoint["coolCelsius"]; present {
		t.Fatal("absent cool setpoint invented")
	}
}

func TestTypedSyntheticMalformedKnownPayloads(t *testing.T) {
	t.Parallel()

	cases := []string{
		`{}`,
		`null`,
		`{"name":"synthetic","traits":{"sdm.devices.traits.Temperature":{"ambientTemperatureCelsius":"warm"}}}`,
		`{"name":"synthetic","traits":{"sdm.devices.traits.Temperature":null}}`,
		`{"name":"synthetic","traits":{"sdm.devices.traits.CameraMotion":[]}}`,
		`{"name":"synthetic","traits":{"sdm.devices.traits.ThermostatMode":{"availableModes":"HEAT"}}}`,
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			_, err := sdm.DecodeDevice([]byte(input))
			if err == nil {
				t.Fatal("malformed known payload accepted")
			}
		})
	}
}

func TestTypedSyntheticEveryEventFamily(t *testing.T) {
	t.Parallel()

	names := []string{"CameraMotion.Motion",
		"CameraPerson.Person",
		"CameraSound.Sound",
		"DoorbellChime.Chime",
		"CameraClipPreview.ClipPreview"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			payload := `{"eventId":"synthetic-inner","eventSessionId":"synthetic-session","futureField":1}`
			if name == "CameraClipPreview.ClipPreview" {
				payload = `{"eventSessionId":"synthetic-session","previewUrl":"https://example.invalid/clip","futureField":1}`
			}

			input := `{"eventId":"synthetic-outer","userId":"synthetic-user",` +
				`"timestamp":"2026-10-03T12:00:00Z","resourceUpdate":{"name":"synthetic-resource",` +
				`"events":{"sdm.devices.events.` + name + `":` + payload + `,"future.events.Event":{"keep":true}}}}`

			event, err := sdm.DecodeEvent([]byte(input))
			if err != nil {
				t.Fatal(err)
			}

			encoded, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(string(encoded), "synthetic-outer") ||
				!strings.Contains(string(encoded), "future.events.Event") ||
				!strings.Contains(string(encoded), "futureField") {
				t.Fatal("event identity or future fields lost")
			}
		})
	}
}

func TestTypedSyntheticPartialAndRelationEvents(t *testing.T) {
	t.Parallel()

	inputs := []string{
		`{"eventId":"synthetic-newer","userId":"synthetic-user","timestamp":"2026-10-03T12:00:01Z",` +
			`"resourceUpdate":{"name":"synthetic-resource",` +
			`"traits":{"sdm.devices.traits.ThermostatMode":{"mode":"HEAT"}}}}`,
		`{"eventId":"synthetic-older","userId":"synthetic-user","timestamp":"2026-10-03T12:00:00Z",` +
			`"resourceUpdate":{"name":"synthetic-resource",` +
			`"traits":{"sdm.devices.traits.ThermostatMode":{"mode":"OFF"}}}}`,
		`{"eventId":"synthetic-relation","userId":"synthetic-user",` +
			`"timestamp":"2026-10-03T12:00:00Z","relationUpdate":{"type":"CREATED","subject":"",` +
			`"object":"synthetic-resource"}}`,
	}
	for _, input := range inputs {
		_, err := sdm.DecodeEvent([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err := sdm.DecodeEvent([]byte(inputs[0]))
	if err != nil {
		t.Fatal("decode must not deduplicate delivery", err)
	}
}
