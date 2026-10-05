package replay_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"gopkg.in/yaml.v3"
)

const (
	eventResourceUpdate = "resourceUpdate"
	eventRelationUpdate = "relationUpdate"
)

type eventPresenceBranch struct {
	Title    string                `yaml:"title"`
	Required []string              `yaml:"required"`
	AnyOf    []eventPresenceBranch `yaml:"anyOf"`
}

type eventPresenceComponents struct {
	Schemas map[string]eventPresenceBranch `yaml:"schemas"`
}

type eventPresenceDocument struct {
	Components eventPresenceComponents `yaml:"components"`
}

func TestEventPresenceVariantsHaveNamesAndCommonRequirements(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../api/asyncapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var document eventPresenceDocument

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		t.Fatal(err)
	}

	envelope := document.Components.Schemas["EventEnvelope"]
	titles := []string{"Resource update", "Relation update"}
	fields := []string{eventResourceUpdate, eventRelationUpdate}

	if len(envelope.AnyOf) != len(titles) {
		t.Fatal("event presence variant count changed")
	}

	for index, branch := range envelope.AnyOf {
		if branch.Title != titles[index] {
			t.Fatalf("event variant title %q", branch.Title)
		}

		expected := append(append([]string(nil), envelope.Required...), fields[index])
		if len(branch.Required) != len(expected) {
			t.Fatalf("variant %q does not repeat common requirements", branch.Title)
		}

		for i, key := range expected {
			if branch.Required[i] != key {
				t.Fatalf("variant %q is missing common or variant requirement %q", branch.Title, key)
			}
		}
	}
}

func TestNamedEventPresenceBranchesRetainEnvelopeSemantics(t *testing.T) {
	t.Parallel()

	updates := map[string]json.RawMessage{
		eventResourceUpdate: json.RawMessage(`{
 "name":"enterprises/example/devices/example",
 "traits":{"sdm.devices.traits.Temperature":{"ambientTemperatureCelsius":21}}
}`),
		eventRelationUpdate: json.RawMessage(`{
 "type":"CREATED",
 "subject":"enterprises/example/structures/example",
 "object":"enterprises/example/devices/example"
}`),
	}

	variants := [][]string{
		{eventResourceUpdate}, {eventRelationUpdate}, {eventResourceUpdate, eventRelationUpdate}, {},
	}
	for _, fields := range variants {
		payload := map[string]any{
			"eventId": "synthetic-envelope-id", "timestamp": "2026-01-01T12:00:00Z", "userId": "synthetic-user",
		}
		for _, field := range fields {
			payload[field] = updates[field]
		}

		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}

		var decoded wire.EventEnvelope

		err = json.Unmarshal(encoded, &decoded)
		if len(fields) == 0 {
			if err == nil {
				t.Fatal("generated decoder lost update variant requirement")
			}

			continue
		}

		if err != nil {
			t.Fatal(err)
		}

		validateFixturePayload(t, "events.openapi.yaml", "EventEnvelope", encoded)

		for _, key := range []string{"eventId", "timestamp", "userId"} {
			value := payload[key]
			delete(payload, key)

			missing, encodeErr := json.Marshal(payload)
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}

			validationErr := contracts.Validate("events.openapi.yaml", "EventEnvelope", missing)
			if validationErr == nil {
				t.Fatalf("accepted missing %q", key)
			}

			decodeErr := json.Unmarshal(missing, &decoded)
			if decodeErr == nil {
				t.Fatalf("generated decoder accepted missing %q", key)
			}

			payload[key] = value
		}
	}
}
