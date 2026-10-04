package main

import (
	"go/ast"
	"go/token"
)

func safeQueryAppend(expression ast.Expr, function *ast.FuncDecl, imports map[string]string) bool {
	binary, recognized := expression.(*ast.BinaryExpr)
	if !recognized || binary.Op != token.ADD {
		return false
	}

	literal, recognized := binary.X.(*ast.BasicLit)
	if !recognized || literal.Value != `"?"` {
		return false
	}

	encode, recognized := binary.Y.(*ast.CallExpr)
	if !recognized || len(encode.Args) != 0 {
		return false
	}

	selector, recognized := encode.Fun.(*ast.SelectorExpr)
	if !recognized || selector.Sel.Name != queryEncoder {
		return false
	}

	query, recognized := selector.X.(*ast.Ident)
	if !recognized || query.Obj == nil {
		return false
	}

	parents := nodeParents(function.Body)
	assignments := 0
	valid := true

	ast.Inspect(function.Body, func(node ast.Node) bool {
		identifier, recognized := node.(*ast.Ident)
		if !recognized || identifier.Obj != query.Obj {
			return true
		}

		parent := parents[identifier]
		if assignment, recognized := parent.(*ast.AssignStmt); recognized {
			assignments++
			valid = valid && queryDeclaration(assignment, identifier, imports)

			return true
		}

		method, recognized := parent.(*ast.SelectorExpr)
		if !recognized || method.X != identifier {
			valid = false

			return true
		}

		call, recognized := parents[method].(*ast.CallExpr)
		valid = valid && recognized && queryCall(call, method, encode, imports)

		return true
	})

	return valid && assignments == 1
}

func queryDeclaration(assignment *ast.AssignStmt, identifier *ast.Ident, imports map[string]string) bool {
	if assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 ||
		assignment.Lhs[0] != identifier {
		return false
	}

	composite, recognized := assignment.Rhs[0].(*ast.CompositeLit)
	if !recognized || len(composite.Elts) != 0 {
		return false
	}

	kind, recognized := composite.Type.(*ast.SelectorExpr)
	if !recognized || kind.Sel.Name != "Values" {
		return false
	}

	owner, recognized := kind.X.(*ast.Ident)

	return recognized && owner.Obj == nil && imports[owner.Name] == "net/url"
}

func queryCall(call *ast.CallExpr, method *ast.SelectorExpr, encode *ast.CallExpr, imports map[string]string) bool {
	switch method.Sel.Name {
	case "Set", "Add":
		if len(call.Args) != keyValueArguments {
			return false
		}

		_, recognized := generated(call.Args[0], imports, "Query")

		return recognized
	case queryEncoder:
		return call == encode
	default:
		return false
	}
}

func nodeParents(body *ast.BlockStmt) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}

	var stack []ast.Node

	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]

			return false
		}

		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}

		stack = append(stack, node)

		return true
	})

	return parents
}
