package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const runtimeDirectoryMode = 0o750

// runtimeSchemas projects canonical OpenAPI components to Draft 7. Keeping the
// component pointer layout preserves all local references without duplication.
func runtimeSchemas() error {
	paths, err := filepath.Glob("api/*.openapi.yaml")
	if err != nil {
		return fmt.Errorf("find runtime schemas: %w", err)
	}

	paths = append(paths, "api/openapi.yaml")

	externalPaths, err := filepath.Glob("api/external/*.openapi.yaml")
	if err != nil {
		return fmt.Errorf("find dependency runtime schemas: %w", err)
	}

	paths = append(paths, externalPaths...)

	err = os.MkdirAll("api/contracts", runtimeDirectoryMode)
	if err != nil {
		return fmt.Errorf("create runtime schema directory: %w", err)
	}

	for _, path := range paths {
		err = runtimeSchema(path)
		if err != nil {
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
		"$comment":   runtimeSourceComment(path),
		"components": projectSchema(source["components"]),
	}

	data, err := json.MarshalIndent(projected, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime schema: %w", err)
	}

	output := filepath.Join("api/contracts", filepath.Base(path))

	err = os.WriteFile(output, append(data, '\n'), generatedFileMode)
	if err != nil {
		return fmt.Errorf("write runtime schema: %w", err)
	}

	return nil
}

func runtimeSourceComment(path string) string {
	canonicalPath := strings.ReplaceAll(filepath.ToSlash(path), "\\", "/")

	return "Generated from " + canonicalPath + "; DO NOT EDIT."
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
			if reference, ok := value.(string); key == "$ref" && ok {
				value = strings.TrimPrefix(reference, "../")
			}

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
