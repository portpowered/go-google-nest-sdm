package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
)

type jsonExchangeAudit struct {
	function   *ast.FuncDecl
	imports    map[string]string
	parameters map[string]*ast.Ident
	parents    map[ast.Node]ast.Node
	marshal    *ast.CallExpr
	reader     *ast.CallExpr
	readerName *ast.Ident
	send       *ast.CallExpr
	body       *ast.Ident
}

func auditJSONHelper(root string, models map[string]ast.Expr) error {
	path := filepath.Join(root, "pkg/dependencies/httptransport/exchange.go")

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return fmt.Errorf("parse JSON helper: %w", err)
	}

	imports, err := auditImports(file)
	if err != nil {
		return err
	}

	bindGeneratedConstants(imports, models)

	var function *ast.FuncDecl

	for _, declaration := range file.Decls {
		candidate, recognized := declaration.(*ast.FuncDecl)
		if recognized && candidate.Name.Name == jsonExchangeHelper {
			if function != nil {
				return fmt.Errorf("%w: ambiguous JSON exchange helper", errRouteInvalid)
			}

			function = candidate
		}
	}

	if function == nil {
		return fmt.Errorf("%w: missing JSON exchange helper", errRouteInvalid)
	}

	err = verifyJSONHelper(function, imports)
	if err != nil {
		return err
	}

	models["approved-helper:"+jsonExchangeHelper] = function.Type

	return nil
}

func verifyJSONHelper(function *ast.FuncDecl, imports map[string]string) error {
	state := jsonExchangeAudit{function: function, imports: imports, parameters: map[string]*ast.Ident{},
		parents: nodeParents(function.Body), marshal: nil, reader: nil, readerName: nil, send: nil, body: nil}

	for _, field := range function.Type.Params.List {
		for _, name := range field.Names {
			state.parameters[name.Name] = name
		}
	}

	if len(state.parameters) != exchangeArguments-1 || !requestClientReceiver(function) {
		return fmt.Errorf("%w: JSON helper signature or receiver changed", errRouteInvalid)
	}

	valid := true

	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, recognized := node.(*ast.CallExpr)
		if !recognized {
			return true
		}

		if _, recognized = importedCall(call, imports, "encoding/json", "Marshal"); recognized {
			valid = valid && state.marshal == nil && len(call.Args) == 1 && state.parameter(call.Args[0], "input")
			state.marshal = call
		}

		_, mutableReader := importedCall(call, imports, "bytes", "NewReader")

		_, immutableReader := importedCall(call, imports, "strings", "NewReader")

		if mutableReader || immutableReader {
			valid = valid && state.reader == nil
			state.reader = call
		}

		if selector, recognized := call.Fun.(*ast.SelectorExpr); recognized && selector.Sel.Name == exchangeHelper {
			valid = valid && state.send == nil && sameReceiver(selector.X, function)
			state.send = call
		}

		return true
	})

	if !valid || !state.bindBody() || !state.forwardedCall() || !state.safeUses() {
		return fmt.Errorf("%w: JSON helper does not forward original route and serialized input", errRouteInvalid)
	}

	return nil
}

func (state *jsonExchangeAudit) parameter(expression ast.Expr, name string) bool {
	identifier, recognized := expression.(*ast.Ident)
	parameter := state.parameters[name]

	return recognized && parameter != nil && identifier.Obj == parameter.Obj
}

func (state *jsonExchangeAudit) bindBody() bool {
	if state.marshal == nil || state.reader == nil || state.send == nil || len(state.reader.Args) != 1 {
		return false
	}

	assignment, recognized := state.parents[state.marshal].(*ast.AssignStmt)
	if !recognized || len(assignment.Lhs) != 2 || len(assignment.Rhs) != 1 || assignment.Tok != token.DEFINE {
		return false
	}

	body, recognized := assignment.Lhs[0].(*ast.Ident)
	if !recognized {
		return false
	}

	state.body = body
	input := state.readerBody()

	return input != nil && input.Obj == body.Obj && state.marshal.Pos() < state.send.Pos() &&
		state.parents[assignment] == state.function.Body
}

func (state *jsonExchangeAudit) readerBody() *ast.Ident {
	input := state.reader.Args[0]
	if _, immutable := importedCall(state.reader, state.imports, "strings", "NewReader"); immutable {
		conversion, recognized := input.(*ast.CallExpr)
		if !recognized || len(conversion.Args) != 1 {
			return nil
		}

		kind, recognized := conversion.Fun.(*ast.Ident)
		if !recognized || kind.Obj != nil || kind.Name != "string" {
			return nil
		}

		input = conversion.Args[0]
	}

	identifier, recognized := input.(*ast.Ident)
	if !recognized {
		return nil
	}

	return identifier
}

func (state *jsonExchangeAudit) forwardedCall() bool {
	if len(state.send.Args) != exchangeArguments || !state.forwardedReader(state.send.Args[6]) ||
		!state.parameter(state.send.Args[7], "result") {
		return false
	}

	for position, name := range []string{"ctx", "operation", "method", "endpoint", "token"} {
		if !state.parameter(state.send.Args[position], name) {
			return false
		}
	}

	selector, recognized := state.send.Args[5].(*ast.SelectorExpr)
	if !recognized || selector.Sel.Name != "MIMEApplicationJSON" {
		return false
	}

	owner, recognized := selector.X.(*ast.Ident)
	if !recognized || owner.Obj != nil || state.imports[owner.Name] != module+"/internal/protocol" ||
		state.imports[generatedBindingPrefix+selector.Sel.Name] == "" {
		return false
	}

	statement, recognized := state.parents[state.send].(*ast.ReturnStmt)

	return recognized && len(statement.Results) == 1 && statement.Results[0] == state.send &&
		state.parents[statement] == state.function.Body
}

func (state *jsonExchangeAudit) forwardedReader(expression ast.Expr) bool {
	if expression == state.reader {
		return true
	}

	identifier, recognized := expression.(*ast.Ident)
	if !recognized || identifier.Obj == nil {
		return false
	}

	assignment, recognized := identifier.Obj.Decl.(*ast.AssignStmt)
	if !recognized || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 ||
		assignment.Rhs[0] != state.reader || state.parents[assignment] != state.function.Body ||
		assignment.Pos() <= state.marshal.Pos() || assignment.End() >= state.send.Pos() {
		return false
	}

	state.readerName = identifier

	return true
}

func (state *jsonExchangeAudit) safeUses() bool {
	valid := true

	ast.Inspect(state.function.Body, func(node ast.Node) bool {
		identifier, recognized := node.(*ast.Ident)
		if !recognized {
			return true
		}

		parent := state.parents[identifier]
		valid = valid && state.safeReaderName(identifier, parent)

		if actual := receiver(state.function); actual != nil && identifier.Obj == actual.Obj {
			selector, isSelector := parent.(*ast.SelectorExpr)
			valid = valid && isSelector && state.send.Fun == selector
		}

		if identifier.Obj == state.body.Obj {
			valid = valid && (parent == state.parents[state.marshal] || parent == state.reader ||
				state.immutableConversion(parent))
		}

		for name, parameter := range state.parameters {
			if identifier.Obj != parameter.Obj {
				continue
			}

			allowed := parent == state.send || name == "input" && parent == state.marshal

			if name == "operation" {
				call, isCall := parent.(*ast.CallExpr)
				if isCall {
					callee, isIdentifier := call.Fun.(*ast.Ident)
					allowed = allowed || isIdentifier && callee.Name == "fail"
				}
			}

			valid = valid && allowed
		}

		return valid
	})

	return valid
}

func (state *jsonExchangeAudit) safeReaderName(identifier *ast.Ident, parent ast.Node) bool {
	if state.readerName == nil || identifier.Obj != state.readerName.Obj {
		return true
	}

	return parent == state.parents[state.reader] || parent == state.send
}

func (state *jsonExchangeAudit) immutableConversion(node ast.Node) bool {
	conversion, recognized := node.(*ast.CallExpr)
	if !recognized || state.parents[conversion] != state.reader {
		return false
	}

	_, immutable := importedCall(state.reader, state.imports, "strings", "NewReader")

	return immutable && state.readerBody() != nil
}
