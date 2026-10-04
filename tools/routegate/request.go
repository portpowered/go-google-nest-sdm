package main

import "go/ast"

const requestHeaderField = "Header"

const exchangeEndpointParameter = "endpoint"

// safeRequestBoundary admits only the constructor, synchronous schema-keyed header
// writes, and the injected send. Aliases, clones, URL access, body factories and
// framing fields cannot inherit trust from the original request constructor.
func safeRequestBoundary(state *exchangeAudit) bool {
	if !requestClientReceiver(state.function) {
		return false
	}

	construction, recognized := state.parents[state.constructor].(*ast.AssignStmt)
	if !recognized || state.parents[construction] != state.function.Body {
		return false
	}

	send, recognized := state.parents[state.send].(*ast.AssignStmt)
	if !recognized || state.parents[send] != state.function.Body {
		return false
	}

	valid := true

	ast.Inspect(state.function.Body, func(node ast.Node) bool {
		identifier, recognized := node.(*ast.Ident)
		if !recognized {
			return true
		}

		if identifier.Obj == receiver(state.function).Obj {
			valid = valid && safeInjectedClientUse(state, identifier)
		}

		if identifier.Obj != state.request.Obj {
			return true
		}

		selector, recognized := requestParent(state.parents, identifier).(*ast.SelectorExpr)
		if !recognized || selector.Sel.Name != requestHeaderField {
			return true
		}

		valid = valid && immediateHeaderWrite(state, selector)

		return true
	})

	return valid
}

func safeInjectedClientUse(state *exchangeAudit, identifier *ast.Ident) bool {
	field, recognized := requestParent(state.parents, identifier).(*ast.SelectorExpr)
	if !recognized {
		return false
	}

	if field.Sel.Name == "oauthBaseURL" {
		_, recognized = requestParent(state.parents, field).(*ast.BinaryExpr)

		return recognized
	}

	if field.Sel.Name != "httpClient" {
		return false
	}

	method, recognized := requestParent(state.parents, field).(*ast.SelectorExpr)
	if !recognized || method.Sel.Name != "Do" {
		return false
	}

	return state.parents[method] == state.send
}

func requestClientReceiver(function *ast.FuncDecl) bool {
	if receiver(function) == nil {
		return false
	}

	pointer, recognized := function.Recv.List[0].Type.(*ast.StarExpr)
	if !recognized {
		return false
	}

	client, recognized := pointer.X.(*ast.Ident)

	return recognized && client.Name == "Client"
}

func immediateHeaderWrite(state *exchangeAudit, header *ast.SelectorExpr) bool {
	return safeHeaderOrigin(state, header)
}

func synchronousHeaderCall(state *exchangeAudit, call *ast.CallExpr) bool {
	if call.Pos() <= state.constructor.Pos() || call.Pos() >= state.send.Pos() {
		return false
	}

	statement, recognized := state.parents[call].(*ast.ExprStmt)
	if !recognized {
		return false
	}

	for parent := state.parents[statement]; parent != state.function.Body; parent = state.parents[parent] {
		switch parent.(type) {
		case *ast.BlockStmt, *ast.IfStmt:
			// Conditional synchronous header writes preserve the generated request.
		default:
			return false
		}
	}

	return true
}

func requestParent(parents map[ast.Node]ast.Node, node ast.Node) ast.Node {
	parent := parents[node]

	for {
		wrapped, recognized := parent.(*ast.ParenExpr)
		if !recognized {
			return parent
		}

		parent = parents[wrapped]
	}
}
