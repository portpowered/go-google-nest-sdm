package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
)

// auditWirePackage resolves package declarations before tracing wire provenance.
// Import qualifiers are unique per file so sibling helpers retain their own bindings.
func auditWirePackage(files []*ast.File, models map[string]ast.Expr) error {
	if len(files) == 0 {
		return nil
	}

	copies, err := clonePackageFiles(files)
	if err != nil {
		return err
	}

	files = copies
	declarations := packageDeclarations(files)
	imports := map[string]string{}
	merged := *files[0]
	merged.Decls = nil
	merged.Imports = nil

	for index, file := range files {
		bindings, err := auditImports(file)
		if err != nil {
			return err
		}

		renamePackageImports(file, bindings, imports, index)
		bindPackageIdentifiers(file, declarations)
		merged.Decls = append(merged.Decls, file.Decls...)
		merged.Imports = append(merged.Imports, file.Imports...)
	}

	bindGeneratedConstants(imports, models)

	return auditWireValues(&merged, imports, models)
}

func packageDeclarations(files []*ast.File) map[string]*ast.Ident {
	declarations := map[string]*ast.Ident{}

	for _, file := range files {
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if value.Recv == nil {
					declarations[value.Name.Name] = value.Name
				}
			case *ast.GenDecl:
				for _, spec := range value.Specs {
					switch definition := spec.(type) {
					case *ast.TypeSpec:
						declarations[definition.Name.Name] = definition.Name
					case *ast.ValueSpec:
						for _, name := range definition.Names {
							declarations[name.Name] = name
						}
					}
				}
			}
		}
	}

	return declarations
}

func renamePackageImports(file *ast.File, bindings, imports map[string]string, index int) {
	aliases := map[string]string{}

	for name, path := range bindings {
		alias := fmt.Sprintf("wirePackageFile%d_%s", index, name)
		aliases[name] = alias
		imports[alias] = path
	}

	ast.Inspect(file, func(node ast.Node) bool {
		selector, recognized := node.(*ast.SelectorExpr)
		if !recognized {
			return true
		}

		qualifier, recognized := selector.X.(*ast.Ident)
		if recognized && qualifier.Obj == nil && aliases[qualifier.Name] != "" {
			qualifier.Name = aliases[qualifier.Name]
		}

		return true
	})

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		for name, binding := range bindings {
			if binding == path {
				spec.Name = ast.NewIdent(aliases[name])
			}
		}
	}
}

func bindPackageIdentifiers(file *ast.File, declarations map[string]*ast.Ident) {
	// Parser objects preserve lexical identity; only unresolved package names are bound.
	imports := map[string]bool{}

	for _, spec := range file.Imports {
		if spec.Name != nil {
			imports[spec.Name.Name] = true
		}
	}

	for _, identifier := range file.Unresolved {
		declaration := declarations[identifier.Name]
		if identifier.Obj == nil && declaration != nil && !imports[identifier.Name] {
			identifier.Obj = declaration.Obj
		}
	}
}

func clonePackageFiles(files []*ast.File) ([]*ast.File, error) {
	copies := make([]*ast.File, 0, len(files))

	for index, file := range files {
		var source bytes.Buffer

		err := format.Node(&source, token.NewFileSet(), file)
		if err != nil {
			return nil, fmt.Errorf("format package AST: %w", err)
		}

		clone, err := parser.ParseFile(token.NewFileSet(), fmt.Sprintf("file%d.go", index), source.Bytes(), 0)
		if err != nil {
			return nil, fmt.Errorf("parse package AST: %w", err)
		}

		copies = append(copies, clone)
	}

	return copies, nil
}
