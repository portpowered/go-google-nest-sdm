package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
)

func loadWireModels(root string) (map[string]ast.Expr, error) {
	models := map[string]ast.Expr{}

	files, err := filepath.Glob(filepath.Join(root, "pkg/dependencymodels/*.gen.go"))
	if err != nil {
		return nil, fmt.Errorf("find generated wire models: %w", err)
	}

	for _, path := range files {
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return nil, fmt.Errorf("parse generated wire model: %w", parseErr)
		}

		for _, declaration := range file.Decls {
			group, recognized := declaration.(*ast.GenDecl)
			if !recognized || group.Tok != token.TYPE && group.Tok != token.CONST {
				continue
			}

			for _, item := range group.Specs {
				if constant, recognized := item.(*ast.ValueSpec); recognized {
					for _, name := range constant.Names {
						models["constant:"+name.Name] = constant.Type
					}

					continue
				}

				spec, recognized := item.(*ast.TypeSpec)
				if recognized {
					models[spec.Name.Name] = spec.Type
				}
			}
		}
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("%w: generated wire model inventory absent", errRouteInvalid)
	}

	return models, nil
}
