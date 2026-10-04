package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"maps"
)

const wireModelImport = module + "/pkg/dependencymodels"

type wireBinding struct {
	declaration ast.Node
	name        string
}

type wireReference struct {
	name      string
	owned     bool
	mapping   bool
	structure *ast.StructType
	sequence  *ast.ArrayType
}

type wireValueAudit struct {
	file          *ast.File
	imports       map[string]string
	models        map[string]ast.Expr
	bindings      map[wireBinding]wireReference
	callArguments map[wireBinding]ast.Expr
}

func auditWireValues(file *ast.File, imports map[string]string, models map[string]ast.Expr) error {
	state := wireValueAudit{
		file: file, imports: imports, models: models,
		bindings: map[wireBinding]wireReference{}, callArguments: map[wireBinding]ast.Expr{},
	}
	state.resolveBindings()

	var failure error

	ast.Inspect(file, func(node ast.Node) bool {
		if failure != nil {
			return false
		}

		switch value := node.(type) {
		case *ast.CompositeLit:
			reference := state.reference(value.Type)
			if reference.owned && reference.structure == nil && state.fixedComposite(value, map[wireBinding]bool{}) {
				failure = fmt.Errorf("%w: handwritten fixed value or key in generated wire object", errRouteInvalid)
			}
		case *ast.AssignStmt:
			failure = state.auditWireAssignment(value)
		case *ast.CallExpr:
			failure = state.auditMapCall(value)
		}

		return true
	})

	return failure
}

func bindingOf(identifier *ast.Ident) wireBinding {
	if identifier.Obj == nil {
		return wireBinding{declaration: nil, name: identifier.Name}
	}

	declaration, recognized := identifier.Obj.Decl.(ast.Node)
	if !recognized {
		declaration = nil
	}

	return wireBinding{declaration: declaration, name: identifier.Name}
}

func (state *wireValueAudit) resolveBindings() {
	changed := true
	for changed {
		changed = false

		ast.Inspect(state.file, func(node ast.Node) bool {
			identifier, recognized := node.(*ast.Ident)
			if !recognized || identifier.Obj == nil {
				return true
			}

			key := bindingOf(identifier)
			if state.bindings[key].owned {
				return true
			}

			reference := state.declarationReference(identifier)
			if reference.owned {
				state.bindings[key] = reference
				changed = true
			}

			return true
		})
	}
}

func (state *wireValueAudit) declarationReference(identifier *ast.Ident) wireReference {
	switch declaration := identifier.Obj.Decl.(type) {
	case *ast.TypeSpec:
		return state.reference(declaration.Type)
	case *ast.Field:
		return state.reference(declaration.Type)
	case *ast.ValueSpec:
		reference := state.reference(declaration.Type)
		if reference.owned {
			return reference
		}

		return state.indexedReference(declaration.Names, declaration.Values, identifier)
	case *ast.AssignStmt:
		for index, left := range declaration.Lhs {
			name, recognized := left.(*ast.Ident)
			if recognized && name.Obj == identifier.Obj && index < len(declaration.Rhs) {
				return state.reference(declaration.Rhs[index])
			}
		}
	}

	return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil}
}

func (state *wireValueAudit) indexedReference(names []*ast.Ident, values []ast.Expr,
	identifier *ast.Ident) wireReference {
	for index, name := range names {
		if name.Obj == identifier.Obj && index < len(values) {
			return state.reference(values[index])
		}
	}

	return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil}
}

func (state *wireValueAudit) reference(expression ast.Expr) wireReference {
	switch value := expression.(type) {
	case *ast.StructType:
		return wireReference{name: "", owned: true, mapping: false, structure: value, sequence: nil}
	case *ast.Ident:
		return state.bindings[bindingOf(value)]
	case *ast.SelectorExpr:
		return state.selectorReference(value)
	case *ast.ParenExpr:
		return state.reference(value.X)
	case *ast.StarExpr:
		return state.reference(value.X)
	case *ast.UnaryExpr:
		return state.reference(value.X)
	case *ast.CompositeLit:
		reference := state.reference(value.Type)
		if reference.owned {
			return reference
		}

		for _, element := range value.Elts {
			if keyed, recognized := element.(*ast.KeyValueExpr); recognized {
				element = keyed.Value
			}

			nested := state.reference(element)
			if nested.mapping {
				return nested
			}
		}

		return reference
	case *ast.ArrayType:
		element := state.reference(value.Elt)

		return wireReference{name: "", owned: element.owned, mapping: false, structure: nil, sequence: value}
	case *ast.IndexExpr:
		container := state.reference(value.X)
		if container.sequence != nil {
			return state.fieldReference(container.sequence.Elt)
		}

		return container
	case *ast.TypeAssertExpr:
		typed := state.reference(value.Type)
		if typed.owned {
			return typed
		}

		return state.reference(value.X)
	case *ast.CallExpr:
		return state.callReference(value)
	}

	return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil}
}

func (state *wireValueAudit) selectorReference(selector *ast.SelectorExpr) wireReference {
	owner, recognized := selector.X.(*ast.Ident)
	if recognized && owner.Obj == nil && state.imports[owner.Name] == wireModelImport {
		_, exists := state.models[selector.Sel.Name]
		if !exists && len(state.models) != 0 {
			return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil}
		}

		return wireReference{name: selector.Sel.Name, owned: true, mapping: state.modelMap(selector.Sel.Name),
			structure: nil, sequence: nil}
	}

	reference := state.reference(selector.X)
	if !reference.owned {
		return reference
	}

	structure, recognized := state.models[reference.name].(*ast.StructType)
	if reference.structure != nil {
		structure = reference.structure
		recognized = true
	}

	if !recognized {
		return reference
	}

	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if name.Name == selector.Sel.Name {
				if reference.structure != nil {
					return state.reference(field.Type)
				}

				return state.fieldReference(field.Type)
			}
		}
	}

	return wireReference{name: "", owned: true, mapping: false, structure: nil, sequence: nil}
}

func (state *wireValueAudit) fieldReference(expression ast.Expr) wireReference {
	switch value := expression.(type) {
	case *ast.SelectorExpr:
		reference := state.reference(value)
		if reference.owned {
			return reference
		}

		return wireReference{name: "", owned: true, mapping: false, structure: nil, sequence: nil}
	case *ast.MapType:
		return wireReference{name: "", owned: true, mapping: true, structure: nil, sequence: nil}
	case *ast.Ident:
		return wireReference{name: value.Name, owned: true, mapping: state.modelMap(value.Name),
			structure: nil, sequence: nil}
	case *ast.StarExpr:
		return state.fieldReference(value.X)
	case *ast.ArrayType:
		return wireReference{name: "", owned: true, mapping: false, structure: nil, sequence: value}
	default:
		return wireReference{name: "", owned: true, mapping: false, structure: nil, sequence: nil}
	}
}

func (state *wireValueAudit) modelMap(name string) bool {
	if len(state.models) == 0 {
		return true
	}

	_, mapping := state.models[name].(*ast.MapType)

	return mapping
}

func (state *wireValueAudit) callReference(call *ast.CallExpr) wireReference {
	if selector, recognized := call.Fun.(*ast.SelectorExpr); recognized {
		return state.selectorReference(selector)
	}

	function := localFunction(call.Fun)
	if function == nil || function.Type.Results == nil || len(function.Type.Results.List) == 0 {
		return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil}
	}

	return state.reference(function.Type.Results.List[0].Type)
}

func localFunction(expression ast.Expr) *ast.FuncDecl {
	identifier, recognized := expression.(*ast.Ident)
	if !recognized || identifier.Obj == nil {
		return nil
	}

	function, _ := identifier.Obj.Decl.(*ast.FuncDecl)

	return function
}

func (state *wireValueAudit) auditWireAssignment(assignment *ast.AssignStmt) error {
	for index, target := range assignment.Lhs {
		if index >= len(assignment.Rhs) || !state.reference(target).owned {
			continue
		}
		// Definitions and aliases retain provenance; the initializer is checked at the wire boundary.
		if _, recognized := target.(*ast.Ident); recognized && assignment.Tok == token.DEFINE {
			continue
		}

		if state.fixedValue(assignment.Rhs[index], map[wireBinding]bool{}) {
			return fmt.Errorf("%w: handwritten fixed value assigned to generated wire field", errRouteInvalid)
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
			if _, fieldName := keyed.Key.(*ast.Ident); (mapping || !fieldName) && state.fixedValue(keyed.Key, visited) {
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

	return false
}

func (state *wireValueAudit) fixedIdentifier(identifier *ast.Ident, visited map[wireBinding]bool) bool {
	if identifier.Name == "true" || identifier.Name == "false" {
		return true
	}

	if identifier.Obj == nil {
		return false
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
		for index, name := range declaration.Names {
			if name.Obj == identifier.Obj && index < len(declaration.Values) {
				return state.fixedValue(declaration.Values[index], visited)
			}
		}
	}

	return state.fixedAssignments(identifier, visited)
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
			if recognized && other.Obj == identifier.Obj && index < len(assignment.Rhs) &&
				state.fixedValue(assignment.Rhs[index], visited) {
				fixed = true
			}
		}

		return true
	})

	return fixed
}

func (state *wireValueAudit) fixedCall(call *ast.CallExpr, visited map[wireBinding]bool) bool {
	if function := localFunction(call.Fun); function != nil {
		return state.fixedHelperCall(call, function, visited)
	}

	if callback, recognized := call.Fun.(*ast.CallExpr); recognized {
		return state.fixedCall(callback, visited)
	}

	for _, argument := range call.Args {
		if state.fixedValue(argument, visited) {
			return true
		}
	}

	return false
}

func (state *wireValueAudit) fixedHelperCall(call *ast.CallExpr, function *ast.FuncDecl,
	visited map[wireBinding]bool) bool {
	previous := state.callArguments
	state.callArguments = copyArguments(previous)
	index := 0

	for _, parameter := range function.Type.Params.List {
		for _, name := range parameter.Names {
			if index < len(call.Args) {
				state.callArguments[bindingOf(name)] = call.Args[index]
			}

			index++
		}
	}

	fixed := state.fixedFunction(function.Type, function.Body, visited)
	state.callArguments = previous

	return fixed
}

func copyArguments(source map[wireBinding]ast.Expr) map[wireBinding]ast.Expr {
	result := make(map[wireBinding]ast.Expr, len(source))
	maps.Copy(result, source)

	return result
}

func (state *wireValueAudit) fixedFunction(signature *ast.FuncType, body *ast.BlockStmt,
	visited map[wireBinding]bool) bool {
	fixed := false

	if signature.Results != nil && len(signature.Results.List) > 0 {
		for _, name := range signature.Results.List[0].Names {
			fixed = fixed || state.fixedIdentifier(name, visited)
		}
	}

	ast.Inspect(body, func(node ast.Node) bool {
		statement, recognized := node.(*ast.ReturnStmt)
		if !recognized || len(statement.Results) == 0 {
			return true
		}

		if state.fixedValue(statement.Results[0], visited) {
			fixed = true
		}

		return true
	})

	return fixed
}

func (state *wireValueAudit) auditMapCall(call *ast.CallExpr) error {
	for index, argument := range call.Args {
		if !state.containsWireMap(argument) {
			continue
		}

		if state.verifiedMapCall(call, index) {
			continue
		}

		return fmt.Errorf("%w: generated wire map escapes to an unverified helper", errRouteInvalid)
	}

	return nil
}

func (state *wireValueAudit) verifiedMapCall(call *ast.CallExpr, argumentIndex int) bool {
	if function := localFunction(call.Fun); function != nil {
		return state.typedMapParameter(function, argumentIndex)
	}

	selector, recognized := call.Fun.(*ast.SelectorExpr)
	if !recognized {
		return false
	}

	if selector.Sel.Name == exchangeHelper {
		return true
	}

	owner, recognized := selector.X.(*ast.Ident)

	return recognized && owner.Obj == nil && state.imports[owner.Name] == "encoding/json" &&
		(selector.Sel.Name == "Marshal" || selector.Sel.Name == "Unmarshal")
}

func (state *wireValueAudit) fixedSelector(selector *ast.SelectorExpr, visited map[wireBinding]bool) bool {
	owner, recognized := selector.X.(*ast.Ident)
	if recognized && owner.Obj == nil {
		imported := state.imports[owner.Name]
		if imported == wireModelImport && len(state.models) != 0 {
			_, generatedConstant := state.models["constant:"+selector.Sel.Name]

			return !generatedConstant
		}

		return imported != wireModelImport && imported != module+"/internal/protocol" && imported != module+"/pkg/sdm"
	}

	return state.fixedValue(selector.X, visited)
}

func (state *wireValueAudit) typedMapParameter(function *ast.FuncDecl, argumentIndex int) bool {
	index := 0

	for _, parameter := range function.Type.Params.List {
		for range parameter.Names {
			if index == argumentIndex {
				return state.reference(parameter.Type).owned
			}

			index++
		}
	}

	return false
}

func (state *wireValueAudit) fixedReceiverKey(expression ast.Expr) bool {
	fixed := false

	ast.Inspect(expression, func(node ast.Node) bool {
		indexed, recognized := node.(*ast.IndexExpr)
		if recognized && state.reference(indexed.X).mapping && state.fixedValue(indexed.Index, map[wireBinding]bool{}) {
			fixed = true
		}

		return true
	})

	return fixed
}

func (state *wireValueAudit) containsWireMap(expression ast.Expr) bool {
	found := false

	ast.Inspect(expression, func(node ast.Node) bool {
		value, recognized := node.(ast.Expr)
		if !recognized {
			return true
		}

		reference := state.reference(value)
		if reference.mapping {
			found = true
		}

		if reference.sequence != nil && state.fieldReference(reference.sequence.Elt).mapping {
			found = true
		}

		if reference.structure != nil {
			for _, field := range reference.structure.Fields.List {
				if state.reference(field.Type).mapping {
					found = true
				}
			}
		}

		return true
	})

	return found
}
