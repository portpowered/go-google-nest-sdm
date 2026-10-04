package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// projection expands only components selected by the public projection schema.
// Canonical provider components remain shared inputs, while public Go types are
// generated independently from the checked-in semantic projection inventory.
func projection(catalog, output string) error {
	doc, err := document("api/client-models.openapi.yaml")
	if err != nil {
		return err
	}
	selected := map[string]any{}
	for name, raw := range schemas(doc) {
		reference, _ := object(raw)["$ref"].(string)
		parts := strings.SplitN(reference, "#/components/schemas/", 2)
		if len(parts) != 2 || parts[0] != "./"+catalog {
			continue
		}
		owner, readErr := document(filepath.Join("api", catalog))
		if readErr != nil {
			return readErr
		}
		selected[name] = schemas(owner)[parts[1]]
	}
	object(doc["components"])["schemas"] = selected
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode projection: %w", err)
	}
	input, err := os.CreateTemp("api", ".projection-*.json")
	if err != nil {
		return fmt.Errorf("create projection input: %w", err)
	}
	defer os.Remove(input.Name())
	if _, err = input.Write(data); err != nil {
		input.Close()
		return fmt.Errorf("write projection input: %w", err)
	}
	if err = input.Close(); err != nil {
		return fmt.Errorf("close projection input: %w", err)
	}
	return command("go", "run", oapiVersion, "--config", "tools/generate/commands-public.yaml", "-o", output, input.Name())
}
