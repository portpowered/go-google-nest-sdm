package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type repositoryLoader struct {
	root string
}

func newRepositoryLoader(root string) (*repositoryLoader, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository: %w", err)
	}

	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve repository links: %w", err)
	}

	return &repositoryLoader{root: absolute}, nil
}

func fileURL(path string) string {
	value := filepath.ToSlash(path)
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}

	var location url.URL

	location.Scheme = "file"
	location.Path = value

	return location.String()
}

func (loader *repositoryLoader) Load(location string) (any, error) {
	parsed, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("parse schema URL: %w", err)
	}

	if parsed.Scheme != "file" || parsed.Host != "" || parsed.RawQuery != "" {
		return nil, fmt.Errorf("%w: only repository file references are allowed: %s", errExample, location)
	}

	path := filepath.FromSlash(parsed.Path)
	if filepath.VolumeName(loader.root) != "" {
		path = strings.TrimPrefix(path, string(filepath.Separator))
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("resolve schema file: %w", err)
	}

	relative, err := filepath.Rel(loader.root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("%w: schema reference escapes repository: %s", errExample, location)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}

	var document any

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return nil, fmt.Errorf("decode schema YAML: %w", err)
	}

	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("normalize schema JSON: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()

	err = decoder.Decode(&document)
	if err != nil {
		return nil, fmt.Errorf("decode schema JSON: %w", err)
	}

	normalizeSchema(document, standaloneSchema(document))

	return document, nil
}

func normalizeSchema(value any, schemaNode bool) {
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			normalizeSchema(child, schemaNode)
		}
	case map[string]any:
		normalizeChildren(node, schemaNode)

		if schemaNode {
			normalizeBounds(node)
			normalizeNullable(node)
		}
	}
}

func normalizeChildren(node map[string]any, schemaNode bool) {
	for key, child := range node {
		if key == exampleKey || key == examplesKey || (schemaNode && schemaAnnotation(key)) {
			continue
		}

		if (schemaNode && schemaMapKeyword(key)) || (!schemaNode && key == "schemas") {
			if schemas, ok := child.(map[string]any); ok {
				for _, schema := range schemas {
					normalizeSchema(schema, true)
				}
			}

			continue
		}

		normalizeSchema(child, schemaNode || key == "schema" || key == "payload")
	}
}

func normalizeBounds(node map[string]any) {
	for _, bound := range []string{"Minimum", "Maximum"} {
		key := "exclusive" + bound
		if exclusive, ok := node[key].(bool); ok {
			delete(node, key)

			if exclusive {
				node[key] = node[strings.ToLower(bound)]
				delete(node, strings.ToLower(bound))
			}
		}
	}
}

func normalizeNullable(node map[string]any) {
	if nullable, ok := node["nullable"].(bool); ok && nullable {
		delete(node, "nullable")

		if kind, typed := node["type"].(string); typed {
			node["type"] = []any{kind, "null"}

			return
		}

		if reference, referenced := node["$ref"].(string); referenced {
			delete(node, "$ref")
			node["anyOf"] = []any{map[string]any{"$ref": reference}, map[string]any{"type": "null"}}
		}
	}
}
