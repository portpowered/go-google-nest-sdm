package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"maps"
)

func (state *wireValueAudit) auditWireAssignment(assignment *ast.AssignStmt) error {
	for index, target := range assignment.Lhs {
		if index >= len(assignment.Rhs) || !state.reference(target).owned {
			continue
		}
		// Definitions and aliases retain provenance; the initializer is checked at the wire boundary.
		if _, recognized := target.(*ast.Ident); recognized && assignment.Tok == token.DEFINE {
			continue
		}

		expected := state.reference(target).name
		if !state.enumValueCompatible(assignment.Rhs[index], expected) {
			return fmt.Errorf("%w: generated wire field uses a different schema value domain", errRouteInvalid)
		}

		if state.fixedValue(assignment.Rhs[index], map[wireBinding]bool{}) {
			return fmt.Errorf("%w: handwritten fixed value assigned to generated wire field at %d",
				errRouteInvalid, target.Pos())
		}

		if state.fixedReceiverKey(target) {
			return fmt.Errorf("%w: handwritten fixed key assigned to generated wire map", errRouteInvalid)
		}
	}

	return nil
}

func (state *wireValueAudit) fixedComposite(composite *ast.CompositeLit, visited map[wireBinding]bool) bool {
	mapping := state.reference(composite.Type).mapping
	if _, nativeMap := composite.Type.(*ast.MapType); nativeMap {
		mapping = true
	}

	for _, element := range composite.Elts {
		if keyed, recognized := element.(*ast.KeyValueExpr); recognized {
			// Struct field identifiers are generated Go fields; map keys are wire protocol values.
			if _, fieldName := keyed.Key.(*ast.Ident); (mapping || !fieldName) &&
				(state.fixedValue(keyed.Key, visited) ||
					!state.enumValueCompatible(keyed.Key, state.reference(composite.Type).name)) {
				return true
			}

			if !state.compositeMemberDomain(composite, keyed) {
				return true
			}

			if state.fixedValue(keyed.Value, visited) {
				return true
			}
		} else if state.fixedValue(element, visited) {
			return true
		}
	}

	return false
}

func (state *wireValueAudit) fixedValue(expression ast.Expr, visited map[wireBinding]bool) bool {
	switch value := expression.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return state.fixedIdentifier(value, visited)
	case *ast.ParenExpr:
		return state.fixedValue(value.X, visited)
	case *ast.UnaryExpr:
		return state.fixedValue(value.X, visited)
	case *ast.StarExpr:
		return state.fixedValue(value.X, visited)
	case *ast.IndexExpr:
		return state.fixedValue(value.X, visited) ||
			state.reference(value.X).mapping && state.fixedValue(value.Index, visited)
	case *ast.TypeAssertExpr:
		return state.fixedValue(value.X, visited)
	case *ast.SliceExpr:
		return state.fixedValue(value.X, visited)
	case *ast.BinaryExpr:
		return state.fixedValue(value.X, visited) || state.fixedValue(value.Y, visited)
	case *ast.CompositeLit:
		return state.fixedComposite(value, visited)
	case *ast.CallExpr:
		return state.fixedCall(value, visited)
	case *ast.SelectorExpr:
		return state.fixedSelector(value, visited)
	case *ast.FuncLit:
		return state.fixedFunction(value.Type, value.Body, visited)
	}

	return true
}

func (state *wireValueAudit) fixedIdentifier(identifier *ast.Ident, visited map[wireBinding]bool) bool {
	if identifier.Name == "true" || identifier.Name == "false" {
		return true
	}

	if identifier.Obj == nil {
		return identifier.Name != nilIdentifier
	}

	key := bindingOf(identifier)
	if visited[key] {
		return false
	}

	visited[key] = true
	if argument, supplied := state.callArguments[key]; supplied {
		return state.fixedValue(argument, visited)
	}

	switch declaration := identifier.Obj.Decl.(type) {
	case *ast.AssignStmt:
		return state.fixedAssignments(identifier, visited)
	case *ast.ValueSpec:
		return state.fixedValueSpec(identifier, declaration, visited)
	}

	if state.callers[key] {
		return state.fixedAssignments(identifier, visited)
	}

	if state.namedResult(identifier) {
		return state.fixedAssignments(identifier, visited)
	}

	return true
}

func (state *wireValueAudit) fixedValueSpec(identifier *ast.Ident, declaration *ast.ValueSpec,
	visited map[wireBinding]bool) bool {
	if len(declaration.Values) == 0 {
		if state.zeroAggregate(identifier) {
			return false
		}

		if state.callers[bindingOf(identifier)] {
			return state.fixedAssignments(identifier, visited)
		}

		return true
	}

	for index, name := range declaration.Names {
		if name.Obj == identifier.Obj {
			if len(declaration.Values) == 1 {
				if call, recognized := declaration.Values[0].(*ast.CallExpr); recognized {
					return state.fixedCall(call, visited, index)
				}
			}

			if index < len(declaration.Values) {
				return state.fixedValue(declaration.Values[index], visited)
			}

			return true
		}
	}

	return true
}

func (state *wireValueAudit) fixedAssignments(identifier *ast.Ident, visited map[wireBinding]bool) bool {
	fixed := false

	ast.Inspect(state.file, func(node ast.Node) bool {
		assignment, recognized := node.(*ast.AssignStmt)
		if !recognized {
			return true
		}

		for index, left := range assignment.Lhs {
			other, recognized := left.(*ast.Ident)
			if !recognized || other.Obj != identifier.Obj {
				continue
			}

			if len(assignment.Rhs) == 1 {
				if call, called := assignment.Rhs[0].(*ast.CallExpr); called {
					fixed = fixed || state.fixedCall(call, visited, index)

					continue
				}
			}

			if index < len(assignment.Rhs) && state.fixedValue(assignment.Rhs[index], visited) {
				fixed = true
			}
		}

		return true
	})

	return fixed
}

func (state *wireValueAudit) fixedCall(call *ast.CallExpr, visited map[wireBinding]bool, slots ...int) bool {
	selected := 0
	if len(slots) != 0 {
		selected = slots[0]
	}

	if selector, recognized := call.Fun.(*ast.SelectorExpr); recognized && state.verifiedProviderCall(call, selector) {
		return false
	}

	if function := localCallable(call.Fun, map[wireBinding]bool{}); function.signature != nil {
		return state.fixedHelperCall(call, function, visited, selected)
	}

	if callback, recognized := call.Fun.(*ast.CallExpr); recognized {
		return state.fixedCall(callback, visited)
	}

	for _, argument := range call.Args {
		if state.fixedValue(argument, visited) {
			return true
		}
	}

	return !state.verifiedScalarCall(call)
}

func (state *wireValueAudit) fixedHelperCall(call *ast.CallExpr, function wireCallable,
	visited map[wireBinding]bool, selected int) bool {
	key := wireBinding{declaration: function.signature, name: "callable"}
	if visited[key] {
		return true
	}

	visited = maps.Clone(visited)
	visited[key] = true

	previous := state.callArguments
	state.callArguments = copyArguments(previous)
	index := 0

	for _, parameter := range function.signature.Params.List {
		for _, name := range parameter.Names {
			if index < len(call.Args) {
				state.callArguments[bindingOf(name)] = call.Args[index]
			}

			index++
		}
	}

	fixed := state.fixedFunction(function.signature, function.body, visited, selected)
	state.callArguments = previous

	return fixed
}

func copyArguments(source map[wireBinding]ast.Expr) map[wireBinding]ast.Expr {
	result := make(map[wireBinding]ast.Expr, len(source))
	maps.Copy(result, source)

	return result
}

func (state *wireValueAudit) fixedFunction(signature *ast.FuncType, body *ast.BlockStmt,
	visited map[wireBinding]bool, slots ...int) bool {
	selected := 0
	if len(slots) != 0 {
		selected = slots[0]
	}

	fixed := false

	field, namedIndex := resultField(signature, selected)

	if field != nil && len(field.Names) > namedIndex {
		fixed = state.fixedIdentifier(field.Names[namedIndex], visited)
	}

	ast.Inspect(body, func(node ast.Node) bool {
		statement, recognized := node.(*ast.ReturnStmt)
		if !recognized || len(statement.Results) == 0 {
			return true
		}

		if len(statement.Results) == 1 {
			if call, called := statement.Results[0].(*ast.CallExpr); called {
				fixed = fixed || state.fixedCall(call, visited, selected)

				return true
			}
		}

		if selected < len(statement.Results) && state.fixedValue(statement.Results[selected], visited) {
			fixed = true
		}

		return true
	})

	return fixed
}

func resultField(signature *ast.FuncType, selected int) (*ast.Field, int) {
	if signature.Results == nil {
		return nil, 0
	}

	index := 0

	for _, field := range signature.Results.List {
		count := max(1, len(field.Names))
		if selected < index+count {
			return field, selected - index
		}

		index += count
	}

	return nil, 0
}
