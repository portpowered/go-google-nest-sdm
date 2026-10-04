package main

import (
	"go/ast"
	"go/token"
	"slices"
)

type routeFlow struct {
	function  *ast.FuncDecl
	send      *ast.CallExpr
	path      *ast.Ident
	operation string
	imports   map[string]string
	parents   map[ast.Node]ast.Node
	found     bool
	safe      bool
}

func (state *routeFlow) walk(statements []ast.Stmt, value bool) bool {
	for _, statement := range statements {
		if state.found {
			break
		}

		switch node := statement.(type) {
		case *ast.AssignStmt:
			if containsRouteSend(node, state.send) {
				state.found = true
				state.safe = value

				break
			}

			value = state.assignment(node, value)
		case *ast.IfStmt:
			value = state.conditional(node, value)
		case *ast.BlockStmt:
			value = state.walk(node.List, value)
		default:
			if containsRouteSend(node, state.send) {
				_, returned := node.(*ast.ReturnStmt)
				_, called := node.(*ast.ExprStmt)
				state.found = true
				state.safe = value && (returned || called)
			}

			if routeAssignments(node, state.path) {
				value = false
			}
		}
	}

	return value
}

func (state *routeFlow) conditional(statement *ast.IfStmt, value bool) bool {
	if statement.Init != nil {
		value = state.walk([]ast.Stmt{statement.Init}, value)
	}

	if containsRouteSend(statement.Cond, state.send) {
		state.found = true
		state.safe = false

		return false
	}

	left := state.walk(statement.Body.List, value)
	if state.found {
		return left
	}

	right := value
	if statement.Else != nil {
		right = state.walk([]ast.Stmt{statement.Else}, value)
	}

	return left && right
}

func (state *routeFlow) assignment(assignment *ast.AssignStmt, value bool) bool {
	for position, target := range assignment.Lhs {
		identifier, ok := queryUnparen(target).(*ast.Ident)
		if !ok || identifier.Obj != state.path.Obj {
			continue
		}

		if position >= len(assignment.Rhs) {
			return false
		}

		if assignment.Tok == token.ADD_ASSIGN {
			return value && safeQueryAppend(assignment.Rhs[position], state.function, state.imports)
		}

		if assignment.Tok != token.ASSIGN && assignment.Tok != token.DEFINE {
			return false
		}

		if operation, generatedPath := generated(assignment.Rhs[position], state.imports, "Path"); generatedPath {
			return operation == state.operation
		}

		return pathConstructor(assignment.Rhs[position], state.function, state.operation, state.imports)
	}

	return value
}

func (state *routeFlow) safeUses() bool {
	for parent := state.parents[state.send]; parent != nil; parent = state.parents[parent] {
		switch parent.(type) {
		case *ast.FuncLit, *ast.GoStmt, *ast.DeferStmt:
			return false
		}
	}

	valid := true

	ast.Inspect(state.function.Body, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok || identifier.Obj != state.path.Obj {
			return true
		}

		parent := state.parents[identifier]

		for {
			if _, wrapped := parent.(*ast.ParenExpr); !wrapped {
				break
			}

			parent = state.parents[parent]
		}

		if assignment, assigned := parent.(*ast.AssignStmt); assigned {
			left := false

			for _, target := range assignment.Lhs {
				name, isIdentifier := queryUnparen(target).(*ast.Ident)
				left = left || isIdentifier && name.Obj == identifier.Obj
			}

			valid = valid && left

			return valid
		}

		if declaration, declared := parent.(*ast.ValueSpec); declared {
			if slices.Contains(declaration.Names, identifier) {
				return true
			}
		}

		valid = valid && routeNodeContains(state.send.Args[3], identifier)

		return valid
	})

	return valid
}

func containsRouteSend(node ast.Node, send *ast.CallExpr) bool {
	found := false

	ast.Inspect(node, func(candidate ast.Node) bool {
		if candidate == send {
			found = true
		}

		return !found
	})

	return found
}

func routeNodeContains(node ast.Node, target ast.Node) bool {
	found := false

	ast.Inspect(node, func(candidate ast.Node) bool {
		if candidate == target {
			found = true
		}

		return !found
	})

	return found
}

func routeAssignments(node ast.Node, path *ast.Ident) bool {
	found := false

	ast.Inspect(node, func(candidate ast.Node) bool {
		assignment, ok := candidate.(*ast.AssignStmt)
		if ok {
			for _, target := range assignment.Lhs {
				identifier, recognized := queryUnparen(target).(*ast.Ident)
				found = found || recognized && identifier.Obj == path.Obj
			}
		}

		return !found
	})

	return found
}
