package main

import (
	"go/ast"
	"strings"
)

const localMethodPrefix = "local-method:"

func bindLocalMethods(file *ast.File, imports map[string]string) {
	for _, declaration := range file.Decls {
		function, recognized := declaration.(*ast.FuncDecl)
		if !recognized || function.Recv == nil || len(function.Recv.List) != 1 || function.Body == nil {
			continue
		}

		kind := receiverType(function)
		if kind != "" {
			imports[localMethodPrefix+kind+"."+function.Name.Name] = kind
		}
	}
}

func receiverType(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) != 1 {
		return ""
	}

	expression := function.Recv.List[0].Type
	if pointer, recognized := expression.(*ast.StarExpr); recognized {
		expression = pointer.X
	}

	kind, recognized := expression.(*ast.Ident)
	if recognized {
		return kind.Name
	}

	return ""
}

// A same-named private behavioral method is resolved by its actual receiver
// declaration. Its implementation is still audited; it acquires no wire trust.
func localBehaviorMethod(function *ast.FuncDecl, selector *ast.SelectorExpr,
	imports map[string]string, path string) bool {
	if selector.Sel.Name != exchangeHelper || !sameReceiver(selector.X, function) ||
		strings.HasPrefix(path, "pkg/dependencies/httptransport/") {
		return false
	}

	kind := receiverType(function)

	return kind != "" && imports[localMethodPrefix+kind+"."+selector.Sel.Name] == kind
}
