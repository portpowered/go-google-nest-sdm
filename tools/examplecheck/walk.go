package main

import (
	"fmt"
	"strconv"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func walkExamples(compiler *jsonschema.Compiler, location, path string, value any, schemaNode bool) error {
	switch node := value.(type) {
	case []any:
		for index, child := range node {
			err := walkExamples(compiler, location, path+"/"+strconv.Itoa(index), child, schemaNode)
			if err != nil {
				return err
			}
		}
	case map[string]any:
		if path == "/components/schemas" {
			return walkSchemaMap(compiler, location, path, node)
		}

		err := checkNodeExamples(compiler, location, path, node, schemaNode)
		if err != nil {
			return err
		}

		return walkChildren(compiler, location, path, node, schemaNode)
	}

	return nil
}

func walkChildren(
	compiler *jsonschema.Compiler, location, path string, node map[string]any, schemaNode bool,
) error {
	for key, child := range node {
		if key == exampleKey || key == examplesKey {
			continue
		}

		if schemaNode && schemaAnnotation(key) {
			continue
		}

		childPath := path + "/" + pointerPart(key)

		if schemaNode && schemaMapKeyword(key) {
			if schemas, ok := child.(map[string]any); ok {
				err := walkSchemaMap(compiler, location, childPath, schemas)
				if err != nil {
					return err
				}
			}

			continue
		}

		isSchema := key == "schema" || key == "payload" || schemaNode

		err := walkExamples(compiler, location, childPath, child, isSchema)
		if err != nil {
			return err
		}
	}

	return nil
}

func schemaAnnotation(key string) bool {
	switch key {
	case "default", "enum", "const":
		return true
	default:
		return false
	}
}

func schemaMapKeyword(key string) bool {
	switch key {
	case "properties", "patternProperties", "definitions", "$defs", "dependentSchemas":
		return true
	default:
		return false
	}
}

func walkSchemaMap(compiler *jsonschema.Compiler, location, path string, schemas map[string]any) error {
	for name, schema := range schemas {
		err := walkExamples(compiler, location, path+"/"+pointerPart(name), schema, true)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkNodeExamples(
	compiler *jsonschema.Compiler, location, path string, node map[string]any, schemaNode bool,
) error {
	owner := path
	if _, ok := node["schema"]; ok && !schemaNode {
		owner += "/schema"
	} else if _, ok := node["payload"]; ok && !schemaNode {
		return checkMessageExamples(compiler, location, path, node)
	} else if !schemaNode {
		return nil
	}

	if value, ok := node[exampleKey]; ok {
		err := validateExample(compiler, location, owner, exampleKey, value)
		if err != nil {
			return err
		}
	}

	switch examples := node[examplesKey].(type) {
	case []any:
		for index, value := range examples {
			err := validateExample(compiler, location, owner, strconv.Itoa(index), value)
			if err != nil {
				return err
			}
		}
	case map[string]any:
		for name, value := range examples {
			example, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("%w: invalid ExampleObject %s", errExample, name)
			}

			if _, exists := example["externalValue"]; exists {
				return fmt.Errorf("%w: externalValue is not offline: %s", errExample, name)
			}

			if _, exists := example["$ref"]; exists {
				return fmt.Errorf("%w: ExampleObject references must be resolved before validation: %s", errExample, name)
			}

			err := validateExample(compiler, location, owner, name, example["value"])
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func checkMessageExamples(compiler *jsonschema.Compiler, location, path string, node map[string]any) error {
	examples, _ := node[examplesKey].([]any)
	for index, value := range examples {
		example, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: invalid message example at %s", errExample, path)
		}

		err := validateExample(compiler, location, path+"/payload", strconv.Itoa(index), example["payload"])
		if err != nil {
			return err
		}
	}

	return nil
}
