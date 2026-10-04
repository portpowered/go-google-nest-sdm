package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

type objectContract struct {
	Properties map[string]yaml.Node `yaml:"properties"`
	Required   []string             `yaml:"required"`
}

type fieldContract struct {
	Type string `yaml:"type"`
}

const objectKind = "object"

const sdmSchemaPath = "api/openapi.yaml"

const oauthSchemaPath = "api/external/oauth.openapi.yaml"

func verifyGeneratedFields(root string) error {
	for _, path := range generatedPaths() {
		schemaPath := modelSchema(path)
		if schemaPath == "" {
			continue
		}
		// #nosec G304 -- the schema and source paths are selected by the fixed generation manifest.
		data, err := os.ReadFile(filepath.Join(root, schemaPath))
		if err != nil {
			return fmt.Errorf("read owned schema: %w", err)
		}

		var document contractDocument

		err = yaml.Unmarshal(data, &document)
		if err != nil {
			return fmt.Errorf("decode owned schema: %w", err)
		}

		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, path), nil, 0)
		if err != nil {
			return fmt.Errorf("parse generated model: %w", err)
		}

		err = verifyFileFields(path, file, document)
		if err != nil {
			return err
		}
	}

	return nil
}

func verifyFileFields(path string, file *ast.File, document contractDocument) error {
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE {
			continue
		}

		for _, spec := range group.Specs {
			model, isModel := spec.(*ast.TypeSpec)
			if !isModel {
				continue
			}

			structure, ok := model.Type.(*ast.StructType)
			if !ok {
				continue
			}

			schema, exists := document.Components.Schemas[model.Name.Name]
			if !exists {
				continue
			} // Generator operation parameter structs are separately schema-owned.

			var expected objectContract

			err := schema.Decode(&expected)
			if err != nil {
				return fmt.Errorf("decode model fields: %w", err)
			}

			err = matchFields(structure, expected)
			if err != nil {
				return fmt.Errorf("%s %s: %w", path, model.Name.Name, err)
			}
		}
	}

	return nil
}

func matchFields(structure *ast.StructType, expected objectContract) error {
	required := map[string]bool{}
	for _, name := range expected.Required {
		required[name] = true
	}

	seen := map[string]bool{}

	for _, field := range structure.Fields.List {
		if field.Tag == nil {
			return fmt.Errorf("%w: generated wire field has no JSON tag", errContract)
		}

		tag := strings.Trim(field.Tag.Value, "`")

		name, optional, _ := strings.Cut(reflect.StructTag(tag).Get("json"), ",")
		if name == "-" {
			continue
		}

		if _, present := expected.Properties[name]; !present {
			return fmt.Errorf("%w: unexpected JSON field %s", errContract, name)
		}

		var shape fieldContract

		property := expected.Properties[name]

		err := property.Decode(&shape)
		if err != nil {
			return fmt.Errorf("decode field shape: %w", err)
		}

		if !fieldTypeMatches(field.Type, shape.Type) {
			return fmt.Errorf("%w: field %s type differs", errContract, name)
		}

		if seen[name] {
			return fmt.Errorf("%w: duplicate JSON field %s", errContract, name)
		}

		seen[name] = true

		if required[name] == strings.Contains(optional, "omitempty") {
			return fmt.Errorf("%w: field %s required/presence differs", errContract, name)
		}
	}

	for name := range expected.Properties {
		if !seen[name] {
			return fmt.Errorf("%w: JSON field %s missing", errContract, name)
		}
	}

	return nil
}

func fieldTypeMatches(expression ast.Expr, kind string) bool {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		return fieldTypeMatches(pointer.X, kind)
	}

	if kind == "" {
		return true
	}

	switch value := expression.(type) {
	case *ast.Ident:
		switch kind {
		case "number":
			return value.Name == "float64" || value.Name == "float32"
		case "integer":
			return value.Name == "int" || value.Name == "int64" || value.Name == "int32"
		case "boolean":
			return value.Name == "bool"
		case "array":
			return false
		default:
			return value.Name != "int" && value.Name != "float64" && value.Name != "bool"
		}
	case *ast.ArrayType:
		return kind == "array" || kind == "string" // Base64 transport bytes are a string on the wire.
	case *ast.MapType:
		return kind == objectKind
	case *ast.SelectorExpr:
		return kind == "string" || kind == objectKind
	case *ast.StructType:
		return kind == objectKind
	default:
		return false
	}
}

func modelSchema(path string) string {
	switch filepath.Base(path) {
	case "commands.gen.go":
		return "api/commands.openapi.yaml"
	case "devices.gen.go":
		return sdmSchemaPath
	case "events.gen.go":
		return "api/events.openapi.yaml"
	case "oauth.gen.go":
		return oauthSchemaPath
	case "oauth-values.gen.go":
		return oauthSchemaPath
	case "errors.gen.go", "error-values.gen.go":
		return "api/errors.openapi.yaml"
	case "provider-errors.gen.go":
		return "api/client-errors.openapi.yaml"
	case "pubsub.gen.go":
		return "api/external/pubsub.openapi.yaml"
	case "resources.gen.go":
		return "api/client-resources.openapi.yaml"
	case "media.gen.go":
		return "api/client-media.openapi.yaml"
	case "traits.gen.go":
		return "api/traits.openapi.yaml"
	case "trait-values.gen.go":
		return "api/traits.openapi.yaml"
	case "device-values.gen.go":
		return sdmSchemaPath
	default:
		return ""
	}
}
