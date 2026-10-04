package main

import "go/ast"

// decoderForwarding proves an untyped helper only hands the selected result to JSON decoding.
func (state *wireValueAudit) decoderForwarding(function wireCallable, outputIndex int) (int, bool) {
	parameters := callableParameters(function.signature)
	if outputIndex >= len(parameters) {
		return 0, false
	}

	output := parameters[outputIndex]
	parents := nodeParents(function.body)
	inputIndex := -1
	valid := true
	count := 0

	ast.Inspect(function.body, func(node ast.Node) bool {
		identifier, recognized := node.(*ast.Ident)
		if !recognized || identifier.Obj != output.Obj {
			return true
		}

		call, recognized := parents[identifier].(*ast.CallExpr)
		if !recognized || len(call.Args) != 2 || call.Args[1] != identifier || !state.jsonDecoder(call) {
			valid = false

			return true
		}

		input, recognized := call.Args[0].(*ast.Ident)
		if !recognized {
			valid = false

			return true
		}

		matched := false

		for index, parameter := range parameters {
			if parameter.Obj == input.Obj {
				inputIndex = index
				matched = true
			}
		}

		valid = valid && matched
		count++

		return true
	})

	return inputIndex, valid && count == 1
}

func callableParameters(signature *ast.FuncType) []*ast.Ident {
	parameters := []*ast.Ident{}
	if signature.Params == nil {
		return parameters
	}

	for _, field := range signature.Params.List {
		parameters = append(parameters, field.Names...)
	}

	return parameters
}

func (state *wireValueAudit) decoderInput(call *ast.CallExpr, outputIndex int) (ast.Expr, bool) {
	if state.jsonDecoder(call) && len(call.Args) == 2 {
		return call.Args[0], true
	}

	function := localCallable(call.Fun, map[wireBinding]bool{})
	if function.signature == nil {
		return nil, false
	}

	inputIndex, valid := state.decoderForwarding(function, outputIndex)
	if !valid || inputIndex >= len(call.Args) {
		return nil, false
	}

	return call.Args[inputIndex], true
}
