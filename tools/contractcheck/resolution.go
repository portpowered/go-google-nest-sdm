package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const inventoryCompileTimeout = 2 * time.Minute

type sourcePackage struct {
	Directory  string   `json:"dir"`
	ImportPath string   `json:"importPath"`
	Export     string   `json:"export"`
	GoFiles    []string `json:"goFiles"`
}

type inventoryResolution struct {
	FileSet    *token.FileSet
	Sources    []*ast.File
	Info       *types.Info
	ImportPath string
}

func inventoryUses(root string, entries []inventoryEntry) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve inventory workspace: %w", err)
	}

	listed, err := listInventoryPackages(root)
	if err != nil {
		return err
	}

	fileSet := token.NewFileSet()
	compiler := inventoryImporter(fileSet, listed)
	index := inventoryIndex(entries)
	edges := map[int]map[int]bool{}

	for _, source := range listed {
		if !localInventoryPackage(source.ImportPath) {
			continue
		}

		resolved, resolveErr := resolveInventoryPackage(source, fileSet, compiler)
		if resolveErr != nil {
			return resolveErr
		}

		err = recordResolvedUses(root, resolved, entries, index, edges)
		if err != nil {
			return err
		}
	}

	traceWireRoots(entries, edges)

	return nil
}

func listInventoryPackages(root string) ([]sourcePackage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), inventoryCompileTimeout)
	defer cancel()

	command := exec.CommandContext(ctx,
		"go", "list", "-export", "-deps", "-json", "./pkg/...", "./internal/...", "./api/...")
	command.Dir = root

	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("compile inventory package exports: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(output))
	result := []sourcePackage{}

	for decoder.More() {
		var entry sourcePackage

		err = decoder.Decode(&entry)
		if err != nil {
			return nil, fmt.Errorf("decode inventory package: %w", err)
		}

		result = append(result, entry)
	}

	return result, nil
}

func inventoryImporter(fileSet *token.FileSet, listed []sourcePackage) types.Importer {
	exports := map[string]string{}
	for _, entry := range listed {
		exports[entry.ImportPath] = entry.Export
	}

	return importer.ForCompiler(fileSet, "gc", func(path string) (io.ReadCloser, error) {
		location, exists := exports[path]
		if !exists || location == "" {
			return nil, fmt.Errorf("%w: export outside package inventory %s", errContract, path)
		}
		// #nosec G304 -- the Go compiler supplied the exact export archive path for this dependency.
		archive, err := os.Open(location)
		if err != nil {
			return nil, fmt.Errorf("read compiler inventory export: %w", err)
		}

		return archive, nil
	})
}

func resolveInventoryPackage(source sourcePackage, fileSet *token.FileSet,
	compiler types.Importer) (inventoryResolution, error) {
	files := []*ast.File{}

	for _, name := range source.GoFiles {
		file, err := parser.ParseFile(fileSet, filepath.Join(source.Directory, name), nil, 0)
		if err != nil {
			return inventoryResolution{}, fmt.Errorf("parse inventory package: %w", err)
		}

		files = append(files, file)
	}

	info := &types.Info{
		Types: nil, Instances: nil, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{},
		Implicits: nil, Selections: nil, Scopes: nil, InitOrder: nil, FileVersions: nil,
	}
	config := types.Config{
		Context: nil, GoVersion: "", IgnoreFuncBodies: false, FakeImportC: false,
		DisableUnusedImportCheck: false, Error: nil, Importer: compiler, Sizes: nil,
	}

	_, err := config.Check(source.ImportPath, fileSet, files, info)
	if err != nil {
		return inventoryResolution{}, fmt.Errorf("resolve inventory package %s: %w", source.ImportPath, err)
	}

	return inventoryResolution{FileSet: fileSet, Sources: files, Info: info, ImportPath: source.ImportPath}, nil
}
