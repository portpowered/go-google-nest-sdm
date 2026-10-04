package main

import (
	"go/ast"
	"go/token"
)

func safeQueryAppend(expression ast.Expr, function *ast.FuncDecl, imports map[string]string) bool {
	binary, recognized := queryUnparen(expression).(*ast.BinaryExpr)
	if !recognized || binary.Op != token.ADD {
		return false
	}

	literal, recognized := queryUnparen(binary.X).(*ast.BasicLit)
	if !recognized || literal.Value != `"?"` {
		return false
	}

	encode, recognized := queryUnparen(binary.Y).(*ast.CallExpr)
	if !recognized || len(encode.Args) != 0 {
		return false
	}

	selector, recognized := queryUnparen(encode.Fun).(*ast.SelectorExpr)
	if !recognized || selector.Sel.Name != queryEncoder {
		return false
	}

	query, recognized := queryUnparen(selector.X).(*ast.Ident)
	if !recognized || !queryLocal(query, function) {
		return false
	}

	aliases := queryAliases(function, queryBinding(query))
	parents := nodeParents(function.Body)
	valid := true

	ast.Inspect(function.Body, func(node ast.Node) bool {
		identifier, found := node.(*ast.Ident)
		if !found || !aliases[queryBinding(identifier)] {
			return true
		}

		valid = valid && queryUse(identifier, function, parents, aliases, encode, imports)

		return true
	})

	return valid
}

func queryUnparen(expression ast.Expr) ast.Expr {
	for {
		parenthesized, recognized := expression.(*ast.ParenExpr)
		if !recognized {
			return expression
		}

		expression = parenthesized.X
	}
}

func queryLocal(identifier *ast.Ident, function *ast.FuncDecl) bool {
	if identifier.Obj == nil || identifier.Obj.Decl == nil {
		return false
	}

	declaration, recognized := identifier.Obj.Decl.(ast.Node)

	return recognized && declaration.Pos() >= function.Body.Pos() && declaration.End() <= function.Body.End()
}

func queryAliases(function *ast.FuncDecl, object ast.Node) map[ast.Node]bool {
	aliases := map[ast.Node]bool{object: true}

	changed := true
	for changed {
		changed = false

		ast.Inspect(function.Body, func(node ast.Node) bool {
			left, right := queryAliasPair(node)
			if left == nil || right == nil || !queryLocal(left, function) || !queryLocal(right, function) {
				return true
			}

			if aliases[queryBinding(left)] && !aliases[queryBinding(right)] {
				aliases[queryBinding(right)] = true
				changed = true
			}

			if aliases[queryBinding(right)] && !aliases[queryBinding(left)] {
				aliases[queryBinding(left)] = true
				changed = true
			}

			return true
		})
	}

	return aliases
}

func queryAliasPair(node ast.Node) (*ast.Ident, *ast.Ident) {
	var left, right ast.Expr

	switch declaration := node.(type) {
	case *ast.AssignStmt:
		if len(declaration.Lhs) != 1 || len(declaration.Rhs) != 1 {
			return nil, nil
		}

		left, right = declaration.Lhs[0], declaration.Rhs[0]
	case *ast.ValueSpec:
		if len(declaration.Names) != 1 || len(declaration.Values) != 1 {
			return nil, nil
		}

		left, right = declaration.Names[0], declaration.Values[0]
	default:
		return nil, nil
	}

	first, _ := queryUnparen(left).(*ast.Ident)
	second, _ := queryUnparen(right).(*ast.Ident)

	return first, second
}

func queryUse(identifier *ast.Ident, function *ast.FuncDecl, parents map[ast.Node]ast.Node,
	aliases map[ast.Node]bool, encode *ast.CallExpr, imports map[string]string) bool {
	var node ast.Node = identifier

	for {
		wrapper, recognized := parents[node].(*ast.ParenExpr)
		if !recognized {
			break
		}

		node = wrapper
	}

	switch parent := parents[node].(type) {
	case *ast.AssignStmt:
		if len(parent.Lhs) != 1 || len(parent.Rhs) != 1 {
			return false
		}

		left, recognized := queryUnparen(parent.Lhs[0]).(*ast.Ident)
		if !recognized || !queryLocal(left, function) || !aliases[queryBinding(left)] {
			return false
		}

		return queryValue(parent.Rhs[0], aliases, imports)
	case *ast.ValueSpec:
		return len(parent.Names) == 1 && len(parent.Values) == 1 && queryLocal(parent.Names[0], function) &&
			aliases[queryBinding(parent.Names[0])] && queryValue(parent.Values[0], aliases, imports)
	case *ast.SelectorExpr:
		call, recognized := parents[parent].(*ast.CallExpr)

		return recognized && queryCall(call, parent, encode, imports)
	case *ast.IndexExpr:
		return queryIndexWrite(parent, parents, imports)
	default:
		return false
	}
}

func queryValue(expression ast.Expr, aliases map[ast.Node]bool, imports map[string]string) bool {
	expression = queryUnparen(expression)
	if identifier, recognized := expression.(*ast.Ident); recognized {
		return aliases[queryBinding(identifier)]
	}

	composite, recognized := expression.(*ast.CompositeLit)
	if !recognized {
		return false
	}

	kind, recognized := queryUnparen(composite.Type).(*ast.SelectorExpr)
	if !recognized || kind.Sel.Name != "Values" {
		return false
	}

	owner, recognized := queryUnparen(kind.X).(*ast.Ident)
	if !recognized || owner.Obj != nil || imports[owner.Name] != urlImport {
		return false
	}

	for _, element := range composite.Elts {
		field, recognized := element.(*ast.KeyValueExpr)
		if !recognized {
			return false
		}

		if _, recognized = generated(queryUnparen(field.Key), imports, "Query"); !recognized {
			return false
		}
	}

	return true
}

func queryIndexWrite(index *ast.IndexExpr, parents map[ast.Node]ast.Node, imports map[string]string) bool {
	var node ast.Node = index

	for {
		wrapper, recognized := parents[node].(*ast.ParenExpr)
		if !recognized {
			break
		}

		node = wrapper
	}

	assignment, recognized := parents[node].(*ast.AssignStmt)
	if !recognized || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || assignment.Lhs[0] != node {
		return false
	}

	_, recognized = generated(queryUnparen(index.Index), imports, "Query")

	return recognized
}

func queryCall(call *ast.CallExpr, method *ast.SelectorExpr, encode *ast.CallExpr, imports map[string]string) bool {
	switch method.Sel.Name {
	case "Set", "Add":
		if len(call.Args) != keyValueArguments {
			return false
		}

		_, recognized := generated(queryUnparen(call.Args[0]), imports, "Query")

		return recognized
	case queryEncoder:
		return call == encode
	default:
		return false
	}
}

func nodeParents(body ast.Node) map[ast.Node]ast.Node {
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

func queryBinding(identifier *ast.Ident) ast.Node {
	if identifier.Obj == nil {
		return nil
	}

	declaration, _ := identifier.Obj.Decl.(ast.Node)

	return declaration
}
