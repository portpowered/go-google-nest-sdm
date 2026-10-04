package main

import (
	"go/ast"
	"go/token"
)

const headerAddMethod = "Add"

const headerSetMethod = "Set"

func safeRequestHeaderValue(state *exchangeAudit, call *ast.CallExpr) bool {
	if len(call.Args) != keyValueArguments {
		return false
	}

	name, recognized := generated(queryUnparen(call.Args[0]), state.imports, requestHeaderField)
	if !recognized {
		return false
	}

	value := queryUnparen(call.Args[1])

	switch name {
	case "Accept":
		return namedHeaderConstant(state, value, "MIMEApplicationJSON")
	case "ContentType":
		return state.parameter(value, "contentType")
	case "Authorization":
		if state.function.Name.Name == downloadHelper {
			return state.parameter(value, "authorization")
		}

		prefix, recognized := value.(*ast.BinaryExpr)

		return recognized && prefix.Op == token.ADD && namedHeaderConstant(state, prefix.X, "BearerPrefix") &&
			state.parameter(queryUnparen(prefix.Y), "token")
	default:
		return false
	}
}

func namedHeaderConstant(state *exchangeAudit, expression ast.Expr, name string) bool {
	selector, recognized := queryUnparen(expression).(*ast.SelectorExpr)
	if !recognized || selector.Sel.Name != name || state.imports[generatedBindingPrefix+name] == "" {
		return false
	}

	owner, recognized := selector.X.(*ast.Ident)

	return recognized && owner.Obj == nil && state.imports[owner.Name] == module+"/internal/protocol"
}

func safeHeaderParameterUse(state *exchangeAudit, identifier *ast.Ident) bool {
	parent := requestParent(state.parents, identifier)
	if binary, recognized := parent.(*ast.BinaryExpr); recognized {
		if binary.Op == token.EQL || binary.Op == token.NEQ {
			return true
		}

		parent = requestParent(state.parents, binary)
	}

	call, recognized := parent.(*ast.CallExpr)
	if !recognized {
		return false
	}

	if headerRequestCall(state, call) {
		return safeRequestHeaderValue(state, call)
	}

	// strings.ContainsAny is an inventoried pure token-validation primitive.
	selector, recognized := call.Fun.(*ast.SelectorExpr)
	if !recognized || selector.Sel.Name != "ContainsAny" || len(call.Args) != keyValueArguments ||
		queryUnparen(call.Args[0]) != identifier {
		return false
	}

	owner, recognized := selector.X.(*ast.Ident)

	return recognized && owner.Obj == nil && state.imports[owner.Name] == "strings"
}

func headerRequestCall(state *exchangeAudit, call *ast.CallExpr) bool {
	method, recognized := queryUnparen(call.Fun).(*ast.SelectorExpr)
	if !recognized || (method.Sel.Name != headerSetMethod && method.Sel.Name != headerAddMethod) {
		return false
	}

	return headerExpression(state, method.X, map[ast.Node]bool{})
}
