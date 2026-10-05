package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalExamples(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		document string
		invalid  bool
	}{
		{name: "required request", document: `openapi: 3.0.3
paths:
  /command:
    post:
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Command'}
            examples:
              bad: {value: {}}
components:
  schemas:
    Command: {type: object, required: [mode], properties: {mode: {type: string}}}
`, invalid: true},
		{name: "response mismatch", document: `openapi: 3.0.3
paths:
  /command:
    post:
      responses:
        '200':
          content:
            application/json:
              schema: {type: integer}
              example: wrong
`, invalid: true},
		{name: "event payload", document: `asyncapi: 3.0.0
components:
  messages:
    Event:
      payload: {$ref: '#/components/schemas/Event'}
      examples: [{payload: {resourceUpdate: wrong}}]
  schemas:
    Event:
      type: object
      properties: {resourceUpdate: {type: object}}
`, invalid: true},
		{name: "nullable referenced projection", document: `openapi: 3.0.3
components:
  schemas:
    Projection:
      type: object
      properties: {mode: {$ref: '#/components/schemas/Mode'}}
      examples: [{mode: null}, {mode: HEAT}]
    Mode: {type: string, nullable: true}
`, invalid: false},
		{name: "pointer escaping", document: `openapi: 3.0.3
components:
  schemas:
    'Mode~/State': {type: integer, example: 4}
    Envelope:
      type: object
      properties: {mode: {$ref: '#/components/schemas/Mode~0~1State'}}
      example: {mode: wrong}
`, invalid: true},
		{name: "parameter mismatch", document: `openapi: 3.0.3
paths:
  /command:
    get:
      parameters:
        - name: count
          schema: {type: integer}
          example: wrong
`, invalid: true},
		{name: "exclusive bound", document: `openapi: 3.0.3
components:
  schemas:
    Temperature: {type: number, minimum: 0, exclusiveMinimum: true, example: 0}
`, invalid: true},
		{name: "external value rejected", document: `openapi: 3.0.3
paths:
  /command:
    post:
      requestBody:
        content:
          application/json:
            schema: {type: object}
            examples: {remote: {externalValue: 'https://example.invalid/payload'}}
`, invalid: true},
		{name: "network reference rejected", document: `openapi: 3.0.3
components:
  schemas:
    Remote: {$ref: 'https://example.invalid/schema', example: {}}
`, invalid: true},
		{name: "example values not walked", document: `openapi: 3.0.3
components:
  schemas:
    Envelope:
      type: object
      example: {schema: {type: integer, example: wrong}}
`, invalid: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := writeSchema(t, test.document)

			err := checkRepository(root)
			if (err != nil) != test.invalid {
				t.Fatalf("invalid = %t, error = %v", test.invalid, err)
			}

			if test.invalid && !errors.Is(err, errExample) {
				t.Fatalf("expected typed verification error, got %v", err)
			}
		})
	}
}

func TestSchemaPropertiesNamedLikeAnnotations(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"default", "enum", "const", "example", "examples"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			document := "openapi: 3.0.3\ncomponents:\n  schemas:\n    Envelope:\n      type: object\n" +
				"      properties:\n        " + name + ": {type: integer, example: wrong}\n"
			root := writeSchema(t, document)

			err := checkRepository(root)
			if !errors.Is(err, errExample) {
				t.Fatalf("property schema skipped: %v", err)
			}

			document = "openapi: 3.0.3\ncomponents:\n  schemas:\n    Envelope:\n      type: object\n" +
				"      properties:\n        " + name + ": {type: integer, nullable: true, example: null}\n"

			err = checkRepository(writeSchema(t, document))
			if err != nil {
				t.Fatalf("nullable property schema was not normalized: %v", err)
			}
		})
	}
}

func TestDefaultResponseExample(t *testing.T) {
	t.Parallel()
	root := writeSchema(t, `openapi: 3.0.3
paths:
  /command:
    post:
      responses:
        default:
          content:
            application/json:
              schema: {type: integer}
              example: wrong
`)

	err := checkRepository(root)
	if !errors.Is(err, errExample) {
		t.Fatalf("default response schema skipped: %v", err)
	}
}

func TestSchemaExampleFormats(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"date-time", "uri", "uuid"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			document := "openapi: 3.0.3\ncomponents:\n  schemas:\n    Value:\n" +
				"      type: string\n      format: " + format + "\n      example: definitely invalid\n"

			err := checkRepository(writeSchema(t, document))
			if !errors.Is(err, errExample) {
				t.Fatalf("format %s was not enforced: %v", format, err)
			}
		})
	}
}

func TestLocalFileReference(t *testing.T) {
	t.Parallel()
	root := writeSchema(t, `openapi: 3.0.3
components:
  schemas:
    External: {$ref: './types.json#/definitions/Value', example: wrong}
`)
	path := filepath.Join(root, "api", "types.json")

	err := os.WriteFile(path, []byte(`{"definitions":{"Value":{"type":"integer"}}}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = checkRepository(root)
	if !errors.Is(err, errExample) {
		t.Fatalf("local reference did not enforce its owning schema: %v", err)
	}
}

func TestLoaderConfinedToRepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	loader, err := newRepositoryLoader(root)
	if err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(t.TempDir(), "outside.json")

	err = os.WriteFile(outside, []byte(`{}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = loader.Load(fileURL(outside))
	if !errors.Is(err, errExample) {
		t.Fatalf("outside reference accepted: %v", err)
	}
}

func writeSchema(t *testing.T, document string) string {
	t.Helper()
	root := t.TempDir()

	err := os.Mkdir(filepath.Join(root, "api"), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(root, "api", "openapi.yaml"), []byte(document), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return root
}
