package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
)

const resourcesDocument = "openapi.yaml"
const listDevicesResponseSchema = "ListDevicesResponse"
const deviceResponseSchema = "Device"
const structuresResponseSchema = "ListStructuresResponse"

type schemaFixture struct {
	name      string
	document  string
	requests  []string
	responses []string
}

func TestSyntheticFixtureSchemas(t *testing.T) {
	t.Parallel()

	cases := []schemaFixture{
		{name: "resource-account-identity", document: resourcesDocument, requests: nil,
			responses: []string{
				deviceResponseSchema, deviceResponseSchema, deviceResponseSchema,
				structuresResponseSchema, structuresResponseSchema, structuresResponseSchema,
			}},
		{name: "account-identity", document: resourcesDocument, requests: nil,
			responses: []string{listDevicesResponseSchema, listDevicesResponseSchema, listDevicesResponseSchema}},
		{name: "rest-resources", document: resourcesDocument, requests: nil,
			responses: []string{
				listDevicesResponseSchema, deviceResponseSchema, structuresResponseSchema, "Structure", "ListRoomsResponse", "Room",
			}},
		{name: "pubsub-lifecycle", document: "pubsub.openapi.yaml",
			requests:  []string{"PullRequest", "ModifyAckDeadlineRequest", "AcknowledgeRequest"},
			responses: []string{"PullResponse", "EmptyResponse", "EmptyResponse"}},
		{name: "oauth", document: "oauth.openapi.yaml", requests: nil,
			responses: []string{"OAuthTokenResponse", "OAuthTokenResponse"}},
		{name: "oauth-error", document: "oauth.openapi.yaml", requests: nil,
			responses: []string{"OAuthErrorResponse"}},
		{name: "provider-error", document: "errors.openapi.yaml", requests: nil,
			responses: []string{"GoogleErrorResponse"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			recorded := readSchemaFixture(t, test.name)
			if len(recorded.Exchanges) != len(test.responses) {
				t.Fatal("schema fixture exchange count differs")
			}

			for index, paired := range recorded.Exchanges {
				if len(test.requests) != 0 {
					validateFixturePayload(t, test.document, test.requests[index], []byte(paired.Request.Body))
				}

				validateFixturePayload(t, test.document, test.responses[index], []byte(paired.Response.Body))
			}

			if test.name == "pubsub-lifecycle" {
				validatePubSubEvent(t, recorded.Exchanges[0].Response.Body)
			}
		})
	}

	t.Run("commands", func(t *testing.T) {
		t.Parallel()

		recorded := readSchemaFixture(t, "commands")
		if len(recorded.Exchanges) != 13 {
			t.Fatal("all thirteen command schemas require fixtures")
		}

		for _, paired := range recorded.Exchanges {
			validateFixturePayload(t, resourcesDocument, "ExecuteCommandRequest", []byte(paired.Request.Body))
			validateFixturePayload(t, resourcesDocument, "ExecuteCommandResponse", []byte(paired.Response.Body))

			var request map[string]json.RawMessage

			err := json.Unmarshal([]byte(paired.Request.Body), &request)
			if err != nil {
				t.Fatal(err)
			}

			var command string

			err = json.Unmarshal(request["command"], &command)
			if err != nil {
				t.Fatal(err)
			}

			component := strings.ReplaceAll(strings.TrimPrefix(command, "sdm.devices.commands."), ".", "")
			validateFixturePayload(t, "commands.openapi.yaml", component+"Params", request["params"])

			var response map[string]json.RawMessage

			err = json.Unmarshal([]byte(paired.Response.Body), &response)
			if err != nil {
				t.Fatal(err)
			}

			results := response["results"]
			if len(results) == 0 {
				results = json.RawMessage(`{}`)
			}

			validateFixturePayload(t, "commands.openapi.yaml", component+"Results", results)
		}
	})
}

func readSchemaFixture(t *testing.T, name string) fixture {
	t.Helper()

	root, err := os.OpenRoot("fixtures/synthetic")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := root.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	file, err := root.Open(name + ".json")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := file.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	var recorded fixture

	err = json.Unmarshal(data, &recorded)
	if err != nil {
		t.Fatal(err)
	}

	if recorded.Provenance != "synthetic" {
		t.Fatal("schema fixture requires synthetic provenance")
	}

	return recorded
}

func validateFixturePayload(t *testing.T, document, component string, data []byte) {
	t.Helper()

	err := contracts.Validate(document, component, data)
	if err != nil {
		t.Fatal(err)
	}
}

func validatePubSubEvent(t *testing.T, body string) {
	t.Helper()

	var response map[string]json.RawMessage

	err := json.Unmarshal([]byte(body), &response)
	if err != nil {
		t.Fatal(err)
	}

	var received []map[string]json.RawMessage

	err = json.Unmarshal(response["receivedMessages"], &received)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range received {
		var message map[string]json.RawMessage

		err = json.Unmarshal(item["message"], &message)
		if err != nil {
			t.Fatal(err)
		}

		var encoded string

		err = json.Unmarshal(message["data"], &encoded)
		if err != nil {
			t.Fatal(err)
		}

		data, decodeErr := base64.StdEncoding.DecodeString(encoded)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		validateFixturePayload(t, "events.openapi.yaml", "EventEnvelope", data)
	}
}
