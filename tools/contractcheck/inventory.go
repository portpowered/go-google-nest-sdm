package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

type inventoryEntry struct {
	Package        string           `json:"package"`
	Declaration    string           `json:"declaration"`
	Kind           string           `json:"kind"`
	Source         string           `json:"source"`
	Schema         string           `json:"schema"`
	Generator      string           `json:"generator"`
	Uses           []string         `json:"uses"`
	ProductionUses []string         `json:"productionUses"`
	WireRoots      []string         `json:"wireRoots"`
	Disposition    string           `json:"disposition"`
	TypeExpression string           `json:"typeExpression"`
	Value          string           `json:"value"`
	Fields         []inventoryField `json:"fields"`
}

type inventoryField struct {
	Name           string `json:"name"`
	TypeExpression string `json:"typeExpression"`
	Required       bool   `json:"required"`
}

const inventoryPermissions = 0o644

func writeInventory(root, output string) error {
	entries, err := modelInventory(root)
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode inventory: %w", err)
	}
	// #nosec G306 -- the output is public schema inventory, with no credentials.
	err = os.WriteFile(output, append(encoded, '\n'), inventoryPermissions)
	if err != nil {
		return fmt.Errorf("write inventory: %w", err)
	}

	return nil
}

func modelInventory(root string) ([]inventoryEntry, error) {
	entries := []inventoryEntry{}

	files := append(generatedPaths(),
		"internal/protocol/routes.gen.go", "internal/protocol/media.gen.go", "internal/protocol/channels.gen.go")
	for _, path := range files {
		fileSet := token.NewFileSet()

		file, err := parser.ParseFile(fileSet, filepath.Join(root, path), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse inventory declaration: %w", err)
		}

		for _, declaration := range file.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range group.Specs {
				entries = append(entries, inventoryDeclarations(path, spec, fileSet)...)
			}
		}
	}

	err := inventoryUses(root, entries)
	if err != nil {
		return nil, err
	}

	for position := range entries {
		entry := &entries[position]

		ownerName := entry.Declaration

		if entry.Kind == "constant" && entry.Schema != "" && !strings.Contains(entry.Schema, "ProtocolValues/") {
			parts := strings.Split(entry.Schema, "/")
			ownerName = parts[len(parts)-1]
		}

		entry.Schema, err = schemaOwner(root, strings.Split(entry.Source, ":")[0], ownerName, entry.Kind)
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Package+entries[left].Declaration < entries[right].Package+entries[right].Declaration
	})

	return entries, nil
}

func checkInventory(root string) error {
	entries, err := modelInventory(root)
	if err != nil {
		return err
	}

	expected, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode expected inventory: %w", err)
	}
	// #nosec G304 -- the inventory is a fixed repository-owned contributor artifact.
	actual, err := os.ReadFile(filepath.Join(root, "docs/model-inventory.json"))
	if err != nil {
		return fmt.Errorf("read checked-in inventory: %w", err)
	}

	return compareInventory(append(expected, '\n'), actual)
}

func compareInventory(expected, actual []byte) error {
	if !bytes.Equal(expected, actual) {
		return fmt.Errorf("%w: model inventory drift; regenerate with -write-inventory docs/model-inventory.json",
			errContract)
	}

	return nil
}

func inventoryDeclarations(path string, spec ast.Spec, fileSet *token.FileSet) []inventoryEntry {
	base := inventoryEntry{
		Package:     "github.com/portpowered/go-google-nest-sdm/" + filepath.ToSlash(filepath.Dir(path)),
		Declaration: "", Kind: "",
		Source: fmt.Sprintf("%s:%d", path, fileSet.Position(spec.Pos()).Line),
		Schema: modelSchema(path), Generator: "go run ./tools/generate", Uses: []string{}, ProductionUses: []string{},
		WireRoots: []string{}, Disposition: "",
		TypeExpression: "", Value: "", Fields: []inventoryField{},
	}
	switch value := spec.(type) {
	case *ast.TypeSpec:
		base.Declaration = value.Name.Name
		base.Kind = "type"
		base.TypeExpression = typeExpression(value.Type)
		base.Fields = inventoryFields(value.Type)
		base.Schema += "#/components/schemas/" + value.Name.Name

		return []inventoryEntry{base}
	case *ast.ValueSpec:
		result := []inventoryEntry{}

		for position, name := range value.Names {
			item := base
			item.Declaration = name.Name

			item.Kind = "constant"
			if position < len(value.Values) {
				item.Value = typeExpression(value.Values[position])
			}

			if value.Type != nil {
				item.TypeExpression = typeExpression(value.Type)
			}

			if kind, ok := value.Type.(*ast.Ident); ok {
				item.Schema += "#/components/schemas/" + kind.Name
			} else {
				item.Schema = "api/protocol.openapi.yaml#/components/schemas/ProtocolValues/properties/" + name.Name
			}

			result = append(result, item)
		}

		return result
	default:
		return nil
	}
}

func typeExpression(expression ast.Expr) string {
	var encoded bytes.Buffer

	err := format.Node(&encoded, token.NewFileSet(), expression)
	if err != nil {
		return "invalid parsed expression"
	}

	return encoded.String()
}

func inventoryFields(expression ast.Expr) []inventoryField {
	fields := []inventoryField{}

	structure, ok := expression.(*ast.StructType)

	if !ok {
		return fields
	}

	for _, field := range structure.Fields.List {
		if field.Tag == nil {
			continue
		}

		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		name, options, _ := strings.Cut(tag.Get("json"), ",")
		fields = append(fields, inventoryField{
			Name: name, TypeExpression: typeExpression(field.Type),
			Required: !strings.Contains(options, "omitempty") && name != "-",
		})
	}

	return fields
}
