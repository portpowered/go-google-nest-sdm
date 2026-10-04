package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
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

	files := append(generatedPaths(), "internal/protocol/routes.gen.go", "internal/protocol/media.gen.go")
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

		entry.Schema, err = schemaOwner(strings.Split(entry.Source, ":")[0], ownerName, entry.Kind)
		if err != nil {
			return nil, err
		}

		entry.ProductionUses = []string{}
		for _, use := range entry.Uses {
			if !strings.Contains(use, ".gen.go:") {
				entry.ProductionUses = append(entry.ProductionUses, use)
			}
		}
	}

	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Package+entries[left].Declaration < entries[right].Package+entries[right].Declaration
	})

	return entries, nil
}

func inventoryDeclarations(path string, spec ast.Spec, fileSet *token.FileSet) []inventoryEntry {
	base := inventoryEntry{
		Package:     "github.com/portpowered/go-google-nest-sdm/" + filepath.ToSlash(filepath.Dir(path)),
		Declaration: "", Kind: "",
		Source: fmt.Sprintf("%s:%d", path, fileSet.Position(spec.Pos()).Line),
		Schema: modelSchema(path), Generator: "go run ./tools/generate", Uses: []string{}, ProductionUses: []string{},
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

func inventoryUses(root string, entries []inventoryEntry) error {
	index := map[string]int{}
	for position, entry := range entries {
		index[entry.Package+"."+entry.Declaration] = position
	}

	err := filepath.WalkDir(filepath.Join(root, "pkg"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("scan inventory uses: %w", walkErr)
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		return inspectUses(root, path, entries, index)
	})
	if err != nil {
		return fmt.Errorf("inventory use tree: %w", err)
	}

	return nil
}

func inspectUses(root, path string, entries []inventoryEntry, index map[string]int) error {
	fileSet := token.NewFileSet()

	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return fmt.Errorf("parse inventory use: %w", err)
	}

	imports := map[string]string{}

	for _, imported := range file.Imports {
		value, decodeErr := strconv.Unquote(imported.Path.Value)
		if decodeErr != nil {
			return fmt.Errorf("decode inventory import: %w", decodeErr)
		}

		name := filepath.Base(value)
		if imported.Name != nil {
			name = imported.Name.Name
		}

		imports[name] = value
	}

	relative, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve inventory source: %w", err)
	}

	localPackage := "github.com/portpowered/go-google-nest-sdm/" + filepath.ToSlash(filepath.Dir(relative))

	ast.Inspect(file, func(node ast.Node) bool {
		key := ""

		switch value := node.(type) {
		case *ast.SelectorExpr:
			if owner, ok := value.X.(*ast.Ident); ok && owner.Obj == nil {
				key = imports[owner.Name] + "." + value.Sel.Name
			}
		case *ast.Ident:
			if value.Obj != nil {
				key = localPackage + "." + value.Name
			}
		}

		if position, exists := index[key]; exists {
			use := fmt.Sprintf("%s:%d", filepath.ToSlash(relative), fileSet.Position(node.Pos()).Line)
			entries[position].Uses = append(entries[position].Uses, use)
		}

		return true
	})

	return nil
}
