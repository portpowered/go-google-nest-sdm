package main

import (
	"go/ast"
	"strings"
)

// unionWireFields recognizes oapi-codegen's private oneOf backing storage. It
// leaves every other untagged field visible to the ordinary wire-field check.
func unionWireFields(file *ast.File, model *ast.TypeSpec, structure *ast.StructType,
	expected objectContract) *ast.StructType {
	if len(expected.OneOf) < 2 || !unionJSONImport(file) ||
		!unionCodec(file, model.Name.Name, "MarshalJSON") ||
		!unionCodec(file, model.Name.Name, "UnmarshalJSON") {
		return structure
	}

	fields := []*ast.Field{}

	for _, field := range structure.Fields.List {
		if field.Tag == nil && len(field.Names) == 1 && field.Names[0].Name == "union" &&
			typeExpression(field.Type) == "json.RawMessage" {
			continue
		}

		fields = append(fields, field)
	}

	result := *structure
	list := *structure.Fields
	list.List = fields
	result.Fields = &list

	return &result
}

func unionJSONImport(file *ast.File) bool {
	for _, imported := range file.Imports {
		if strings.Trim(imported.Path.Value, `"`) == "encoding/json" &&
			(imported.Name == nil || imported.Name.Name == "json") {
			return true
		}
	}

	return false
}

func unionCodec(file *ast.File, model, codec string) bool {
	for _, declaration := range file.Decls {
		function, recognized := declaration.(*ast.FuncDecl)
		if !recognized || function.Name.Name != codec || function.Recv == nil ||
			len(function.Recv.List) != 1 || function.Body == nil || len(function.Body.List) == 0 {
			continue
		}

		field := function.Recv.List[0]
		if len(field.Names) != 1 || strings.TrimPrefix(typeExpression(field.Type), "*") != model {
			continue
		}

		assignment, recognized := function.Body.List[0].(*ast.AssignStmt)
		if !recognized || len(assignment.Rhs) != 1 {
			continue
		}

		call, recognized := assignment.Rhs[0].(*ast.CallExpr)
		if !recognized {
			continue
		}

		method, recognized := call.Fun.(*ast.SelectorExpr)
		if !recognized || method.Sel.Name != codec {
			continue
		}

		cache, recognized := method.X.(*ast.SelectorExpr)
		if !recognized || cache.Sel.Name != "union" {
			continue
		}

		owner, recognized := cache.X.(*ast.Ident)
		if recognized && owner.Obj == field.Names[0].Obj {
			return true
		}
	}

	return false
}
