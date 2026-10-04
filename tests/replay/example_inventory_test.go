package replay_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type traitExampleComponents struct {
	Schemas map[string]json.RawMessage `json:"schemas"`
}

type traitExampleDocument struct {
	Components traitExampleComponents `json:"components"`
}

type traitExampleCatalog struct {
	Properties map[string]documentationSchema `json:"properties"`
}

type traitPayloadExample struct {
	Example json.RawMessage `json:"example"`
}

func TestOperationExampleInventory(t *testing.T) {
	t.Parallel()

	documents := []string{
		"openapi.yaml", "external/oauth.openapi.yaml", "external/pubsub.openapi.yaml", "external/media.openapi.yaml",
	}
	for _, filename := range documents {
		t.Run(filename, func(t *testing.T) {
			t.Parallel()

			data, err := fs.ReadFile(os.DirFS("../../api"), filename)
			if err != nil {
				t.Fatal(err)
			}

			var document documentationREST

			err = json.Unmarshal(data, &document)
			if err != nil {
				t.Fatal(err)
			}

			assertDocumentExamples(t, filename, document)
		})
	}
}

func assertDocumentExamples(t *testing.T, filename string, document documentationREST) {
	t.Helper()

	for path, methods := range document.Paths {
		for method, operation := range methods {
			label := path + " " + method

			for media, content := range operation.RequestBody.Content {
				assertOperationExamples(t, filename, label+" request "+media, content)
			}

			for status, response := range operation.Responses {
				assertResponseExamples(t, filename, label, status, response)
			}
		}
	}
}

func assertResponseExamples(t *testing.T, filename, label, status string, response documentationContent) {
	t.Helper()

	for media, content := range response.Content {
		if content.Schema.Format == "binary" {
			if !strings.Contains(response.Description, "opaque binary bytes") {
				t.Fatal("binary response lacks honest payload explanation")
			}

			continue
		}

		assertOperationExamples(t, filename, label+" response "+status+" "+media, content)
	}

	if len(response.Content) > 0 {
		return
	}

	switch {
	case strings.HasPrefix(filename, "external/media"):
		if !strings.Contains(response.Description, "HTTP 403") {
			t.Fatal("media failure lacks status illustration")
		}
	case status == "302":
		assertCallbackQueryExamples(t, response.Description)
	default:
		t.Fatalf("unexplained response without payload %s %s %s", filename, label, status)
	}
}

func assertCallbackQueryExamples(t *testing.T, description string) {
	t.Helper()

	blocks := strings.Split(description, "```json\n")

	const expectedBlockCount = 3

	if len(blocks) != expectedBlockCount || !strings.Contains(description, `"code"`) ||
		!strings.Contains(description, `"error"`) || !strings.Contains(description, `"state"`) {
		t.Fatal("authorization callback lacks granted and denied query examples")
	}

	for _, block := range blocks[1:] {
		payload, _, found := strings.Cut(block, "\n```")
		if !found {
			t.Fatal("callback query example has no closing delimiter")
		}

		validateFixturePayload(t, "oauth.openapi.yaml", "OAuthAuthorizationCallback", []byte(payload))
	}
}

func assertOperationExamples(t *testing.T, filename, label string, content documentationMedia) {
	t.Helper()

	if len(content.Examples) == 0 {
		t.Fatalf("missing complete payload examples: %s", label)
	}

	file, component, ok := strings.Cut(content.Schema.Ref, "#/components/schemas/")
	if !ok {
		t.Fatalf("example schema is not named: %s", label)
	}

	if file == "" {
		file = filepath.Base(filename)
	} else {
		file = filepath.Base(file)
	}

	for name, example := range content.Examples {
		if len(example.Value) == 0 {
			t.Fatalf("empty example %s %s", label, name)
		}

		validateFixturePayload(t, file, component, example.Value)
	}
}

func TestEveryKnownTraitHasSyntheticPayloadExample(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../api/traits.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var document traitExampleDocument

	err = json.Unmarshal(data, &document)
	if err != nil {
		t.Fatal(err)
	}

	var catalog traitExampleCatalog

	err = json.Unmarshal(document.Components.Schemas["Traits"], &catalog)
	if err != nil {
		t.Fatal(err)
	}

	for name, reference := range catalog.Properties {
		component := strings.TrimPrefix(reference.Ref, "#/components/schemas/")

		var schema traitPayloadExample

		err = json.Unmarshal(document.Components.Schemas[component], &schema)
		if err != nil {
			t.Fatal(err)
		}

		if len(schema.Example) == 0 {
			t.Fatalf("trait lacks example: %s", name)
		}

		validateFixturePayload(t, "traits.openapi.yaml", component, schema.Example)
	}
}
