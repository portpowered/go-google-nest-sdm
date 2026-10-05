package replay_test

import (
	"encoding/json"
	"testing"

	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
)

func TestGeneratedCommandUnionHelpersPreserveEnvelopeProperties(t *testing.T) {
	t.Parallel()

	var request wire.ExecuteCommandRequest

	variant := wire.FanSetTimerRequest{
		Command: wire.CommandName("sdm.devices.commands.Fan.SetTimer"),
		Params:  wire.FanSetTimerParams{TimerMode: wire.FanSetTimerParamsTimerMode("ON"), Duration: nil},
	}

	err := request.FromFanSetTimerRequest(variant)
	if err != nil {
		t.Fatal(err)
	}

	assertFanCommandEnvelope(t, request, "ON")

	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	validateFixturePayload(t, "openapi.yaml", "ExecuteCommandRequest", encoded)

	var decoded wire.ExecuteCommandRequest

	err = json.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatal(err)
	}

	assertFanCommandEnvelope(t, decoded, "ON")

	variant.Params.TimerMode = wire.FanSetTimerParamsTimerMode("OFF")

	err = decoded.MergeFanSetTimerRequest(variant)
	if err != nil {
		t.Fatal(err)
	}

	assertFanCommandEnvelope(t, decoded, "OFF")

	// Direct field edits must remain visible through generated union accessors.
	decoded.Params = json.RawMessage(`{"timerMode":"ON"}`)
	assertFanCommandEnvelope(t, decoded, "ON")
}

func TestGeneratedCommandUnionRetainsStructLiteralAndFuturePayloads(t *testing.T) {
	t.Parallel()

	known := wire.ExecuteCommandRequest{
		Command: wire.CommandName("sdm.devices.commands.Fan.SetTimer"),
		Params:  json.RawMessage(`{"timerMode":"ON"}`),
	}
	assertFanCommandEnvelope(t, known, "ON")

	future := wire.ExecuteCommandRequest{
		Command: wire.CommandName("sdm.devices.commands.Future.Example"),
		Params:  json.RawMessage(`{"callerDefined":{"integer":9007199254740993,"unknown":null}}`),
	}

	encoded, err := json.Marshal(future)
	if err != nil {
		t.Fatal(err)
	}

	validateFixturePayload(t, "openapi.yaml", "ExecuteCommandRequest", encoded)

	var decoded wire.ExecuteCommandRequest

	err = json.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatal(err)
	}

	if decoded.Command != future.Command || string(decoded.Params) != string(future.Params) {
		t.Fatalf("future payload changed: %s", encoded)
	}

	futureVariant, err := decoded.AsFutureCommandRequest()
	if err != nil {
		t.Fatal(err)
	}

	var fromFuture wire.ExecuteCommandRequest

	err = fromFuture.FromFutureCommandRequest(futureVariant)
	if err != nil {
		t.Fatal(err)
	}

	if fromFuture.Command != future.Command || string(fromFuture.Params) != string(future.Params) {
		t.Fatal("future union constructor changed unknown fields")
	}
}

func assertFanCommandEnvelope(t *testing.T, request wire.ExecuteCommandRequest, mode string) {
	t.Helper()

	variant, err := request.AsFanSetTimerRequest()
	if err != nil {
		t.Fatal(err)
	}

	if variant.Command != wire.CommandName("sdm.devices.commands.Fan.SetTimer") ||
		string(variant.Params.TimerMode) != mode {
		t.Fatalf("union fields lost: command=%q mode=%q", variant.Command, variant.Params.TimerMode)
	}

	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	validateFixturePayload(t, "openapi.yaml", "FanSetTimerRequest", encoded)
}
