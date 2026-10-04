package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"testing"
)

type inventoryTestImporter struct{ foreign *types.Package }

func (value inventoryTestImporter) Import(_ string) (*types.Package, error) {
	return value.foreign, nil
}

func TestResolvedInventoryUses(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileSet := token.NewFileSet()
	sources := []string{
		`package p
type Parent struct { Child *Nested; Deep *ChildOnly }
type Nested struct{}
type Orphan struct{}
type ChildOnly struct{}`,
		`package p
import other "foreign"
func consume(p Parent) { _ = p; var n Nested; _ = n; var f other.Nested; _ = f }
func shadow() { type Nested int; var n Nested; _ = n }`,
	}

	files := make([]*ast.File, len(sources))

	for position, source := range sources {
		name := "models.gen.go"
		if position > 0 {
			name = "consume.go"
		}

		file, err := parser.ParseFile(fileSet, filepath.Join(root, name), source, 0)
		if err != nil {
			t.Fatal(err)
		}

		files[position] = file
	}

	foreign := types.NewPackage("foreign", "foreign")
	foreignName := types.NewTypeName(token.NoPos, foreign, "Nested", nil)
	_ = types.NewNamed(foreignName, types.NewStruct(nil, nil), nil)
	foreign.Scope().Insert(foreignName)
	foreign.MarkComplete()

	info := &types.Info{
		Types: nil, Instances: nil, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{},
		Implicits: nil, Selections: nil, Scopes: nil, InitOrder: nil, FileVersions: nil,
	}
	config := types.Config{
		Context: nil, GoVersion: "", IgnoreFuncBodies: false, FakeImportC: false,
		DisableUnusedImportCheck: false, Error: nil, Importer: inventoryTestImporter{foreign: foreign}, Sizes: nil,
	}

	_, err := config.Check("local", fileSet, files, info)
	if err != nil {
		t.Fatal(err)
	}

	entries := make([]inventoryEntry, 5)
	for position, name := range []string{"Parent", "Nested", "Orphan", "Nested", "ChildOnly"} {
		entries[position].Package = "local"
		entries[position].Declaration = name
	}

	entries[3].Package = "foreign"
	edges := map[int]map[int]bool{}
	resolved := inventoryResolution{FileSet: fileSet, Sources: files, Info: info, ImportPath: "local"}

	err = recordResolvedUses(root, resolved, entries, inventoryIndex(entries), edges)
	if err != nil {
		t.Fatal(err)
	}

	traceWireRoots(entries, edges)

	if len(entries[2].Uses) != 0 {
		t.Fatal("declaration incorrectly counted as use")
	}

	if len(entries[1].ProductionUses) != 1 || len(entries[3].ProductionUses) != 1 {
		t.Fatal("cross-file, aliased foreign or shadowed names resolved incorrectly", entries)
	}

	if !edges[0][1] || len(entries[1].WireRoots) < 2 {
		t.Fatal("generated nested field did not reach handwritten production root")
	}

	if len(entries[4].ProductionUses) != 0 || len(entries[4].WireRoots) == 0 {
		t.Fatal("nested-only generated model incorrectly classified as unused")
	}
}
