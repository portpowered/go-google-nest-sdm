package main

import (
	"go/ast"
	"go/token"
	"strings"
)

type wireAccess struct {
	root    wireBinding
	members []string
	valid   bool
}

func (state *wireValueAudit) fixedSelectorMutation(selector *ast.SelectorExpr,
	visited map[wireBinding]bool) bool {
	access := accessPath(selector, map[wireBinding]bool{})
	if !access.valid {
		return true
	}

	key := wireBinding{declaration: access.root.declaration,
		name: "access:" + access.root.name + ":" + strings.Join(access.members, ".")}
	if visited[key] {
		return false
	}

	visited[key] = true
	defer delete(visited, key)

	fixed := false

	ast.Inspect(state.file, func(node ast.Node) bool {
		assignment, recognized := node.(*ast.AssignStmt)
		if !recognized {
			return true
		}

		for index, left := range assignment.Lhs {
			if identifier, direct := left.(*ast.Ident); direct && bindingOf(identifier) != access.root {
				continue
			}

			candidate := accessPath(left, map[wireBinding]bool{})
			if !overlappingAccess(access, candidate) || index >= len(assignment.Rhs) {
				continue
			}

			if state.fixedValue(assignment.Rhs[index], visited) {
				fixed = true
			}
		}

		return true
	})

	return fixed
}

func accessPath(expression ast.Expr, visited map[wireBinding]bool) wireAccess {
	switch value := expression.(type) {
	case *ast.Ident:
		key := bindingOf(value)
		if key.declaration == nil || visited[key] {
			return wireAccess{root: key, members: nil, valid: false}
		}

		visited[key] = true
		if alias := pointerAlias(value); alias != nil {
			return accessPath(alias, visited)
		}

		return wireAccess{root: key, members: nil, valid: true}
	case *ast.SelectorExpr:
		access := accessPath(value.X, visited)
		access.members = append(access.members, value.Sel.Name)

		return access
	case *ast.ParenExpr:
		return accessPath(value.X, visited)
	case *ast.StarExpr:
		return accessPath(value.X, visited)
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			return accessPath(value.X, visited)
		}
	case *ast.IndexExpr:
		access := accessPath(value.X, visited)
		access.members = append(access.members, "[]")

		return access
	}

	return wireAccess{root: wireBinding{declaration: nil, name: ""}, members: nil, valid: false}
}

func pointerAlias(identifier *ast.Ident) ast.Expr {
	if identifier.Obj == nil {
		return nil
	}

	assignment, recognized := identifier.Obj.Decl.(*ast.AssignStmt)
	if !recognized {
		return nil
	}

	for index, left := range assignment.Lhs {
		name, recognized := left.(*ast.Ident)
		if !recognized || name.Obj != identifier.Obj || index >= len(assignment.Rhs) {
			continue
		}

		address, recognized := assignment.Rhs[index].(*ast.UnaryExpr)
		if recognized && address.Op == token.AND {
			return address.X
		}
	}

	return nil
}

func overlappingAccess(left, right wireAccess) bool {
	if !left.valid || !right.valid || left.root != right.root {
		return false
	}

	leftPath := strings.Join(left.members, ".")
	rightPath := strings.Join(right.members, ".")

	return leftPath == rightPath || rightPath == "" || leftPath == "" ||
		strings.HasPrefix(leftPath, rightPath+".") || strings.HasPrefix(rightPath, leftPath+".")
}
