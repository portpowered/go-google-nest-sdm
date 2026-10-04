package main

import (
	"fmt"
	"go/ast"
)

const wireModelImport = module + "/pkg/dependencymodels"

type wireBinding struct {
	declaration ast.Node
	name        string
}

type wireReference struct {
	name        string
	owned       bool
	mapping     bool
	structure   *ast.StructType
	sequence    *ast.ArrayType
	association *ast.MapType
}

type wireValueAudit struct {
	file          *ast.File
	imports       map[string]string
	models        map[string]ast.Expr
	bindings      map[wireBinding]wireReference
	callArguments map[wireBinding]ast.Expr
	callers       map[wireBinding]bool
	parents       map[ast.Node]ast.Node
}

func auditWireValues(file *ast.File, imports map[string]string, models map[string]ast.Expr) error {
	state := wireValueAudit{
		file: file, imports: imports, models: models,
		bindings: map[wireBinding]wireReference{}, callArguments: map[wireBinding]ast.Expr{},
		callers: map[wireBinding]bool{}, parents: nodeParents(file),
	}
	state.collectCallers()
	state.collectProviderResults()
	state.resolveBindings()

	var failure error

	ast.Inspect(file, func(node ast.Node) bool {
		if failure != nil {
			return false
		}

		switch value := node.(type) {
		case *ast.CompositeLit:
			reference := state.compositeReference(value)
			if reference.owned && reference.name != "" && state.fixedComposite(value, map[wireBinding]bool{}) {
				failure = fmt.Errorf("%w: handwritten fixed value or key in generated wire object", errRouteInvalid)
			}
		case *ast.AssignStmt:
			failure = state.auditWireAssignment(value)
		case *ast.CallExpr:
			failure = state.auditWireCall(value)
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
			if recognized && name.Obj == identifier.Obj {
				if len(declaration.Rhs) == 1 {
					if call, called := declaration.Rhs[0].(*ast.CallExpr); called {
						return state.callReferenceAt(call, index)
					}
				}

				if index < len(declaration.Rhs) {
					return state.reference(declaration.Rhs[index])
				}

				return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil, association: nil}
			}
		}
	}

	return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil, association: nil}
}

func (state *wireValueAudit) indexedReference(names []*ast.Ident, values []ast.Expr,
	identifier *ast.Ident) wireReference {
	for index, name := range names {
		if name.Obj == identifier.Obj {
			if len(values) == 1 {
				if call, called := values[0].(*ast.CallExpr); called {
					return state.callReferenceAt(call, index)
				}
			}

			if index < len(values) {
				return state.reference(values[index])
			}

			return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil, association: nil}
		}
	}

	return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil, association: nil}
}

func (state *wireValueAudit) reference(expression ast.Expr) wireReference {
	switch value := expression.(type) {
	case *ast.StructType:
		return state.nativeStructReference(value)
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
		reference := state.compositeReference(value)
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
	case *ast.MapType:
		element := state.reference(value.Value)

		return wireReference{name: "", owned: element.owned, mapping: false,
			structure: nil, sequence: nil, association: value}
	case *ast.ArrayType:
		element := state.reference(value.Elt)

		return wireReference{name: "", owned: element.owned,
			mapping: false, structure: nil, sequence: value, association: nil}
	case *ast.IndexExpr:
		container := state.reference(value.X)
		if container.sequence != nil {
			return state.fieldReference(container.sequence.Elt)
		}

		if container.association != nil {
			return state.fieldReference(container.association.Value)
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

	return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil, association: nil}
}

func (state *wireValueAudit) selectorReference(selector *ast.SelectorExpr) wireReference {
	owner, recognized := selector.X.(*ast.Ident)
	if recognized && owner.Obj == nil &&
		(state.imports[owner.Name] == wireModelImport || state.imports[owner.Name] == module+"/pkg/sdm") {
		_, exists := state.models[selector.Sel.Name]
		if !exists && len(state.models) != 0 {
			return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil, association: nil}
		}

		return wireReference{name: selector.Sel.Name, owned: true, mapping: state.modelMap(selector.Sel.Name),
			structure: nil, sequence: nil, association: nil}
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

	return wireReference{name: "", owned: true, mapping: false, structure: nil, sequence: nil, association: nil}
}

func (state *wireValueAudit) fieldReference(expression ast.Expr) wireReference {
	switch value := expression.(type) {
	case *ast.SelectorExpr:
		reference := state.reference(value)
		if reference.owned {
			return reference
		}

		return wireReference{name: "", owned: true, mapping: false, structure: nil, sequence: nil, association: nil}
	case *ast.MapType:
		return wireReference{name: "", owned: true, mapping: true, structure: nil, sequence: nil, association: nil}
	case *ast.Ident:
		return wireReference{name: value.Name, owned: true, mapping: state.modelMap(value.Name),
			structure: nil, sequence: nil, association: nil}
	case *ast.StarExpr:
		return state.fieldReference(value.X)
	case *ast.ArrayType:
		return wireReference{name: "", owned: true,
			mapping: false, structure: nil, sequence: value, association: nil}
	default:
		return wireReference{name: "", owned: true, mapping: false, structure: nil, sequence: nil, association: nil}
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
	return state.callReferenceAt(call, 0)
}

func (state *wireValueAudit) callReferenceAt(call *ast.CallExpr, selected int) wireReference {
	if selector, recognized := call.Fun.(*ast.SelectorExpr); recognized {
		return state.selectorReference(selector)
	}

	function := localCallable(call.Fun, map[wireBinding]bool{})
	if function.signature == nil || function.signature.Results == nil || len(function.signature.Results.List) == 0 {
		return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil, association: nil}
	}

	field, _ := resultField(function.signature, selected)
	if field == nil {
		return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil, association: nil}
	}

	return state.reference(field.Type)
}

type wireCallable struct {
	signature *ast.FuncType
	body      *ast.BlockStmt
}

func localCallable(expression ast.Expr, visited map[wireBinding]bool) wireCallable {
	switch value := expression.(type) {
	case *ast.FuncLit:
		return wireCallable{signature: value.Type, body: value.Body}
	case *ast.ParenExpr:
		return localCallable(value.X, visited)
	case *ast.Ident:
		return identifierCallable(value, visited)
	}

	return wireCallable{signature: nil, body: nil}
}

func identifierCallable(identifier *ast.Ident, visited map[wireBinding]bool) wireCallable {
	if identifier.Obj == nil {
		return wireCallable{signature: nil, body: nil}
	}

	key := bindingOf(identifier)
	if visited[key] {
		return wireCallable{signature: nil, body: nil}
	}

	visited[key] = true

	switch declaration := identifier.Obj.Decl.(type) {
	case *ast.FuncDecl:
		return wireCallable{signature: declaration.Type, body: declaration.Body}
	case *ast.AssignStmt:
		for index, left := range declaration.Lhs {
			other, recognized := left.(*ast.Ident)
			if recognized && other.Obj == identifier.Obj && index < len(declaration.Rhs) {
				return localCallable(declaration.Rhs[index], visited)
			}
		}
	case *ast.ValueSpec:
		for index, name := range declaration.Names {
			if name.Obj == identifier.Obj && index < len(declaration.Values) {
				return localCallable(declaration.Values[index], visited)
			}
		}
	}

	return wireCallable{signature: nil, body: nil}
}

func (state *wireValueAudit) nativeStructReference(shape *ast.StructType) wireReference {
	owned := false
	for _, field := range shape.Fields.List {
		owned = owned || state.reference(field.Type).owned
	}

	return wireReference{name: "", owned: owned, mapping: false, structure: shape, sequence: nil, association: nil}
}
