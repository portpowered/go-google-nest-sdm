package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

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
	if function := localCallable(call.Fun, map[wireBinding]bool{}); function.signature != nil {
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
		return state.generatedConstantFixed(selector)
	}

	if state.fixedSelectorMutation(selector, visited) {
		return true
	}

	return state.fixedValue(selector.X, visited)
}

func (state *wireValueAudit) typedMapParameter(function wireCallable, argumentIndex int) bool {
	index := 0

	for _, parameter := range function.signature.Params.List {
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
		if recognized && state.reference(indexed.X).mapping &&
			(state.fixedValue(indexed.Index, map[wireBinding]bool{}) ||
				!state.enumValueCompatible(indexed.Index, state.reference(indexed.X).name)) {
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

func (state *wireValueAudit) collectCallers() {
	ast.Inspect(state.file, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.FuncDecl:
			state.recordCallerFields(declaration.Type.Params)
			state.recordCallerFields(declaration.Recv)
		case *ast.FuncLit:
			state.recordCallerFields(declaration.Type.Params)
		}

		return true
	})
}

func (state *wireValueAudit) recordCallerFields(fields *ast.FieldList) {
	if fields == nil {
		return
	}

	for _, field := range fields.List {
		for _, name := range field.Names {
			state.callers[bindingOf(name)] = true
		}
	}
}

func (state *wireValueAudit) namedResult(identifier *ast.Ident) bool {
	_, field := identifier.Obj.Decl.(*ast.Field)

	return field && !state.callers[bindingOf(identifier)]
}

func (state *wireValueAudit) verifiedScalarCall(call *ast.CallExpr) bool {
	if identifier, recognized := call.Fun.(*ast.Ident); recognized && identifier.Obj == nil {
		return identifier.Name == "string" || identifier.Name == "int" ||
			identifier.Name == "float64" || identifier.Name == "bool"
	}

	if array, recognized := call.Fun.(*ast.ArrayType); recognized {
		element, recognized := array.Elt.(*ast.Ident)

		return recognized && element.Obj == nil && (element.Name == "byte" || element.Name == "rune")
	}

	selector, recognized := call.Fun.(*ast.SelectorExpr)
	if !recognized {
		return false
	}

	if state.verifiedProviderCall(call, selector) {
		return true
	}

	owner, recognized := selector.X.(*ast.Ident)
	if !recognized || owner.Obj != nil {
		return false
	}

	imported := state.imports[owner.Name]
	if imported == wireModelImport || imported == module+"/pkg/sdm" {
		_, registered := state.models[selector.Sel.Name]

		return registered
	}

	return imported == "encoding/json" && (selector.Sel.Name == "Marshal" || selector.Sel.Name == "RawMessage")
}

func (state *wireValueAudit) compositeReference(composite *ast.CompositeLit) wireReference {
	if composite.Type != nil {
		return state.reference(composite.Type)
	}

	parent := state.parents[composite]
	if keyed, recognized := parent.(*ast.KeyValueExpr); recognized {
		parent = state.parents[keyed]
	}

	outer, recognized := parent.(*ast.CompositeLit)
	if !recognized {
		return wireReference{name: "", owned: false, mapping: false,
			structure: nil, sequence: nil, association: nil}
	}

	switch shape := outer.Type.(type) {
	case *ast.MapType:
		return state.reference(shape.Value)
	case *ast.ArrayType:
		return state.reference(shape.Elt)
	}

	return wireReference{name: "", owned: false, mapping: false, structure: nil, sequence: nil, association: nil}
}

func (state *wireValueAudit) auditWireCall(call *ast.CallExpr) error {
	err := state.auditMapCall(call)
	if err != nil {
		return err
	}

	for index, argument := range call.Args {
		if !state.wireAddress(argument) {
			continue
		}

		if !state.verifiedMapCall(call, index) {
			return fmt.Errorf("%w: generated wire address escapes to an unverified helper", errRouteInvalid)
		}

		if state.jsonDecoder(call) && len(call.Args) > 0 && state.fixedValue(call.Args[0], map[wireBinding]bool{}) {
			return fmt.Errorf("%w: handwritten fixed JSON decoded into a generated wire object", errRouteInvalid)
		}
	}

	return nil
}

func (state *wireValueAudit) wireAddress(expression ast.Expr) bool {
	found := false

	ast.Inspect(expression, func(node ast.Node) bool {
		address, recognized := node.(*ast.UnaryExpr)
		if recognized && address.Op == token.AND && state.reference(address.X).owned {
			found = true
		}

		return true
	})

	return found
}

func (state *wireValueAudit) jsonDecoder(call *ast.CallExpr) bool {
	_, recognized := importedCall(call, state.imports, "encoding/json", "Unmarshal")

	return recognized
}

func (state *wireValueAudit) zeroAggregate(identifier *ast.Ident) bool {
	reference := state.reference(identifier)
	if !reference.owned {
		return false
	}

	if reference.mapping || reference.structure != nil || reference.sequence != nil || reference.association != nil {
		return true
	}

	_, structure := state.models[reference.name].(*ast.StructType)

	return structure
}

func (state *wireValueAudit) verifiedProviderCall(call *ast.CallExpr, selector *ast.SelectorExpr) bool {
	if !knownOperation(selector.Sel.Name) {
		return false
	}

	for parent := state.parents[call]; parent != nil; parent = state.parents[parent] {
		function, recognized := parent.(*ast.FuncDecl)
		if recognized {
			return receiverField(selector.X, function, "transport") || sameReceiver(selector.X, function)
		}
	}

	return false
}

func (state *wireValueAudit) collectProviderResults() {
	ast.Inspect(state.file, func(node ast.Node) bool {
		call, recognized := node.(*ast.CallExpr)
		if !recognized || len(call.Args) != exchangeArguments {
			return true
		}

		selector, recognized := call.Fun.(*ast.SelectorExpr)
		if !recognized || selector.Sel.Name != exchangeHelper {
			return true
		}

		for parent := state.parents[call]; parent != nil; parent = state.parents[parent] {
			function, recognized := parent.(*ast.FuncDecl)
			if !recognized {
				continue
			}

			if verifyRoute(function, call, state.imports) != nil {
				return true
			}

			address, recognized := call.Args[len(call.Args)-1].(*ast.UnaryExpr)
			if !recognized || address.Op != token.AND {
				return true
			}

			identifier, recognized := address.X.(*ast.Ident)
			if recognized {
				state.callers[bindingOf(identifier)] = true
			}

			return true
		}

		return true
	})
}

func (state *wireValueAudit) compositeMemberDomain(composite *ast.CompositeLit, member *ast.KeyValueExpr) bool {
	reference := state.compositeReference(composite)

	structure, recognized := state.models[reference.name].(*ast.StructType)
	if !recognized {
		return true
	}

	name, recognized := member.Key.(*ast.Ident)
	if !recognized {
		return true
	}

	for _, field := range structure.Fields.List {
		for _, candidate := range field.Names {
			if candidate.Name != name.Name {
				continue
			}

			expected := state.fieldReference(field.Type).name

			return state.enumValueCompatible(member.Value, expected)
		}
	}

	return false
}
