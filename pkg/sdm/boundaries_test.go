package sdm_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const nullJSON = "null"

func TestCommandRejectsUnrepresentableNumberAndBlankIdentifier(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		command sdm.CommandName
		data    string
	}{
		{sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat, `{"heatCelsius":1e400}`},
		{sdm.SdmDevicesCommandsCameraEventImageGenerateImage, `{
  "eventId": " \t "
}`},
	} {
		err := sdm.ValidateCommandParams(test.command, json.RawMessage(test.data))
		assertPublicFailure(t, err, sdm.ErrorInvalidRequest)
	}
}

func TestSDPRejectsIncompleteCodecMappings(t *testing.T) {
	t.Parallel()

	offers := map[string]string{
		"wrong version": strings.ReplaceAll(validSyntheticOffer, "v=0", "v=1"),
		"missing payload": strings.ReplaceAll(validSyntheticOffer,
			"m=audio 9 UDP/TLS/RTP/SAVPF 111", "m=audio 9 UDP/TLS/RTP/SAVPF"),
		"malformed mapping": strings.ReplaceAll(validSyntheticOffer,
			"a=rtpmap:111 opus/48000/2", "a=rtpmap:111"),
		"undeclared mapping": strings.ReplaceAll(validSyntheticOffer,
			"a=rtpmap:111", "a=rtpmap:112"),
		"unmapped payload": strings.ReplaceAll(validSyntheticOffer,
			"m=audio 9 UDP/TLS/RTP/SAVPF 111", "m=audio 9 UDP/TLS/RTP/SAVPF 111 112"),
	}
	for name, offer := range offers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := sdm.ValidateCommandParams(sdm.SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream,
				json.RawMessage(offerParamsJSON(t, offer)))
			assertPublicFailure(t, err, sdm.ErrorInvalidRequest)
		})
	}
}

func TestDecodeRejectsUnrepresentableKnownNumbers(t *testing.T) {
	t.Parallel()

	_, err := sdm.DecodeDevice([]byte(`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.Temperature": {
      "ambientTemperatureCelsius": Infinity
    }
  }
}`))
	assertPublicFailure(t, err, sdm.ErrorInvalidResponse)
}

func TestPreflightPermitsSupportedNonSetpointCommand(t *testing.T) {
	t.Parallel()

	device, err := sdm.DecodeDevice([]byte(`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.Fan": {}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	err = sdm.CheckCommand(sdm.CheckCommandRequest{Device: device, Command: sdm.SdmDevicesCommandsFanSetTimer})
	if err != nil {
		t.Fatal("supported fan command rejected", err)
	}
}

func TestHandlerRequiresContext(t *testing.T) {
	t.Parallel()

	var missingContext context.Context

	_, err := sdm.HandleEvent(missingContext, sdm.HandleEventRequest{Data: []byte(eventJSON)},
		func(context.Context, sdm.EventEnvelope) error {
			t.Fatal("event dispatched without context")

			return nil
		})
	assertPublicFailure(t, err, sdm.ErrorInvalidRequest)
}

func assertPublicFailure(t *testing.T, err error, kind sdm.ErrorKind) {
	t.Helper()

	var typed *sdm.Error
	if !errors.As(err, &typed) || typed.Kind != kind || typed.Unwrap() == nil {
		t.Fatalf("expected typed %s with cause, got %v", kind, err)
	}
}

func TestReconcileRejectsMalformedCallerStateAndEvent(t *testing.T) {
	t.Parallel()

	event, err := sdm.DecodeEvent([]byte(eventJSON))
	if err != nil {
		t.Fatal(err)
	}

	var state sdm.DeviceState

	state.Device.Name = event.ResourceUpdate.Name
	state.Device.AdditionalProperties = map[string]json.RawMessage{"caller.extension": json.RawMessage("invalid")}
	_, err = sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})
	assertPublicFailure(t, err, sdm.ErrorInvalidResponse)

	state.Device.AdditionalProperties = map[string]json.RawMessage{"name": json.RawMessage("1")}
	_, err = sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})
	assertPublicFailure(t, err, sdm.ErrorInvalidResponse)

	state.Device.AdditionalProperties = nil
	event.AdditionalProperties = map[string]json.RawMessage{"caller.extension": json.RawMessage("invalid")}
	_, err = sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})
	assertPublicFailure(t, err, sdm.ErrorInvalidResponse)

	event.AdditionalProperties = nil
	event.Timestamp = time.Time{}
	_, err = sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})
	assertPublicFailure(t, err, sdm.ErrorInvalidResponse)
}

func TestReconcilePreservesExistingParentMetadata(t *testing.T) {
	t.Parallel()

	device, err := sdm.DecodeDevice([]byte(`{
  "name": "enterprises/example/devices/device",
  "parentRelations": [
    {
      "parent": "enterprises/example/structures/home",
      "displayName": "Kitchen"
    }
  ]
}`))
	if err != nil {
		t.Fatal(err)
	}

	event, err := sdm.DecodeEvent([]byte(`{
  "eventId": "relation",
  "timestamp": "2026-10-04T00:00:00Z",
  "userId": "user",
  "relationUpdate": {
    "type": "UPDATED",
    "subject": "enterprises/example/structures/home",
    "object": "enterprises/example/devices/device"
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	var state sdm.DeviceState

	state.Device = device

	result, err := sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})
	if err != nil {
		t.Fatal(err)
	}

	parent := (*result.State.Device.ParentRelations)[0]
	if parent.DisplayName == nil || *parent.DisplayName != "Kitchen" || result.Disposition != sdm.ReconcileApplied {
		t.Fatal("relation refresh discarded known parent metadata")
	}

	*parent.DisplayName = "Changed"

	if *(*device.ParentRelations)[0].DisplayName != "Kitchen" {
		t.Fatal("reconciled result shares caller-owned parent storage")
	}
}

func TestReconcileEventWithoutTraitChangesRetainsSnapshot(t *testing.T) {
	t.Parallel()

	event, err := sdm.DecodeEvent([]byte(eventJSON))
	if err != nil {
		t.Fatal(err)
	}

	device, err := sdm.DecodeDevice([]byte(`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.Temperature": {
      "ambientTemperatureCelsius": 20
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	var state sdm.DeviceState

	state.Device = device

	result, err := sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})
	if err != nil || result.Disposition != sdm.ReconcileApplied ||
		*result.State.Device.Traits.SdmDevicesTraitsTemperature.AmbientTemperatureCelsius != 20 {
		t.Fatal("camera event discarded unrelated snapshot traits", err)
	}
}

func TestDecodeRejectsImageDimensionsOutsideGoIntegerRange(t *testing.T) {
	t.Parallel()

	_, err := sdm.DecodeDevice([]byte(`{
 "name":"enterprises/example/devices/device",
 "traits":{"sdm.devices.traits.CameraImage":{
 "maxImageResolution":{"width":9223372036854775808,"height":1}
 }}
 }`))
	assertPublicFailure(t, err, sdm.ErrorInvalidResponse)
}

func TestReconcileRejectsInvalidPriorKnownTrait(t *testing.T) {
	t.Parallel()

	event, err := sdm.DecodeEvent([]byte(eventJSON))
	if err != nil {
		t.Fatal(err)
	}

	event.ResourceUpdate.Traits = new(sdm.Traits)

	var state sdm.DeviceState

	state.Device.Name = event.ResourceUpdate.Name
	state.Device.Traits = new(sdm.Traits)
	state.Device.Traits.SdmDevicesTraitsThermostatMode = new(sdm.ThermostatMode)
	mode := sdm.ThermostatModeValue("invalid lowercase mode")
	state.Device.Traits.SdmDevicesTraitsThermostatMode.Mode = &mode

	_, err = sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})
	assertPublicFailure(t, err, sdm.ErrorInvalidResponse)
}

func TestReconcileOpaqueNestedPatchPreservesNumbersAndReplacesCollections(t *testing.T) {
	t.Parallel()

	device, err := sdm.DecodeDevice([]byte(`{
  "name":"enterprises/example/devices/device",
  "traits":{"future.trait":{
    "nested":{"integer":9007199254740993,"keep":true,"changed":"before"},
    "array":[1,2],"nullable":{"before":true},"scalar":false
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}

	event, err := sdm.DecodeEvent([]byte(`{
  "eventId":"opaque-patch","timestamp":"2026-10-04T00:00:00Z","userId":"user",
  "resourceUpdate":{"name":"enterprises/example/devices/device","traits":{
    "future.trait":{
      "nested":{"changed":"after","newInteger":9007199254740995},
      "array":[3],"nullable":null,"scalar":{"now":"object"}
    }
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}

	var state sdm.DeviceState

	state.Device = device

	result, err := sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage

	err = json.Unmarshal(result.State.Device.Traits.AdditionalProperties["future.trait"], &fields)
	if err != nil {
		t.Fatal(err)
	}

	var nested map[string]json.RawMessage

	err = json.Unmarshal(fields["nested"], &nested)
	if err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]string{
		"integer": "9007199254740993", "keep": "true", "changed": `"after"`, "newInteger": "9007199254740995",
	} {
		if string(nested[name]) != want {
			t.Errorf("nested %s = %s, want %s", name, nested[name], want)
		}
	}

	if string(fields["array"]) != "[3]" || string(fields["nullable"]) != nullJSON ||
		string(fields["scalar"]) != `{"now":"object"}` {
		t.Fatal("patch did not replace arrays, nulls, and scalar values")
	}

	if !strings.Contains(string(device.Traits.AdditionalProperties["future.trait"]), `"before"`) ||
		!strings.Contains(string(event.ResourceUpdate.Traits.AdditionalProperties["future.trait"]), `"after"`) {
		t.Fatal("patch mutated its caller-owned inputs")
	}
}
