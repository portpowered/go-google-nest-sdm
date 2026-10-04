package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

const callbackListenerFlow = `package main
func loginAccount() {
 redirect, err := loginRedirect(redirectURI)
 dependencies := app.login
 if dependencies.listen == nil || dependencies.openBrowser == nil { dependencies = defaultLoginDependencies() }
 address := redirect.Host
 if redirect.Hostname() == "localhost" { address = net.JoinHostPort("127.0.0.1", redirect.Port()) }
 listener, err := dependencies.listen(ctx, address)
 result, err := app.receiveAuthorization(ctx, session, redirect, listener, dependencies.openBrowser)
}`

func verifyCLIFlow(function *ast.FuncDecl) error {
	if function.Name.Name == receiveHelper {
		return verifyBrowserLaunch(function)
	}

	expected, err := parser.ParseFile(token.NewFileSet(), "callback-listener.go", callbackListenerFlow, 0)
	if err != nil {
		return fmt.Errorf("parse listener ownership inventory: %w", err)
	}

	declaration, recognized := expected.Decls[0].(*ast.FuncDecl)
	if !recognized {
		return fmt.Errorf("%w: listener flow inventory missing", errRouteInvalid)
	}

	owned := declaration.Body.List
	actual := []ast.Stmt{}

	for _, statement := range function.Body.List {
		if referencesListenerFlow(statement) {
			actual = append(actual, statement)
		}
	}

	if len(actual) != len(owned) {
		return fmt.Errorf("%w: callback listener ownership flow changed", errRouteInvalid)
	}

	for index, statement := range actual {
		if !sameSyntax(statement, owned[index]) {
			return fmt.Errorf("%w: callback listener origin or injection flow changed", errRouteInvalid)
		}
	}

	return nil
}

func referencesListenerFlow(node ast.Node) bool {
	uses := false
	parents := nodeParents(node)
	ast.Inspect(node, func(current ast.Node) bool {
		identifier, recognized := current.(*ast.Ident)
		if recognized {
			if selector, selected := parents[identifier].(*ast.SelectorExpr); selected && selector.Sel == identifier {
				return true
			}

			switch identifier.Name {
			case "redirect", "dependencies", "address", "listener":
				uses = true
			}
		}

		return true
	})

	return uses
}

func verifyBrowserLaunch(function *ast.FuncDecl) error {
	var failure error

	count := 0
	parents := nodeParents(function.Body)
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if identifier, recognized := node.(*ast.Ident); recognized && identifier.Name == launcherParameter {
			call, direct := parents[identifier].(*ast.CallExpr)
			if !direct || call.Fun != identifier || !isParameter(function, identifier) {
				failure = fmt.Errorf("%w: browser launcher aliases or escapes", errRouteInvalid)
			}
		}

		if browserInputMutation(node) {
			failure = fmt.Errorf("%w: authorization browser/session input mutated", errRouteInvalid)
		}

		call, recognized := node.(*ast.CallExpr)
		if !recognized {
			return true
		}

		if invalidCallbackServe(call, function) {
			failure = fmt.Errorf("%w: callback server does not consume the owned inbound listener", errRouteInvalid)
		}

		identifier, direct := call.Fun.(*ast.Ident)
		if !direct || identifier.Name != launcherParameter {
			return true
		}

		count++

		if len(call.Args) != 2 || !isParameter(function, identifier) || !browserSessionURL(call.Args[1], function) {
			failure = fmt.Errorf("%w: browser target is not the caller-owned SDK consent URL", errRouteInvalid)
		}

		return true
	})

	if failure != nil {
		return failure
	}

	if count != 1 {
		return fmt.Errorf("%w: browser launch boundary absent or duplicated", errRouteInvalid)
	}

	return nil
}

func browserInputMutation(node ast.Node) bool {
	assignment, recognized := node.(*ast.AssignStmt)
	if !recognized {
		return false
	}

	for _, target := range assignment.Lhs {
		identifier, direct := target.(*ast.Ident)
		if direct && (identifier.Name == "session" || identifier.Name == launcherParameter || identifier.Name == "listener") {
			return true
		}
	}

	return false
}

func invalidCallbackServe(call *ast.CallExpr, function *ast.FuncDecl) bool {
	selector, selected := call.Fun.(*ast.SelectorExpr)
	if !selected || selector.Sel.Name != serveMethod {
		return false
	}

	owner, direct := selector.X.(*ast.Ident)

	return !direct || owner.Obj == nil || len(call.Args) != 1 ||
		!parameterNamed(call.Args[0], function, "listener") || !serverDeclaration(owner)
}

func parameterNamed(expression ast.Expr, function *ast.FuncDecl, name string) bool {
	identifier, recognized := expression.(*ast.Ident)

	return recognized && identifier.Name == name && isParameter(function, identifier)
}

func serverDeclaration(identifier *ast.Ident) bool {
	assignment, recognized := identifier.Obj.Decl.(*ast.AssignStmt)
	if !recognized || len(assignment.Rhs) != 1 {
		return false
	}

	call, recognized := assignment.Rhs[0].(*ast.CallExpr)
	if !recognized || len(call.Args) != 1 {
		return false
	}

	constructor, recognized := call.Fun.(*ast.Ident)
	if !recognized || constructor.Name != "new" || constructor.Obj != nil {
		return false
	}

	kind, recognized := call.Args[0].(*ast.SelectorExpr)
	if !recognized || kind.Sel.Name != "Server" {
		return false
	}

	owner, recognized := kind.X.(*ast.Ident)

	return recognized && owner.Name == "http" && owner.Obj == nil
}

func browserSessionURL(expression ast.Expr, function *ast.FuncDecl) bool {
	call, recognized := expression.(*ast.CallExpr)
	if !recognized || len(call.Args) != 0 {
		return false
	}

	selector, recognized := call.Fun.(*ast.SelectorExpr)
	if !recognized || selector.Sel.Name != "AuthorizationURL" {
		return false
	}

	owner, recognized := selector.X.(*ast.Ident)

	return recognized && owner.Name == "session" && isParameter(function, owner)
}
