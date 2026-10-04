package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
)

func packageFunction(identifier *ast.Ident, name string) bool {
	return identifier.Name == name && (identifier.Obj == nil || identifier.Obj.Kind == ast.Fun)
}

func auditHelpers(root string) error {
	file, err := parser.ParseFile(token.NewFileSet(),
		filepath.Join(root, "pkg/dependencies/httptransport/resources.go"), nil, 0)
	if err != nil {
		return fmt.Errorf("parse route helper source: %w", err)
	}

	imports, err := auditImports(file)
	if err != nil {
		return err
	}

	helper := namedFunction(file, "resourcePath")
	if helper == nil || !safeResourceHelper(helper, imports) {
		return fmt.Errorf("%w: resourcePath does not preserve generated route", errRouteInvalid)
	}

	file, err = parser.ParseFile(token.NewFileSet(),
		filepath.Join(root, "pkg/dependencies/media/download.go"), nil, 0)
	if err != nil {
		return fmt.Errorf("parse media helper source: %w", err)
	}

	imports, err = auditImports(file)
	if err != nil {
		return err
	}

	return auditMediaPatterns(file, imports)
}

func namedFunction(file *ast.File, name string) *ast.FuncDecl {
	for _, declaration := range file.Decls {
		function, recognized := declaration.(*ast.FuncDecl)
		if recognized && function.Name.Name == name {
			return function
		}
	}

	return nil
}

func importedCall(expression ast.Expr, imports map[string]string, packagePath, method string) (*ast.CallExpr, bool) {
	call, recognized := expression.(*ast.CallExpr)
	if !recognized {
		return nil, false
	}

	selector, recognized := call.Fun.(*ast.SelectorExpr)
	if !recognized || selector.Sel.Name != method {
		return nil, false
	}

	owner, recognized := selector.X.(*ast.Ident)

	return call, recognized && owner.Obj == nil && imports[owner.Name] == packagePath
}

func resourceReturn(function *ast.FuncDecl,
	imports map[string]string) (*ast.ReturnStmt, *ast.CallExpr, *ast.Ident, *ast.Ident, bool) {
	if function.Body == nil || len(function.Body.List) == 0 {
		return nil, nil, nil, nil, false
	}

	terminal, recognized := function.Body.List[len(function.Body.List)-1].(*ast.ReturnStmt)
	if !recognized || len(terminal.Results) != keyValueArguments {
		return nil, nil, nil, nil, false
	}

	empty, recognized := terminal.Results[1].(*ast.Ident)
	if !recognized || empty.Name != nilIdentifier {
		return nil, nil, nil, nil, false
	}

	call, recognized := importedCall(terminal.Results[0], imports, "fmt", "Sprintf")
	if !recognized || len(call.Args) != keyValueArguments || !call.Ellipsis.IsValid() {
		return nil, nil, nil, nil, false
	}

	template, recognized := call.Args[0].(*ast.Ident)
	if !recognized || template.Name != "template" || !isParameter(function, template) {
		return nil, nil, nil, nil, false
	}

	values, recognized := call.Args[1].(*ast.Ident)
	if !recognized || values.Obj == nil {
		return nil, nil, nil, nil, false
	}

	return terminal, call, template, values, true
}

func safeResourceHelper(function *ast.FuncDecl, imports map[string]string) bool {
	terminal, call, template, values, recognized := resourceReturn(function, imports)
	if !recognized {
		return false
	}

	parents := nodeParents(function.Body)
	valid := true
	assignments := 0

	ast.Inspect(function.Body, func(node ast.Node) bool {
		if other, recognized := node.(*ast.ReturnStmt); recognized && other != terminal {
			valid = valid && errorOnlyReturn(other)
		}

		identifier, recognized := node.(*ast.Ident)
		if !recognized {
			return true
		}

		if identifier.Obj == template.Obj && identifier != template {
			valid = false
		}

		if identifier.Obj != values.Obj {
			return true
		}

		parent := parents[identifier]
		if parent == call {
			return true
		}

		if assignment, recognized := parent.(*ast.AssignStmt); recognized {
			assignments++
			valid = valid && valuesDeclaration(assignment, identifier)

			return true
		}

		valid = valid && escapedValuesUse(parent, identifier, parents, function, imports)

		return true
	})

	return valid && assignments == 1
}

func valuesDeclaration(assignment *ast.AssignStmt, identifier *ast.Ident) bool {
	if assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 ||
		assignment.Lhs[0] != identifier {
		return false
	}

	call, recognized := assignment.Rhs[0].(*ast.CallExpr)
	if !recognized || len(call.Args) != keyValueArguments {
		return false
	}

	builtin, recognized := call.Fun.(*ast.Ident)
	if !recognized || builtin.Name != "make" || builtin.Obj != nil {
		return false
	}

	slice, recognized := call.Args[0].(*ast.ArrayType)
	if !recognized || slice.Len != nil {
		return false
	}

	element, recognized := slice.Elt.(*ast.Ident)

	return recognized && element.Name == "any" && element.Obj == nil
}

func escapedValueAssignment(assignment *ast.AssignStmt, index *ast.IndexExpr,
	function *ast.FuncDecl, imports map[string]string) bool {
	if assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 ||
		assignment.Lhs[0] != index {
		return false
	}

	call, recognized := importedCall(assignment.Rhs[0], imports, "net/url", "PathEscape")
	if !recognized || len(call.Args) != 1 {
		return false
	}

	identifier, recognized := call.Args[0].(*ast.Ident)

	return recognized && identifier.Obj != nil && identifier.Obj.Kind == ast.Var && !isParameter(function, identifier)
}

func auditMediaPatterns(file *ast.File, imports map[string]string) error {
	for name, operation := range map[string]string{"imageURL": imageOperation, "clipURL": clipOperation} {
		object := file.Scope.Lookup(name)
		if object == nil {
			return fmt.Errorf("%w: missing media pattern %s", errRouteInvalid, name)
		}

		spec, recognized := object.Decl.(*ast.ValueSpec)
		if !recognized || !mediaPatternValue(spec, name, operation, imports) {
			return fmt.Errorf("%w: media pattern %s is not generated", errRouteInvalid, name)
		}
	}

	return nil
}

func mediaPatternValue(spec *ast.ValueSpec, name, operation string, imports map[string]string) bool {
	if len(spec.Names) != 1 || spec.Names[0].Name != name || len(spec.Values) != 1 {
		return false
	}

	call, recognized := importedCall(spec.Values[0], imports, "regexp", "MustCompile")
	if !recognized || len(call.Args) != 1 {
		return false
	}

	pattern, recognized := generated(call.Args[0], imports, "Media")

	expected := "ImageURLPattern"
	if operation == clipOperation {
		expected = "ClipURLPattern"
	}

	return recognized && pattern == expected
}

func validPatternObject(identifier *ast.Ident, operation string) bool {
	if identifier.Obj == nil {
		return true
	}

	spec, recognized := identifier.Obj.Decl.(*ast.ValueSpec)
	if !recognized {
		return false
	}

	imports := map[string]string{"regexp": "regexp", "protocol": module + "/internal/protocol"}

	return mediaPatternValue(spec, identifier.Name, operation, imports)
}

func escapedValuesUse(parent ast.Node, identifier *ast.Ident, parents map[ast.Node]ast.Node,
	function *ast.FuncDecl, imports map[string]string) bool {
	index, recognized := parent.(*ast.IndexExpr)
	if !recognized || index.X != identifier {
		return false
	}

	assignment, recognized := parents[index].(*ast.AssignStmt)

	return recognized && escapedValueAssignment(assignment, index, function, imports)
}

func errorOnlyReturn(statement *ast.ReturnStmt) bool {
	if len(statement.Results) != keyValueArguments {
		return false
	}

	empty, recognized := statement.Results[0].(*ast.BasicLit)
	if !recognized || empty.Kind != token.STRING || empty.Value != `""` {
		return false
	}

	result, recognized := statement.Results[1].(*ast.Ident)

	return !recognized || result.Name != nilIdentifier
}
