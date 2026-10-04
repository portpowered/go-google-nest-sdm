package main

import (
	"go/ast"
	"go/token"
	"maps"
)

func (state *wireValueAudit) generatedConstantFixed(selector *ast.SelectorExpr) bool {
	_, exists := state.constantType(selector)

	return !exists
}
func (state *wireValueAudit) constantType(selector *ast.SelectorExpr) (ast.Expr, bool) {
	owner, ok := selector.X.(*ast.Ident)
	if !ok || owner.Obj != nil {
		return nil, false
	}

	prefix := catalogPrefix(state.imports[owner.Name])
	if prefix == catalogSkipped {
		return nil, false
	}

	value, exists := state.models[prefix+"constant:"+selector.Sel.Name]

	return value, exists
}
func (state *wireValueAudit) enumValueCompatible(expression ast.Expr, expectedTypeName string) bool {
	return state.compatibleEnum(expression, expectedTypeName, map[wireBinding]bool{})
}
func (state *wireValueAudit) compatibleEnum(expression ast.Expr, expected string, visited map[wireBinding]bool) bool {
	switch value := expression.(type) {
	case *ast.SelectorExpr:
		return state.enumSelectorCompatible(value, expected)
	case *ast.CallExpr:
		if callback, isCallback := value.Fun.(*ast.CallExpr); isCallback &&
			!state.compatibleEnum(callback, expected, visited) {
			return false
		}

		function := localCallable(value.Fun, map[wireBinding]bool{})
		if function.signature != nil {
			return state.enumCallCompatible(value, function, expected, visited, 0)
		}

		return state.enumExpressionsCompatible(value.Args, expected, visited)
	case *ast.ParenExpr:
		return state.compatibleEnum(value.X, expected, visited)
	case *ast.UnaryExpr:
		return state.compatibleEnum(value.X, expected, visited)
	case *ast.BinaryExpr:
		return state.compatibleEnum(value.X, expected, visited) && state.compatibleEnum(value.Y, expected, visited)
	case *ast.IndexExpr:
		return state.compatibleEnum(value.X, expected, visited)
	case *ast.StarExpr:
		return state.compatibleEnum(value.X, expected, visited)
	case *ast.TypeAssertExpr:
		return state.compatibleEnum(value.X, expected, visited)
	case *ast.SliceExpr:
		return state.compatibleEnum(value.X, expected, visited)
	case *ast.CompositeLit:
		return state.enumAggregateCompatible(value, expected, visited)
	case *ast.Ident:
		return state.enumIdentifierCompatible(value, expected, visited)
	case *ast.FuncLit:
		return state.enumReturnsCompatible(value.Type, value.Body, expected, visited, 0)
	}

	return true
}

func (state *wireValueAudit) enumAggregateCompatible(composite *ast.CompositeLit,
	expected string, visited map[wireBinding]bool) bool {
	if state.reference(composite.Type).structure != nil {
		return true // Each generated struct member is checked against its own original field binding.
	}

	for _, element := range composite.Elts {
		if member, keyed := element.(*ast.KeyValueExpr); keyed {
			element = member.Value
		}

		if !state.compatibleEnum(element, expected, visited) {
			return false
		}
	}

	return true
}
func (state *wireValueAudit) enumSelectorCompatible(selector *ast.SelectorExpr, expected string) bool {
	constant, exists := state.constantType(selector)
	if !exists {
		return true
	}

	name, namedType := constant.(*ast.Ident)
	if !namedType {
		return false
	}

	owner, packageSelector := selector.X.(*ast.Ident)
	if !packageSelector {
		return false
	}

	prefix := catalogPrefix(state.imports[owner.Name])
	left, leftOK := state.models["domain:"+expected].(*ast.BasicLit)
	right, rightOK := state.models[prefix+"domain:"+name.Name].(*ast.BasicLit)

	return leftOK && rightOK && left.Value == right.Value
}
func (state *wireValueAudit) enumExpressionsCompatible(values []ast.Expr,
	expected string, visited map[wireBinding]bool) bool {
	for _, value := range values {
		if !state.compatibleEnum(value, expected, visited) {
			return false
		}
	}

	return true
}
func (state *wireValueAudit) enumIdentifierCompatible(value *ast.Ident,
	expected string, visited map[wireBinding]bool) bool {
	if value.Obj == nil {
		return true
	}

	key := bindingOf(value)
	if visited[key] {
		return true
	}

	visited[key] = true
	if !state.enumMutationsCompatible(value, expected, visited) {
		return false
	}

	if argument, supplied := state.callArguments[key]; supplied {
		return state.compatibleEnum(argument, expected, visited)
	}

	switch declaration := value.Obj.Decl.(type) {
	case *ast.ValueSpec:
		return state.enumExpressionsCompatible(declaration.Values, expected, visited)
	case *ast.AssignStmt:
		return state.enumAssignmentCompatible(value, declaration, expected, visited)
	}

	return true
}

func (state *wireValueAudit) enumMutationsCompatible(identifier *ast.Ident,
	expected string, visited map[wireBinding]bool) bool {
	compatible := true

	ast.Inspect(state.file, func(node ast.Node) bool {
		assignment, isAssignment := node.(*ast.AssignStmt)
		if !isAssignment || assignment.Tok != token.ASSIGN {
			return true
		}

		for position, target := range assignment.Lhs {
			name, isIdentifier := target.(*ast.Ident)
			if isIdentifier && name.Obj == identifier.Obj && position < len(assignment.Rhs) {
				compatible = compatible && state.compatibleEnum(assignment.Rhs[position], expected, visited)
			}
		}

		return compatible
	})

	return compatible
}
func (state *wireValueAudit) enumAssignmentCompatible(identifier *ast.Ident, assignment *ast.AssignStmt,
	expected string, visited map[wireBinding]bool) bool {
	if len(assignment.Rhs) == 1 {
		call, isCall := assignment.Rhs[0].(*ast.CallExpr)
		if isCall {
			function := localCallable(call.Fun, map[wireBinding]bool{})

			for slot, target := range assignment.Lhs {
				value, isIdentifier := target.(*ast.Ident)
				if isIdentifier && value.Obj == identifier.Obj && function.signature != nil {
					return state.enumCallCompatible(call, function, expected, visited, slot)
				}
			}
		}
	}

	return state.enumExpressionsCompatible(assignment.Rhs, expected, visited)
}
func (state *wireValueAudit) enumCallCompatible(call *ast.CallExpr, function wireCallable,
	expected string, visited map[wireBinding]bool, slot int) bool {
	visited = maps.Clone(visited)

	key := wireBinding{declaration: function.signature, name: "enum callable"}

	if visited[key] {
		return false
	}

	visited[key] = true
	defer delete(visited, key)

	previous := state.callArguments

	state.callArguments = copyArguments(previous)

	defer func() { state.callArguments = previous }()

	position := 0

	for _, parameter := range function.signature.Params.List {
		for _, name := range parameter.Names {
			if position < len(call.Args) {
				state.callArguments[bindingOf(name)] = call.Args[position]
			}

			position++
		}
	}

	return state.enumReturnsCompatible(function.signature, function.body, expected, visited, slot)
}

func (state *wireValueAudit) enumReturnsCompatible(signature *ast.FuncType, body *ast.BlockStmt,
	expected string, visited map[wireBinding]bool, slot int) bool {
	position := 0

	if signature.Results != nil {
		for _, result := range signature.Results.List {
			for _, name := range result.Names {
				if position == slot && !state.compatibleEnum(name, expected, visited) {
					return false
				}

				position++
			}
		}
	}

	compatible := true

	ast.Inspect(body, func(node ast.Node) bool {
		statement, isReturn := node.(*ast.ReturnStmt)
		if isReturn && slot < len(statement.Results) {
			compatible = compatible && state.compatibleEnum(statement.Results[slot], expected, visited)
		}

		return compatible
	})

	return compatible
}
