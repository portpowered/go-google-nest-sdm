package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBundlePreservesOriginsAuthorizationAndComponentReferences(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	documents := []source{
		{filename: "api/rest.json", prefix: "REST"},
		{filename: "api/external/oauth.json", prefix: "OAuth"},
	}
	for index, input := range documents {
		origin := "https://" + input.prefix + ".example.invalid"

		document := testDocument("/operation"+input.prefix, "Operation"+input.prefix, origin)
		if index == 1 {
			components := mapValue(t, document["components"])
			schemas := mapValue(t, components["schemas"])
			schemas["External"] = object{"$ref": "../errors.json#/components/schemas/Error"}
		}

		writeDocument(t, root, input.filename, document)
	}

	bundle, err := combine(root, outputFile, documents)
	if err != nil {
		t.Fatal(err)
	}

	paths := mapValue(t, bundle["paths"])
	for _, input := range documents {
		path := mapValue(t, paths["/operation"+input.prefix])

		servers := arrayValue(t, path["servers"])

		server := mapValue(t, servers[0])
		if server["url"] != "https://"+input.prefix+".example.invalid" {
			t.Fatalf("lost source origin for %s", input.prefix)
		}

		operation := mapValue(t, path["get"])

		security := arrayValue(t, operation["security"])

		requirement := mapValue(t, security[0])
		if _, exists := requirement[input.prefix+"__Bearer"]; !exists {
			t.Fatalf("lost source security for %s", input.prefix)
		}

		responses := mapValue(t, operation["responses"])

		response := mapValue(t, responses["200"])
		if response["$ref"] != "#/components/responses/"+input.prefix+"__Success" {
			t.Fatalf("local reference was not renamed: %#v", response)
		}
	}

	components := mapValue(t, bundle["components"])
	schemas := mapValue(t, components["schemas"])

	external := mapValue(t, schemas["OAuth__External"])
	if external["$ref"] != "../api/errors.json#/components/schemas/Error" {
		t.Fatalf("external reference was not rebased: %#v", external)
	}

	repeated, err := combine(root, outputFile, documents)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(bundle, repeated) {
		t.Fatal("bundle depends on generation order or mutable source state")
	}
}

func TestBundleRejectsConflictingPathsAndOperationIDs(t *testing.T) {
	t.Parallel()

	for _, duplicate := range []string{"path", "operation"} {
		t.Run(duplicate, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			path, operation := "/second", "Second"
			if duplicate == "path" {
				path = "/first"
			} else {
				operation = "First"
			}

			writeDocument(t, root, "first.json", testDocument("/first", "First", "https://first.example.invalid"))
			writeDocument(t, root, "second.json", testDocument(path, operation, "https://second.example.invalid"))

			_, err := combine(root, outputFile, []source{
				{filename: "first.json", prefix: "First"}, {filename: "second.json", prefix: "Second"},
			})
			if err == nil {
				t.Fatalf("accepted conflicting %s", duplicate)
			}
		})
	}
}

func TestExplicitPublicOperationSecurityIsPreserved(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	document := testDocument("/public", "Public", "https://public.example.invalid")
	paths := mapValue(t, document["paths"])
	path := mapValue(t, paths["/public"])
	operation := mapValue(t, path["get"])
	operation["security"] = []any{}

	writeDocument(t, root, "public.json", document)

	bundle, err := combine(root, outputFile, []source{{filename: "public.json", prefix: "Public"}})
	if err != nil {
		t.Fatal(err)
	}

	mergedPaths := mapValue(t, bundle["paths"])
	mergedPath := mapValue(t, mergedPaths["/public"])

	merged := mapValue(t, mergedPath["get"])
	if len(arrayValue(t, merged["security"])) != 0 {
		t.Fatal("public operation inherited account authorization")
	}
}

func mapValue(t *testing.T, value any) map[string]any {
	t.Helper()

	switch value := value.(type) {
	case object:
		return map[string]any(value)
	case map[string]any:
		return value
	default:
		t.Fatalf("unexpected documentation object type %T", value)

		return nil
	}
}

func arrayValue(t *testing.T, value any) []any {
	t.Helper()

	result, ok := value.([]any)
	if !ok {
		t.Fatalf("unexpected documentation value type %T", value)
	}

	return result
}

func testDocument(path, operation, origin string) object {
	return object{
		"openapi": "3.0.3", "info": object{"title": "Synthetic documentation contract", "version": "1.0.0"},
		"servers": []any{object{"url": origin}}, "security": []any{object{"Bearer": []any{}}},
		"paths": object{path: object{"get": object{
			"operationId": operation, "responses": object{"200": object{"$ref": "#/components/responses/Success"}},
		}}},
		"components": object{
			"schemas":         object{"Result": object{"type": "object"}},
			"responses":       object{"Success": object{"description": "Accepted"}},
			"securitySchemes": object{"Bearer": object{"type": "http", "scheme": "bearer"}},
		},
	}
}

func writeDocument(t *testing.T, root, name string, document object) {
	t.Helper()

	filename := filepath.Join(root, name)

	err := os.MkdirAll(filepath.Dir(filename), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filename, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
