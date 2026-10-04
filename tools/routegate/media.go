package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

func verifyMediaRoute(function *ast.FuncDecl, call *ast.CallExpr, imports map[string]string) error {
	if len(call.Args) != mediaArguments {
		return fmt.Errorf("%w: unrecognized media helper signature", errRouteInvalid)
	}

	operation, recognized := generated(call.Args[2], imports, "Method")

	mediaOperation := operation == imageOperation || operation == clipOperation

	if !recognized || operation != function.Name.Name || !mediaOperation {
		return fmt.Errorf("%w: media method is not bound to its schema operation", errRouteInvalid)
	}

	if !mediaAuthorization(call.Args[4], operation, function, imports) {
		return fmt.Errorf("%w: media authorization is not schema-bound", errRouteInvalid)
	}

	if selector, recognized := call.Fun.(*ast.SelectorExpr); !recognized || !sameReceiver(selector.X, function) {
		return fmt.Errorf("%w: media receiver shadowed", errRouteInvalid)
	}

	endpoint, recognized := call.Args[3].(*ast.Ident)
	if !recognized || endpoint.Obj == nil {
		return fmt.Errorf("%w: unresolved media endpoint", errRouteInvalid)
	}

	assignments := 0
	valid := true

	ast.Inspect(function.Body, func(node ast.Node) bool {
		if assignment, recognized := node.(*ast.AssignStmt); recognized {
			count, acceptable := mediaAssignment(assignment, endpoint, operation)
			assignments += count
			valid = valid && acceptable
		}

		if escapes(node, endpoint, call) {
			valid = false
		}

		return true
	})

	if !valid || assignments != 1 {
		return fmt.Errorf("%w: provider media URL mutates or escapes validation", errRouteInvalid)
	}

	return nil
}

func mediaAuthorization(expression ast.Expr, operation string,
	function *ast.FuncDecl, imports map[string]string) bool {
	value, recognized := expression.(*ast.BinaryExpr)
	if !recognized || value.Op != token.ADD {
		return false
	}

	prefix := "BasicPrefix"
	field := "EventToken"

	if operation == clipOperation {
		prefix = "BearerPrefix"
		field = "AccessToken"
	}

	constant, recognized := value.X.(*ast.SelectorExpr)
	if !recognized || constant.Sel.Name != prefix {
		return false
	}

	owner, recognized := constant.X.(*ast.Ident)
	if !recognized || owner.Obj != nil || imports[owner.Name] != module+"/internal/protocol" ||
		imports[generatedBindingPrefix+prefix] == "" {
		return false
	}

	tokenField, recognized := value.Y.(*ast.SelectorExpr)
	if !recognized || tokenField.Sel.Name != field {
		return false
	}

	auth, recognized := tokenField.X.(*ast.SelectorExpr)
	if !recognized || auth.Sel.Name != "Auth" {
		return false
	}

	input, recognized := auth.X.(*ast.Ident)

	return recognized && isParameter(function, input)
}

func mediaAssignment(assignment *ast.AssignStmt, endpoint *ast.Ident, operation string) (int, bool) {
	count := 0
	valid := true

	for index, left := range assignment.Lhs {
		if identifier, recognized := left.(*ast.Ident); recognized && identifier.Obj == endpoint.Obj {
			count++
			valid = valid && index < len(assignment.Rhs) && checkedMediaURL(assignment.Rhs[index], operation)
		}

		selection, recognized := left.(*ast.SelectorExpr)
		if !recognized {
			continue
		}

		identifier, recognized := selection.X.(*ast.Ident)
		if !recognized || identifier.Obj != endpoint.Obj {
			continue
		}

		valid = valid && operation == imageOperation && selection.Sel.Name == "RawQuery" &&
			index < len(assignment.Rhs) && encodedQuery(assignment.Rhs[index])
	}

	return count, valid
}

func checkedMediaURL(expression ast.Expr, operation string) bool {
	constructor, recognized := expression.(*ast.CallExpr)
	if !recognized || len(constructor.Args) != keyValueArguments {
		return false
	}

	helper, recognized := constructor.Fun.(*ast.Ident)
	if !recognized || !packageFunction(helper, "checkedURL") {
		return false
	}

	pattern, recognized := constructor.Args[1].(*ast.Ident)

	expected := "imageURL"
	if operation == clipOperation {
		expected = "clipURL"
	}

	return recognized && pattern.Name == expected && validPatternObject(pattern, operation)
}

func encodedQuery(expression ast.Expr) bool {
	call, recognized := expression.(*ast.CallExpr)
	if !recognized {
		return false
	}

	method, recognized := call.Fun.(*ast.SelectorExpr)

	return recognized && method.Sel.Name == queryEncoder
}

func sameReceiver(expression ast.Expr, function *ast.FuncDecl) bool {
	owner, recognized := expression.(*ast.Ident)
	actual := receiver(function)

	return recognized && actual != nil && owner.Obj == actual.Obj
}
