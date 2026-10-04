package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// projection expands components selected by the semantic projection inventory.
func projection(catalog, output string) error {
	doc, err := document("api/client-models.openapi.yaml")
	if err != nil {
		return err
	}

	owner, err := document(filepath.Join("api", catalog))
	if err != nil {
		return err
	}

	selected := map[string]any{}

	for name, raw := range schemas(doc) {
		reference, _ := object(raw)["$ref"].(string)

		file, component, ok := strings.Cut(reference, "#/components/schemas/")
		if !ok || file != "./"+catalog {
			continue
		}

		selected[name] = schemas(owner)[component]
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

	_, writeErr := input.Write(data)
	closeErr := input.Close()

	err = errors.Join(writeErr, closeErr)
	if err == nil {
		err = command("go", "run", oapiVersion, "--config",
			"tools/generate/commands-public.yaml", "-o", output, input.Name())
	}

	removeErr := os.Remove(input.Name())

	err = errors.Join(err, removeErr)
	if err != nil {
		return fmt.Errorf("generate public projection: %w", err)
	}

	return nil
}
