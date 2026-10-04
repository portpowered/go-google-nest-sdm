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
	"strings"
)

const catalogSkipped = "skip:"

type wireCatalogEntry struct {
	Package        string `json:"package"`
	Declaration    string `json:"declaration"`
	Kind           string `json:"kind"`
	Source         string `json:"source"`
	Schema         string `json:"schema"`
	TypeExpression string `json:"typeExpression"`
	Value          string `json:"value"`
}

func loadWireModels(root string) (map[string]ast.Expr, error) {
	// #nosec G304 -- root selects the checked public repository inventory.
	encoded, err := os.ReadFile(filepath.Join(root, "docs/model-inventory.json"))
	if err != nil {
		return nil, fmt.Errorf("read wire declaration inventory: %w", err)
	}

	var entries []wireCatalogEntry

	err = json.Unmarshal(encoded, &entries)
	if err != nil {
		return nil, fmt.Errorf("decode wire declaration inventory: %w", err)
	}

	models := map[string]ast.Expr{}
	files := map[string]*ast.File{}

	for _, entry := range entries {
		prefix := catalogPrefix(entry.Package)
		if prefix == catalogSkipped {
			continue
		}

		path, _, _ := strings.Cut(entry.Source, ":")
		if !filepath.IsLocal(path) || !strings.HasSuffix(path, ".gen.go") {
			return nil, fmt.Errorf("%w: unregistered generated source", errRouteInvalid)
		}

		file := files[path]
		if file == nil {
			file, err = parser.ParseFile(token.NewFileSet(), filepath.Join(root, path), nil, 0)
			if err != nil {
				return nil, fmt.Errorf("parse catalog source: %w", err)
			}

			files[path] = file
		}

		err = registerCatalogEntry(models, file, entry, prefix)
		if err != nil {
			return nil, err
		}
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("%w: generated wire catalog absent", errRouteInvalid)
	}

	return models, nil
}

func catalogPrefix(path string) string {
	switch path {
	case wireModelImport:
		return ""
	case module + "/pkg/sdm":
		return "public:"
	case module + "/internal/protocol":
		return "protocol:"
	default:
		return catalogSkipped
	}
}

func registerCatalogEntry(models map[string]ast.Expr, file *ast.File, entry wireCatalogEntry, prefix string) error {
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range group.Specs {
			if model, recognized := spec.(*ast.TypeSpec); recognized && model.Name.Name == entry.Declaration {
				models[prefix+entry.Declaration] = model.Type
				models[prefix+"domain:"+entry.Declaration] = &ast.BasicLit{
					ValuePos: token.NoPos, Kind: token.STRING, Value: entry.Schema,
				}

				return nil
			}

			constant, recognized := spec.(*ast.ValueSpec)
			if !recognized || group.Tok != token.CONST {
				continue
			}

			for position, name := range constant.Names {
				if name.Name != entry.Declaration {
					continue
				}

				if position >= len(constant.Values) || expressionText(constant.Type) != entry.TypeExpression ||
					expressionText(constant.Values[position]) != entry.Value {
					return fmt.Errorf("%w: generated constant inventory drift %s", errRouteInvalid, entry.Declaration)
				}

				models[prefix+"constant:"+entry.Declaration] = constant.Type

				return nil
			}
		}
	}

	return fmt.Errorf("%w: missing registered declaration %s", errRouteInvalid, entry.Declaration)
}

func expressionText(expression ast.Expr) string {
	if expression == nil {
		return ""
	}

	var output bytes.Buffer

	err := format.Node(&output, token.NewFileSet(), expression)
	if err != nil {
		return ""
	}

	return output.String()
}
