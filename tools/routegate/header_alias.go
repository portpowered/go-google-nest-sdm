package main

import (
	"go/ast"
	"go/token"
)

func headerConversion(state *exchangeAudit, call *ast.CallExpr) bool {
	selector, recognized := queryUnparen(call.Fun).(*ast.SelectorExpr)
	if !recognized || selector.Sel.Name != requestHeaderField || len(call.Args) != 1 {
		return false
	}

	owner, recognized := selector.X.(*ast.Ident)

	return recognized && owner.Obj == nil && state.imports[owner.Name] == httpImport
}

func headerExpression(state *exchangeAudit, expression ast.Expr, seen map[ast.Node]bool) bool {
	switch value := queryUnparen(expression).(type) {
	case *ast.SelectorExpr:
		identifier, recognized := queryUnparen(value.X).(*ast.Ident)

		return recognized && value.Sel.Name == requestHeaderField && identifier.Obj == state.request.Obj
	case *ast.CallExpr:
		return headerConversion(state, value) && headerExpression(state, value.Args[0], seen)
	case *ast.Ident:
		if value.Obj == nil {
			return false
		}

		assignment, recognized := value.Obj.Decl.(*ast.AssignStmt)
		if !recognized || seen[assignment] || assignment.Tok != token.DEFINE ||
			len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return false
		}

		seen[assignment] = true

		return headerExpression(state, assignment.Rhs[0], seen)
	default:
		return false
	}
}

func safeHeaderOrigin(state *exchangeAudit, expression ast.Expr) bool {
	parent := requestParent(state.parents, expression)
	if call, recognized := parent.(*ast.CallExpr); recognized && headerConversion(state, call) {
		return safeHeaderOrigin(state, call)
	}

	if assignment, recognized := parent.(*ast.AssignStmt); recognized {
		if assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 ||
			queryUnparen(assignment.Rhs[0]) != queryUnparen(expression) ||
			state.parents[assignment] != state.function.Body ||
			assignment.Pos() <= state.constructor.Pos() || assignment.Pos() >= state.send.Pos() {
			return false
		}

		alias, recognized := assignment.Lhs[0].(*ast.Ident)

		return recognized && safeHeaderAliasUses(state, alias, assignment)
	}

	method, recognized := parent.(*ast.SelectorExpr)
	if !recognized || (method.Sel.Name != headerSetMethod && method.Sel.Name != headerAddMethod) {
		return false
	}

	call, recognized := requestParent(state.parents, method).(*ast.CallExpr)

	return recognized && headerRequestCall(state, call) &&
		safeRequestHeaderValue(state, call) && synchronousHeaderCall(state, call)
}

func safeHeaderAliasUses(state *exchangeAudit, alias *ast.Ident, declaration *ast.AssignStmt) bool {
	valid := true

	ast.Inspect(state.function.Body, func(node ast.Node) bool {
		identifier, recognized := node.(*ast.Ident)
		if !recognized || identifier.Obj != alias.Obj || identifier == alias {
			return true
		}

		valid = valid && identifier.Pos() > declaration.End() && safeHeaderOrigin(state, identifier)

		return true
	})

	return valid
}
