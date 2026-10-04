package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// runtimeSchemas projects canonical OpenAPI components to Draft 7. Keeping the
// component pointer layout preserves all local references without duplication.
func runtimeSchemas() error {
	paths, err := filepath.Glob("api/*.openapi.yaml")
	if err != nil {
		return fmt.Errorf("find runtime schemas: %w", err)
	}
	paths = append(paths, "api/openapi.yaml")
	if err = os.MkdirAll("api/contracts", 0o750); err != nil {
		return fmt.Errorf("create runtime schema directory: %w", err)
	}
	for _, path := range paths {
		if err = runtimeSchema(path); err != nil {
			return err
		}
	}
	return nil
}

func runtimeSchema(path string) error {
	source, err := document(path)
	if err != nil {
		return err
	}
	projected := map[string]any{
		"$schema":    "http://json-schema.org/draft-07/schema#",
		"$comment":   "Generated from " + path + "; DO NOT EDIT.",
		"components": projectSchema(source["components"]),
	}
	data, err := json.MarshalIndent(projected, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime schema: %w", err)
	}
	output := filepath.Join("api/contracts", filepath.Base(path))
	if err = os.WriteFile(output, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write runtime schema: %w", err)
	}
	return nil
}

func projectSchema(value any) any {
	switch value := value.(type) {
	case []any:
		result := make([]any, len(value))
		for index, child := range value {
			result[index] = projectSchema(child)
		}
		return result
	case map[string]any:
		return projectObject(value)
	default:
		return value
	}
}

func projectObject(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		if key != "nullable" && key != "exclusiveMinimum" && key != "exclusiveMaximum" {
			result[key] = projectSchema(value)
		}
	}
	for _, bound := range []string{"Minimum", "Maximum"} {
		key := "exclusive" + bound
		if exclusive, ok := source[key].(bool); ok {
			if exclusive {
				lower := "minimum"
				if bound == "Maximum" {
					lower = "maximum"
				}
				result[key] = source[lower]
				delete(result, lower)
			}
		} else if number, exists := source[key]; exists {
			result[key] = number
		}
	}
	if nullable, ok := source["nullable"].(bool); ok && nullable {
		return map[string]any{"anyOf": []any{result, map[string]any{"type": "null"}}}
	}
	return result
}
