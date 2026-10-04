package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

type exchangeAudit struct {
	function    *ast.FuncDecl
	imports     map[string]string
	parameters  map[string]*ast.Ident
	parents     map[ast.Node]ast.Node
	constructor *ast.CallExpr
	send        *ast.CallExpr
	request     *ast.Ident
	valid       bool
}

func verifyExchange(function *ast.FuncDecl, imports map[string]string) error {
	state := exchangeAudit{
		function: function, imports: imports, parameters: map[string]*ast.Ident{},
		parents: nodeParents(function.Body), valid: true, constructor: nil, send: nil, request: nil,
	}

	for _, field := range function.Type.Params.List {
		for _, name := range field.Names {
			state.parameters[name.Name] = name
		}
	}

	ast.Inspect(function.Body, state.inspectCalls)

	if state.constructor == nil || state.send == nil || state.request == nil || !state.valid {
		return fmt.Errorf("%w: network helper must construct and send precisely one injected request", errRouteInvalid)
	}

	identifier, recognized := state.send.Args[0].(*ast.Ident)
	if !recognized || identifier.Obj != state.request.Obj || state.constructor.Pos() >= state.send.Pos() {
		return fmt.Errorf("%w: constructed and sent request differ", errRouteInvalid)
	}

	ast.Inspect(function.Body, state.inspectUses)

	if !state.valid {
		return fmt.Errorf("%w: request, target or headers mutate or escape verified helper", errRouteInvalid)
	}

	return nil
}

func (state *exchangeAudit) inspectCalls(node ast.Node) bool {
	call, recognized := node.(*ast.CallExpr)
	if !recognized {
		return true
	}

	selector, recognized := call.Fun.(*ast.SelectorExpr)
	if !recognized {
		return true
	}

	owner, ownerOK := selector.X.(*ast.Ident)
	if ownerOK && owner.Obj == nil && state.imports[owner.Name] == httpImport && selector.Sel.Name == requestConstructor {
		state.inspectConstructor(call)
	}

	if selector.Sel.Name == "Do" {
		if state.send != nil || !receiverField(selector.X, state.function, "httpClient") || len(call.Args) != 1 {
			state.valid = false
		}

		state.send = call
	}

	return true
}

func (state *exchangeAudit) inspectConstructor(call *ast.CallExpr) {
	if state.constructor != nil || len(call.Args) != 4 {
		state.valid = false

		return
	}

	state.constructor = call
	for index, name := range []string{"ctx", "method", "endpoint", "body"} {
		state.valid = state.valid && state.constructorArgument(call.Args[index], name)
	}

	assignment, recognized := state.parents[call].(*ast.AssignStmt)
	if !recognized || len(assignment.Lhs) != keyValueArguments {
		state.valid = false

		return
	}

	identifier, recognized := assignment.Lhs[0].(*ast.Ident)
	if !recognized {
		state.valid = false

		return
	}

	state.request = identifier
}

func (state *exchangeAudit) constructorArgument(expression ast.Expr, name string) bool {
	if state.function.Name.Name == downloadHelper {
		if name == "endpoint" {
			return state.endpointString(expression)
		}

		if name == "body" {
			identifier, recognized := expression.(*ast.Ident)

			return recognized && identifier.Name == nilIdentifier
		}
	}

	return state.parameter(expression, name)
}

func (state *exchangeAudit) parameter(expression ast.Expr, name string) bool {
	identifier, recognized := expression.(*ast.Ident)
	parameter := state.parameters[name]

	return recognized && parameter != nil && identifier.Obj == parameter.Obj
}

func (state *exchangeAudit) endpointString(expression ast.Expr) bool {
	call, recognized := expression.(*ast.CallExpr)
	if !recognized || len(call.Args) != 0 {
		return false
	}

	selector, recognized := call.Fun.(*ast.SelectorExpr)

	return recognized && selector.Sel.Name == "String" && state.parameter(selector.X, "endpoint")
}

func (state *exchangeAudit) inspectUses(node ast.Node) bool {
	if assignment, recognized := node.(*ast.AssignStmt); recognized {
		state.inspectAssignment(assignment)
	}

	identifier, recognized := node.(*ast.Ident)
	if !recognized {
		return true
	}

	if state.parameter(identifier, "method") || state.parameter(identifier, "endpoint") {
		state.valid = state.valid && state.allowedParameterUse(identifier)
	}

	if identifier.Obj == state.request.Obj {
		state.valid = state.valid && state.allowedRequestUse(identifier)
	}

	return true
}

func (state *exchangeAudit) inspectAssignment(assignment *ast.AssignStmt) {
	if assignment.Pos() == state.parents[state.constructor].Pos() {
		return
	}

	for _, left := range assignment.Lhs {
		ast.Inspect(left, func(child ast.Node) bool {
			identifier, recognized := child.(*ast.Ident)
			if recognized && (state.parameter(identifier, "method") || state.parameter(identifier, "endpoint") ||
				identifier.Obj == state.request.Obj) {
				state.valid = false
			}

			return true
		})
	}
}

func (state *exchangeAudit) allowedParameterUse(identifier *ast.Ident) bool {
	parent := state.parents[identifier]
	if state.function.Name.Name == downloadHelper && state.parameter(identifier, "endpoint") {
		if selector, recognized := parent.(*ast.SelectorExpr); recognized && selector.Sel.Name == "String" {
			call, recognized := state.parents[selector].(*ast.CallExpr)
			if recognized && state.parents[call] == state.constructor {
				return true
			}
		}
	}

	if parent == state.constructor {
		return true
	}

	comparison, recognized := parent.(*ast.BinaryExpr)

	return recognized && (comparison.Op == token.NEQ || comparison.Op == token.EQL)
}

func (state *exchangeAudit) allowedRequestUse(identifier *ast.Ident) bool {
	switch parent := state.parents[identifier].(type) {
	case *ast.AssignStmt:
		return parent == state.parents[state.constructor]
	case *ast.CallExpr:
		return parent == state.send
	case *ast.SelectorExpr:
		return state.allowedHeaderUse(parent)
	default:
		return false
	}
}

func (state *exchangeAudit) allowedHeaderUse(parent *ast.SelectorExpr) bool {
	if parent.Sel.Name != "Header" {
		return false
	}

	method, recognized := state.parents[parent].(*ast.SelectorExpr)
	if !recognized || (method.Sel.Name != "Set" && method.Sel.Name != "Add") {
		return false
	}

	call, recognized := state.parents[method].(*ast.CallExpr)
	if !recognized || len(call.Args) != keyValueArguments {
		return false
	}

	_, recognized = generated(call.Args[0], state.imports, "Header")

	return recognized
}
