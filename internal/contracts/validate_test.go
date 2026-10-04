package contracts

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/api"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestValidateEvents(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		data  string
		valid bool
	}{
		{"empty identity", `{"eventId":"","userId":"u","timestamp":"2026-01-01T00:00:00Z"}`, false},
		{"missing identity", `{"eventId":"e","timestamp":"2026-01-01T00:00:00Z"}`, false},
		{"invalid timestamp", `{"eventId":"e","userId":"u","timestamp":"yesterday"}`, false},
		{"null known field", `{"eventId":"e","userId":"u","timestamp":"2026-01-01T00:00:00Z","resourceUpdate":null}`, false},
		{"lowercase relation", `{"eventId":"e","userId":"u","timestamp":"2026-01-01T00:00:00Z","relationUpdate":{"type":"created","subject":"","object":"device"}}`, false},
		{"empty relation target", `{"eventId":"e","userId":"u","timestamp":"2026-01-01T00:00:00Z","relationUpdate":{"type":"CREATED","subject":"","object":""}}`, false},
		{"http clip", `{"eventId":"e","userId":"u","timestamp":"2026-01-01T00:00:00Z","resourceUpdate":{"name":"device","events":{"sdm.devices.events.CameraClipPreview.ClipPreview":{"eventSessionId":"s","previewUrl":"http://example.com/clip"}}}}`, false},
		{"missing clip session", `{"eventId":"e","userId":"u","timestamp":"2026-01-01T00:00:00Z","resourceUpdate":{"name":"device","events":{"sdm.devices.events.CameraClipPreview.ClipPreview":{"previewUrl":"https://example.com/clip"}}}}`, false},
		{"wrong known trait type", `{"eventId":"e","userId":"u","timestamp":"2026-01-01T00:00:00Z","resourceUpdate":{"name":"device","traits":{"sdm.devices.traits.Temperature":{"ambientTemperatureCelsius":"20"}}}}`, false},
		{"partial update future values", `{"eventId":"e","userId":"u","timestamp":"2026-01-01T00:00:00Z","resourceUpdate":{"name":"device","traits":{"sdm.devices.traits.ThermostatMode":{"mode":"FUTURE_MODE"},"future.trait":{"unknown":null}},"events":{"future.event":{"unknown":1}}},"extra":true}`, true},
		{"future relation empty subject", `{"eventId":"e","userId":"u","timestamp":"2026-01-01T00:00:00Z","relationUpdate":{"type":"FUTURE_STATE","subject":"","object":"device"}}`, true},
		{"clip without image event", `{"eventId":"e","userId":"u","timestamp":"2026-01-01T00:00:00Z","resourceUpdate":{"name":"device","events":{"sdm.devices.events.CameraClipPreview.ClipPreview":{"eventSessionId":"s","previewUrl":"https://example.com/clip"}}}}`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := Validate("events.openapi.yaml", "EventEnvelope", []byte(test.data))
			if (err == nil) != test.valid {
				t.Fatalf("valid = %v, error = %v", test.valid, err)
			}
		})
	}
}

func TestValidateCommands(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		data  string
		valid bool
	}{
		{"required", `{}`, false},
		{"null", `{"timerMode":null}`, false},
		{"closed outbound enum", `{"timerMode":"FUTURE_MODE"}`, false},
		{"zero duration", `{"timerMode":"ON","duration":"0s"}`, false},
		{"duration above bound", `{"timerMode":"ON","duration":"43201s"}`, false},
		{"unknown outbound field", `{"timerMode":"ON","typo":1}`, false},
		{"optional default duration", `{"timerMode":"ON"}`, true},
		{"upper duration bound", `{"timerMode":"ON","duration":"43200s"}`, true},
		{"lower duration bound", `{"timerMode":"ON","duration":"1s"}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := Validate("client-models.openapi.yaml", "FanSetTimerParams", []byte(test.data))
			if (err == nil) != test.valid {
				t.Fatalf("valid = %v, error = %v", test.valid, err)
			}
		})
	}
}

func TestValidateDevice(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		data  string
		valid bool
	}{
		{`{"name":"enterprises/project/devices/device","type":"sdm.devices.types.FUTURE","traits":{"sdm.devices.traits.Fan":{}}}`, true},
		{`{"name":"enterprises/project/devices/"}`, false},
		{`{"name":"enterprises/project/devices/device","traits":{"sdm.devices.traits.Fan":{"timerMode":"on"}}}`, false},
		{`{"name":"enterprises/project/devices/device","traits":{"sdm.devices.traits.Fan":null}}`, false},
	} {
		err := Validate("client-resources.openapi.yaml", "Device", []byte(test.data))
		if (err == nil) != test.valid {
			t.Fatalf("valid = %v, error = %v", test.valid, err)
		}
	}
}

func TestAllEmbeddedComponentsCompile(t *testing.T) {
	t.Parallel()
	entries, err := fs.ReadDir(api.RuntimeSchemas, "contracts")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, readErr := api.RuntimeSchemas.ReadFile("contracts/" + entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		value, decodeErr := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		document, ok := value.(map[string]any)
		if !ok {
			t.Fatal("contract is not an object")
		}
		components, ok := document["components"].(map[string]any)
		if !ok {
			t.Fatal("components is not an object")
		}
		schemas, ok := components["schemas"].(map[string]any)
		if !ok {
			t.Fatal("schemas is not an object")
		}
		for name := range schemas {
			if _, compileErr := runtimeRegistry.compile(entry.Name(), name); compileErr != nil {
				t.Errorf("%s/%s: %v", entry.Name(), name, compileErr)
			}
		}
	}
}

func TestErrorPreservesCause(t *testing.T) {
	t.Parallel()
	err := Validate("events.openapi.yaml", "EventEnvelope", []byte(`{}`))
	var contractError *Error
	var validationError *jsonschema.ValidationError
	if !errors.As(err, &contractError) || !errors.As(err, &validationError) {
		t.Fatalf("missing typed error cause: %v", err)
	}
	if contractError.Document != "events.openapi.yaml" || contractError.Component != "EventEnvelope" {
		t.Fatalf("incorrect error contract: %v", err)
	}
}

func TestOfflineLoaderRejectsExternalReferences(t *testing.T) {
	t.Parallel()
	for _, location := range []string{"https://example.com/schema", "file:///etc/passwd", "https://sdm.local/contracts/missing.yaml", "https://sdm.local/contracts/events.openapi.yaml?token=x"} {
		if _, err := (offlineLoader{}).Load(location); err == nil {
			t.Errorf("accepted noninventory URL %s", location)
		}
	}
	if err := Validate("../events.openapi.yaml", "EventEnvelope", []byte(`{}`)); err == nil {
		t.Fatal("accepted traversal document")
	}
}
