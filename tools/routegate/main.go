// Command routegate rejects network edges without generated route provenance.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const module = "github.com/portpowered/go-google-nest-sdm"

func main() {
	err := audit(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	_, _ = fmt.Fprintln(os.Stdout, "route/source gate passed")
}

func audit(root string) error {
	err := auditHelpers(root)
	if err != nil {
		return err
	}

	err = auditProtocol(root)
	if err != nil {
		return err
	}

	err = filepath.WalkDir(filepath.Join(root, "pkg"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse source: %w", err)
		}

		if strings.HasSuffix(path, ".gen.go") {
			return nil
		}

		err = auditFile(file, filepath.ToSlash(path))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk sources: %w", err)
	}

	return nil
}

func auditFile(file *ast.File, path string) error {
	imports, err := auditImports(file)
	if err != nil {
		return err
	}

	for _, declaration := range file.Decls {
		group, isGroup := declaration.(*ast.GenDecl)
		if isGroup {
			err = auditGlobal(group, imports)
			if err != nil {
				return err
			}
		}

		function, recognized := declaration.(*ast.FuncDecl)
		if !recognized || function.Body == nil {
			continue
		}

		err = auditFunction(function, imports, path)
		if err != nil {
			return err
		}
	}

	return nil
}

func auditImports(file *ast.File) (map[string]string, error) {
	imports := make(map[string]string)

	for _, imported := range file.Imports {
		value, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("unquote import: %w", err)
		}

		name := filepath.Base(value)
		if imported.Name != nil {
			name = imported.Name.Name
		}

		imports[name] = value
		rawSocket := value == "net" || value == "crypto/tls"

		extraTransport := strings.Contains(value, "websocket") || strings.Contains(value, "cloud.google.com")

		if rawSocket || extraTransport {
			return nil, fmt.Errorf("%w: uninventoried network import %s", errRouteInvalid, value)
		}
	}

	return imports, nil
}

func verifiedHelper(function *ast.FuncDecl, path string) bool {
	switch function.Name.Name {
	case exchangeHelper:
		return strings.HasSuffix(path, "pkg/dependencies/httptransport/exchange.go")
	case downloadHelper:
		return strings.HasSuffix(path, "pkg/dependencies/media/download.go")
	default:
		return false
	}
}

func auditFunction(function *ast.FuncDecl, imports map[string]string, path string) error {
	var failure error

	ast.Inspect(function.Body, func(node ast.Node) bool {
		if failure != nil {
			return false
		}

		selector, recognized := node.(*ast.SelectorExpr)
		if recognized {
			failure = auditSelector(function, selector, imports, path)
		}

		return true
	})

	if failure != nil {
		return failure
	}

	if verifiedHelper(function, path) {
		return verifyExchange(function, imports)
	}

	return nil
}

func auditSelector(function *ast.FuncDecl, selector *ast.SelectorExpr, imports map[string]string, path string) error {
	owner, ownerOK := selector.X.(*ast.Ident)
	if ownerOK && imports[owner.Name] == httpImport && networkPrimitive(selector.Sel.Name) {
		if !verifiedHelper(function, path) || selector.Sel.Name != requestConstructor {
			return fmt.Errorf("%w: uninventoried HTTP primitive %s", errRouteInvalid, selector.Sel.Name)
		}
	}

	switch selector.Sel.Name {
	case exchangeHelper:
		call := findCall(function.Body, selector)
		if call == nil {
			return fmt.Errorf("%w: transport helper method value escapes", errRouteInvalid)
		}

		return verifyRoute(function, call, imports)
	case downloadHelper:
		call := findCall(function.Body, selector)
		if !strings.HasSuffix(path, "pkg/dependencies/media/download.go") || call == nil {
			return fmt.Errorf("%w: uninventoried media helper escape", errRouteInvalid)
		}

		return verifyMediaRoute(function, call, imports)
	case resourceHelper:
		if strings.HasSuffix(path, "pkg/dependencies/httptransport/sdk_resources.go") {
			return verifyResourceCall(findCall(function.Body, selector), imports)
		}
	case "Do", "RoundTrip":
		if !verifiedHelper(function, path) {
			return fmt.Errorf("%w: uninventoried HTTP send", errRouteInvalid)
		}
	}

	return nil
}

func verifyResourceCall(call *ast.CallExpr, imports map[string]string) error {
	if call == nil || len(call.Args) < 6 {
		return fmt.Errorf("%w: unrecognized resource helper escape", errRouteInvalid)
	}

	method, methodOK := generated(call.Args[4], imports, "Method")

	route, routeOK := generated(call.Args[5], imports, "Path")

	if !methodOK || !routeOK || method != route {
		return fmt.Errorf("%w: resource helper method/path differ", errRouteInvalid)
	}

	return nil
}

func networkPrimitive(name string) bool {
	switch name {
	case "Get", "Post", "PostForm", "Head", "NewRequest", requestConstructor, "Do", "RoundTrip":
		return true
	default:
		return false
	}
}

func findCall(body *ast.BlockStmt, selector *ast.SelectorExpr) *ast.CallExpr {
	var result *ast.CallExpr

	ast.Inspect(body, func(node ast.Node) bool {
		if call, recognized := node.(*ast.CallExpr); recognized && call.Fun == selector {
			result = call
		}

		return true
	})

	return result
}

func generated(expression ast.Expr, imports map[string]string, prefix string) (string, bool) {
	selector, recognized := expression.(*ast.SelectorExpr)
	if !recognized {
		return "", false
	}

	owner, recognized := selector.X.(*ast.Ident)
	if !recognized || owner.Obj != nil || imports[owner.Name] != module+"/internal/protocol" ||
		!strings.HasPrefix(selector.Sel.Name, prefix) {
		return "", false
	}

	if (prefix == "Method" || prefix == "Path") && !knownOperation(strings.TrimPrefix(selector.Sel.Name, prefix)) {
		return "", false
	}

	return strings.TrimPrefix(selector.Sel.Name, prefix), true
}

func receiver(function *ast.FuncDecl) *ast.Ident {
	if function.Recv == nil || len(function.Recv.List) != 1 || len(function.Recv.List[0].Names) != 1 {
		return nil
	}

	return function.Recv.List[0].Names[0]
}

func isParameter(function *ast.FuncDecl, identifier *ast.Ident) bool {
	for _, field := range function.Type.Params.List {
		for _, name := range field.Names {
			if name.Obj == identifier.Obj {
				return true
			}
		}
	}

	return false
}

func receiverField(expression ast.Expr, function *ast.FuncDecl, fields ...string) bool {
	for _, field := range slices.Backward(fields) {
		selector, recognized := expression.(*ast.SelectorExpr)
		if !recognized || selector.Sel.Name != field {
			return false
		}

		expression = selector.X
	}

	identifier, recognized := expression.(*ast.Ident)
	actual := receiver(function)

	return recognized && actual != nil && identifier.Obj == actual.Obj
}

func verifyRoute(function *ast.FuncDecl, call *ast.CallExpr, imports map[string]string) error {
	if len(call.Args) != exchangeArguments {
		return fmt.Errorf("%w: unrecognized exchange signature", errRouteInvalid)
	}

	method, recognized := routeMethod(call.Args[2], function, imports, "Method", "method")
	if !recognized {
		return fmt.Errorf("%w: HTTP method is not schema-generated", errRouteInvalid)
	}

	selector, recognized := call.Fun.(*ast.SelectorExpr)
	if !recognized {
		return fmt.Errorf("%w: exchange is not a receiver method", errRouteInvalid)
	}

	if !sameReceiver(selector.X, function) && !receiverField(selector.X, function, "transport") {
		return fmt.Errorf("%w: exchange receiver is shadowed", errRouteInvalid)
	}

	endpoint, recognized := call.Args[3].(*ast.BinaryExpr)
	if !recognized || endpoint.Op != token.ADD || !configuredOrigin(endpoint.X, function) {
		return fmt.Errorf("%w: endpoint origin is not actual configured receiver", errRouteInvalid)
	}

	if pathOperation, valid := generated(endpoint.Y, imports, "Path"); valid {
		if pathOperation != method {
			return fmt.Errorf("%w: method/path pair differs", errRouteInvalid)
		}

		return nil
	}

	path, recognized := endpoint.Y.(*ast.Ident)
	if !recognized || path.Obj == nil {
		return fmt.Errorf("%w: unresolved or transformed endpoint path", errRouteInvalid)
	}

	return verifyPath(function, call, path, method, imports)
}

func routeMethod(expression ast.Expr, function *ast.FuncDecl, imports map[string]string,
	prefix, name string) (string, bool) {
	operation, recognized := generated(expression, imports, prefix)
	if recognized {
		return operation, true
	}

	identifier, recognized := expression.(*ast.Ident)
	if function.Name.Name == resourceHelper && recognized && identifier.Name == name && isParameter(function, identifier) {
		return resourceHelper, true
	}

	return "", false
}

func configuredOrigin(expression ast.Expr, function *ast.FuncDecl) bool {
	for _, name := range []string{"sdmBaseURL", "pubSubBaseURL", "oauthBaseURL"} {
		if receiverField(expression, function, name) {
			return true
		}
	}

	return receiverField(expression, function, "transport", "sdmBaseURL")
}

func verifyPath(function *ast.FuncDecl, call *ast.CallExpr,
	path *ast.Ident, method string, imports map[string]string) error {
	assignments := 0
	valid := true

	ast.Inspect(function.Body, func(node ast.Node) bool {
		if assignment, recognized := node.(*ast.AssignStmt); recognized {
			count, acceptable := pathAssignment(assignment, function, call, path, method, imports)
			assignments += count
			valid = valid && acceptable
		}

		if escapes(node, path, call) {
			valid = false
		}

		return true
	})

	if assignments != 1 || !valid {
		return fmt.Errorf("%w: path has mutations, conditional or unverified provenance", errRouteInvalid)
	}

	if !topLevelAssignment(function, path) {
		return fmt.Errorf("%w: route assignment is conditional", errRouteInvalid)
	}

	return nil
}

func pathAssignment(assignment *ast.AssignStmt, function *ast.FuncDecl, call *ast.CallExpr,
	path *ast.Ident, method string, imports map[string]string) (int, bool) {
	count := 0
	valid := true

	for index, left := range assignment.Lhs {
		identifier, recognized := left.(*ast.Ident)
		if !recognized || identifier.Obj != path.Obj {
			continue
		}

		if assignment.Tok == token.ADD_ASSIGN && index == 0 && len(assignment.Rhs) == 1 &&
			safeQueryAppend(assignment.Rhs[0], function, imports) {
			continue
		}

		count++

		if assignment.Pos() > call.Pos() || index >= len(assignment.Rhs) {
			valid = false

			continue
		}

		valid = valid && pathConstructor(assignment.Rhs[index], function, method, imports)
	}

	return count, valid
}

func pathConstructor(expression ast.Expr, function *ast.FuncDecl, method string, imports map[string]string) bool {
	constructor, recognized := expression.(*ast.CallExpr)
	if !recognized || len(constructor.Args) < 2 {
		return false
	}

	helper, recognized := constructor.Fun.(*ast.Ident)
	if !recognized || !packageFunction(helper, "resourcePath") {
		return false
	}

	route, recognized := routeMethod(constructor.Args[1], function, imports, "Path", "template")

	return recognized && route == method
}

func escapes(node ast.Node, identifier *ast.Ident, allowed *ast.CallExpr) bool {
	if unary, recognized := node.(*ast.UnaryExpr); recognized && unary.Op == token.AND {
		if value, recognized := unary.X.(*ast.Ident); recognized && value.Obj == identifier.Obj {
			return true
		}
	}

	if call, recognized := node.(*ast.CallExpr); recognized && call != allowed {
		for _, argument := range call.Args {
			if value, recognized := argument.(*ast.Ident); recognized && value.Obj == identifier.Obj {
				return true
			}
		}
	}

	return false
}

func topLevelAssignment(function *ast.FuncDecl, identifier *ast.Ident) bool {
	for _, statement := range function.Body.List {
		assignment, recognized := statement.(*ast.AssignStmt)
		if !recognized {
			continue
		}

		for _, left := range assignment.Lhs {
			if value, recognized := left.(*ast.Ident); recognized && value.Obj == identifier.Obj {
				return true
			}
		}
	}

	return false
}

func auditGlobal(group *ast.GenDecl, imports map[string]string) error {
	var failure error

	ast.Inspect(group, func(node ast.Node) bool {
		selector, recognized := node.(*ast.SelectorExpr)
		if !recognized {
			return true
		}

		owner, recognized := selector.X.(*ast.Ident)
		if recognized && imports[owner.Name] == httpImport && networkPrimitive(selector.Sel.Name) {
			failure = fmt.Errorf("%w: file-scope HTTP primitive %s", errRouteInvalid, selector.Sel.Name)
		}

		return true
	})

	return failure
}
