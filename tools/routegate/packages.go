package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

const generatedBindingPrefix = "schema-generated:"

type packageSource struct {
	file *ast.File
	path string
}

func bindGeneratedConstants(imports map[string]string, models map[string]ast.Expr) {
	for key := range models {
		if name, registered := strings.CutPrefix(key, "protocol:constant:"); registered {
			imports[generatedBindingPrefix+name] = name
		}
	}
}

func auditSourcePackages(root string, models map[string]ast.Expr) error {
	packages := map[string][]packageSource{}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, relativeErr := filepath.Rel(root, path)
		if relativeErr != nil {
			return fmt.Errorf("resolve package source: %w", relativeErr)
		}

		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if excludedSourceTree(relative) {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		if strings.HasSuffix(path, ".gen.go") && models["approved-source:"+relative] != nil {
			return nil
		}

		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parse package source: %w", parseErr)
		}

		packages[filepath.Dir(path)] = append(packages[filepath.Dir(path)], packageSource{file: file, path: relative})

		return nil
	})
	if err != nil {
		return fmt.Errorf("collect package sources: %w", err)
	}

	for _, sources := range packages {
		files := make([]*ast.File, len(sources))
		for position, source := range sources {
			files[position] = source.file
		}

		err = auditWirePackage(files, models)
		if err != nil {
			return fmt.Errorf("package %s: %w", sources[0].path, err)
		}

		for _, source := range sources {
			err = auditFilePolicy(source.file, source.path, models, true)
			if err != nil {
				return fmt.Errorf("%s: %w", source.path, err)
			}
		}
	}

	return nil
}

func excludedSourceTree(relative string) bool {
	first, _, _ := strings.Cut(relative, "/")
	if strings.HasPrefix(first, ".") && first != "." {
		return true
	}
	// CLI production is independently inventoried by auditCLI. Verification,
	// fixtures and documentation tooling are not shipped provider modules.
	switch first {
	case "cmd", "tools", "tests", "docs", "node_modules", "vendor":
		return true
	default:
		return false
	}
}
