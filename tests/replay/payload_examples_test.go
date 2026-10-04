package replay_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
	"gopkg.in/yaml.v3"
)

type documentationExample struct {
	Value json.RawMessage `json:"value"`
}

type documentationMedia struct {
	Examples map[string]documentationExample `json:"examples"`
	Schema   documentationSchema             `json:"schema"`
}

type documentationSchema struct {
	Ref    string `json:"$ref"`
	Format string `json:"format"`
}

type documentationContent struct {
	Content     map[string]documentationMedia `json:"content"`
	Description string                        `json:"description"`
}

type documentationOperation struct {
	RequestBody documentationContent            `json:"requestBody"`
	Responses   map[string]documentationContent `json:"responses"`
}

type documentationREST struct {
	Paths map[string]map[string]documentationOperation `json:"paths"`
}

type documentationCommand struct {
	Command string          `json:"command"`
	Params  json.RawMessage `json:"params"`
}

type documentationResult struct {
	Results json.RawMessage `json:"results"`
}

type documentationEventExample struct {
	Name    string         `yaml:"name"`
	Payload map[string]any `yaml:"payload"`
}

type documentationMessage struct {
	Examples []documentationEventExample `yaml:"examples"`
}

type documentationEventComponents struct {
	Messages map[string]documentationMessage `yaml:"messages"`
}

type documentationEvents struct {
	Components documentationEventComponents `yaml:"components"`
}

func TestExecuteCommandDocumentationExamples(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var document documentationREST

	err = json.Unmarshal(data, &document)
	if err != nil {
		t.Fatal(err)
	}

	operation := document.Paths["/enterprises/{enterpriseId}/devices/{deviceId}:executeCommand"]["post"]
	requests := operation.RequestBody.Content["application/json"].Examples
	responses := operation.Responses["200"].Content["application/json"].Examples

	const commandCount = 13

	if len(requests) != commandCount || len(responses) != commandCount {
		t.Fatalf("command examples: requests=%d responses=%d", len(requests), len(responses))
	}

	for name, example := range requests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			validateFixturePayload(t, "openapi.yaml", "ExecuteCommandRequest", example.Value)
			validateFixturePayload(t, "openapi.yaml", name+"Request", example.Value)

			var request documentationCommand

			decodeErr := json.Unmarshal(example.Value, &request)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}

			validateFixturePayload(t, "commands.openapi.yaml", name+"Params", request.Params)

			response, exists := responses[name]
			if !exists {
				t.Fatal("missing command-specific response example")
			}

			validateFixturePayload(t, "openapi.yaml", "ExecuteCommandResponse", response.Value)

			var result documentationResult

			decodeErr = json.Unmarshal(response.Value, &result)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}

			validateFixturePayload(t, "commands.openapi.yaml", name+"Results", result.Results)
			// A known command cannot fall through the open future-command branch.
			invalid, marshalErr := json.Marshal(map[string]any{
				"command": request.Command, "params": map[string]any{"unregisteredParameter": true},
			})
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}

			validationErr := contracts.Validate("openapi.yaml", "ExecuteCommandRequest", invalid)
			if validationErr == nil {
				t.Fatal("accepted mismatched known command parameters")
			}
		})
	}

	future := `{"command":"sdm.devices.commands.Future.Example","params":{"callerDefined":true}}`
	validateFixturePayload(t, "openapi.yaml", "ExecuteCommandRequest", []byte(future))

	mismatch := `{"command":"sdm.devices.commands.ThermostatTemperatureSetpoint.SetHeat","params":{"coolCelsius":25}}`

	err = contracts.Validate("openapi.yaml", "ExecuteCommandRequest", []byte(mismatch))
	if err == nil {
		t.Fatal("accepted another command's params")
	}
}

func TestAsyncAPIDocumentationExamples(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../api/asyncapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var document documentationEvents

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		t.Fatal(err)
	}

	examples := document.Components.Messages["SDMEvent"].Examples

	expectedNames := map[string]bool{
		"ThermostatTraitUpdate":   false,
		"ResourceRelationCREATED": false, "ResourceRelationUPDATED": false, "ResourceRelationDELETED": false,
		"ResourceRelationWithoutStructureAccess": false,
		"CameraMotionMotion":                     false, "CameraPersonPerson": false, "CameraSoundSound": false,
		"DoorbellChimeChime": false, "CameraClipPreviewClipPreview": false,
	}
	if len(examples) != len(expectedNames) {
		t.Fatalf("event examples: %d", len(examples))
	}

	for _, example := range examples {
		seen, expected := expectedNames[example.Name]
		if !expected || seen {
			t.Fatalf("unexpected or duplicate event example %q", example.Name)
		}

		expectedNames[example.Name] = true

		t.Run(example.Name, func(t *testing.T) {
			t.Parallel()

			encoded, encodeErr := json.Marshal(example.Payload)
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}

			validateFixturePayload(t, "events.openapi.yaml", "EventEnvelope", encoded)
		})
	}
}
