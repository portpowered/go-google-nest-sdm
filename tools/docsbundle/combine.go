package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func combine(root, output string, documents []source) (object, error) {
	paths := make(map[string]any)
	components := make(map[string]any)
	tags := make([]any, 0)
	operations := make(map[string]bool)
	bundle := object{
		"openapi": "3.0.3",
		"info": object{
			"title": "Google Nest SDM API reference", "version": "1.0.0",
			"description": "REST resources, device commands, OAuth authorization, Pub/Sub event delivery, " +
				"and camera media downloads for Google Nest Smart Device Management.",
		},
		"paths": paths, "components": components,
	}

	for _, input := range documents {
		document, err := readDocument(filepath.Join(root, input.filename))
		if err != nil {
			return nil, err
		}

		if document["openapi"] != bundle["openapi"] {
			return nil, bundleError{operation: "unsupported documentation schema version in " + input.filename, cause: nil}
		}

		rewritten, err := input.references(root, output, document)
		if err != nil {
			return nil, err
		}

		var valid bool

		document, valid = rewritten.(map[string]any)
		if !valid {
			return nil, bundleError{operation: "invalid rewritten documentation source", cause: nil}
		}

		err = mergeComponents(components, document, input.prefix)
		if err != nil {
			return nil, err
		}

		err = mergePaths(paths, operations, document, input.prefix)
		if err != nil {
			return nil, err
		}

		if declaredTags, exists := document["tags"].([]any); exists {
			tags = append(tags, declaredTags...)
		}
	}

	bundle["tags"] = tags

	return bundle, nil
}

func readDocument(filename string) (map[string]any, error) {
	// #nosec G304 -- filenames come from the repository-owned source inventory.
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, bundleError{operation: "read documentation source", cause: err}
	}

	var document map[string]any

	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()

	err = decoder.Decode(&document)
	if err != nil {
		return nil, bundleError{operation: "decode documentation source", cause: err}
	}

	return document, nil
}

func mergeComponents(destination map[string]any, document map[string]any, prefix string) error {
	components, _ := document["components"].(map[string]any)
	for category, value := range components {
		members, ok := value.(map[string]any)
		if !ok {
			return bundleError{operation: "invalid components category " + category, cause: nil}
		}

		merged, exists := destination[category].(map[string]any)
		if !exists {
			merged = make(map[string]any)
			destination[category] = merged
		}

		for name, component := range members {
			name = prefix + "__" + name
			if _, duplicate := merged[name]; duplicate {
				return bundleError{operation: "duplicate documentation component " + name, cause: nil}
			}

			merged[name] = component
		}
	}

	return nil
}

func mergePaths(destination map[string]any, operations map[string]bool, document map[string]any, prefix string) error {
	paths, _ := document["paths"].(map[string]any)
	for name, value := range paths {
		if _, duplicate := destination[name]; duplicate {
			return bundleError{operation: "duplicate documentation route " + name, cause: nil}
		}

		path, ok := value.(map[string]any)
		if !ok {
			return bundleError{operation: "invalid documentation route " + name, cause: nil}
		}

		if _, declared := path["servers"]; !declared {
			if servers, exists := document["servers"]; exists {
				path["servers"] = servers
			}
		}

		for method, value := range path {
			if !httpMethod(method) {
				continue
			}

			operation, ok := value.(map[string]any)
			if !ok {
				return bundleError{operation: "invalid documentation operation " + name, cause: nil}
			}

			id, _ := operation["operationId"].(string)
			if id == "" || operations[id] {
				return bundleError{operation: "missing or duplicate documentation operation ID " + id, cause: nil}
			}

			operations[id] = true

			security, declared := operation["security"]
			if !declared {
				security = document["security"]
			}

			if security != nil {
				operation["security"] = securityNames(security, prefix)
			}
		}

		destination[name] = path
	}

	return nil
}

func httpMethod(method string) bool {
	switch method {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return true
	default:
		return false
	}
}

func securityNames(security any, prefix string) any {
	requirements, ok := security.([]any)
	if !ok {
		return security
	}

	result := make([]any, 0, len(requirements))

	for _, value := range requirements {
		requirement, ok := value.(map[string]any)
		if !ok {
			return security
		}

		renamed := make(map[string]any, len(requirement))
		for name, scopes := range requirement {
			renamed[prefix+"__"+name] = scopes
		}

		result = append(result, renamed)
	}

	return result
}
