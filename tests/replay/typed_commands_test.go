package replay_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
)

func TestEveryTypedCommandReplay(t *testing.T) {
	t.Parallel()

	client, err := transport.NewClient(transport.WithHTTPClient(
		replayHTTPClient(loadTransport(t, "fixtures/synthetic/commands.json"))))
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile("fixtures/synthetic/commands.json")
	if err != nil {
		t.Fatal(err)
	}

	var transcript fixture

	err = json.Unmarshal(data, &transcript)
	if err != nil {
		t.Fatal(err)
	}

	methods := []string{
		"SetFanTimer", "SetThermostatEcoMode", "SetThermostatMode", "SetHeat", "SetCool", "SetRange",
		"GenerateImage", "GenerateRtspStream", "ExtendRtspStream", "StopRtspStream",
		"GenerateWebRtcStream", "ExtendWebRtcStream", "StopWebRtcStream",
	}
	if len(transcript.Exchanges) != len(methods) {
		t.Fatal("command inventory differs")
	}

	for index, methodName := range methods {
		method := reflect.ValueOf(client).MethodByName(methodName)
		if !method.IsValid() {
			t.Fatalf("method %s missing", methodName)
		}

		request := reflect.New(method.Type().In(1))

		var envelope map[string]json.RawMessage

		err = json.Unmarshal([]byte(transcript.Exchanges[index].Request.Body), &envelope)
		if err != nil {
			t.Fatal(err)
		}

		err = json.Unmarshal(envelope["params"], request.Elem().FieldByName("Params").Addr().Interface())
		if err != nil {
			t.Fatal(err)
		}

		request.Elem().FieldByName("Auth").FieldByName("AccessToken").SetString(accessToken)
		request.Elem().FieldByName("DeviceName").SetString(deviceName)

		outcomes := method.Call([]reflect.Value{reflect.ValueOf(t.Context()), request.Elem()})
		if !outcomes[1].IsNil() {
			t.Fatalf("%s: %v", methodName, outcomes[1].Interface())
		}

		actual, marshalErr := json.Marshal(outcomes[0].FieldByName("Results").Interface())
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}

		var expectedEnvelope map[string]json.RawMessage

		err = json.Unmarshal([]byte(transcript.Exchanges[index].Response.Body), &expectedEnvelope)
		if err != nil {
			t.Fatal(err)
		}

		expected := expectedEnvelope["results"]
		if len(expected) == 0 {
			expected = json.RawMessage(`{}`)
		}

		var actualValue, expectedValue any

		err = json.Unmarshal(actual, &actualValue)
		if err != nil {
			t.Fatal(err)
		}

		err = json.Unmarshal(expected, &expectedValue)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(actualValue, expectedValue) {
			t.Fatalf("%s result differs: got %s, want %s", methodName, actual, expected)
		}
	}
}
