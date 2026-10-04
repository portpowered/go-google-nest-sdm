package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestGeneratedEnumOriginalBinding(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body     string
		accepted bool
	}{
		"wire domain": {body: `func output()string{
return string(wire.FanSetTimerParamsTimerModeON)}`, accepted: true},
		"public domain": {body: `func output()string{
return string(sdm.FanSetTimerParamsTimerModeON)}`, accepted: true},
		"foreign domain": {body: `func output()string{return string(wire.AudioCodecOPUS)}`, accepted: false},
		"aliased foreign domain": {body: `func output()string{
mode:=string(wire.AudioCodecOPUS);return mode}`, accepted: false},
		"helper foreign domain": {body: `func mode()string{return string(wire.AudioCodecOPUS)}
func output()string{return mode()}`, accepted: false},
		"named helper domain": {body: `func mode()(s string){s=string(wire.AudioCodecOPUS);return}
func output()string{return mode()}`, accepted: false},
		"mutated alias domain": {body: `func output()string{mode:=string(wire.FanSetTimerParamsTimerModeON)
mode=string(wire.AudioCodecOPUS);return mode}`, accepted: false},
		"selected tuple domain": {body: `func mode()(error,string){return nil,string(wire.AudioCodecOPUS)}
func output()string{_,mode:=mode();return mode}`, accepted: false},
		"caller enum": {body: `func output(mode string)string{return mode}`, accepted: true},
		"binary foreign domain": {body: `func output(mode string)string{
return mode+string(wire.AudioCodecOPUS)}`, accepted: false},
		"indexed foreign domain": {body: `func output()string{
values:=[]string{string(wire.AudioCodecOPUS)};return values[0]}`, accepted: false},
		"repeated helper domain": {body: `func pass(mode string)string{return mode}
func output(mode string)string{return pass(mode)+pass(string(wire.AudioCodecOPUS))}`, accepted: false},
	}
	for name, example := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			file, err := parser.ParseFile(token.NewFileSet(), "probe.go", "package probe;"+example.body, 0)
			if err != nil {
				t.Fatal(err)
			}

			models, err := loadWireModels("../..")
			if err != nil {
				t.Fatal(err)
			}

			state := wireValueAudit{file: file, imports: map[string]string{"wire": wireModelImport, "sdm": module + "/pkg/sdm"},
				models: models, bindings: map[wireBinding]wireReference{}, callArguments: map[wireBinding]ast.Expr{},
				callers: map[wireBinding]bool{}, parents: nil}

			function, isFunction := file.Decls[len(file.Decls)-1].(*ast.FuncDecl)
			if !isFunction {
				t.Fatal("missing probe function")
			}

			statement, ok := function.Body.List[len(function.Body.List)-1].(*ast.ReturnStmt)
			if !ok {
				t.Fatal("missing probe return")
			}

			accepted := state.enumValueCompatible(statement.Results[0], "FanSetTimerParamsTimerMode")
			if accepted != example.accepted {
				t.Fatalf("original enum domain accepted=%v", accepted)
			}
		})
	}
}

func TestGeneratedConstantExactRegistration(t *testing.T) {
	t.Parallel()

	models, err := loadWireModels("../..")
	if err != nil {
		t.Fatal(err)
	}

	state := wireValueAudit{file: nil, imports: map[string]string{
		"wire": wireModelImport, "sdm": module + "/pkg/sdm", "protocol": module + "/internal/protocol",
	},
		models: models, bindings: nil, callArguments: nil, callers: nil, parents: nil}

	for _, text := range []string{"wire.NovelMode", "sdm.NovelMode", "protocol.NovelMode"} {
		expression, parseErr := parser.ParseExpr(text)
		if parseErr != nil {
			t.Fatal(parseErr)
		}

		selector, ok := expression.(*ast.SelectorExpr)
		if !ok {
			t.Fatal("probe not a selector")
		}

		if !state.generatedConstantFixed(selector) {
			t.Fatal("unregistered constant accepted", text)
		}
	}
}

func TestRegisteredConstantSourceDrift(t *testing.T) {
	t.Parallel()

	entry := wireCatalogEntry{
		Package: wireModelImport, Declaration: "FanSetTimerParamsTimerModeON", Kind: "constant",
		Source:         "pkg/dependencymodels/commands.gen.go:1",
		Schema:         "api/commands.openapi.yaml#/components/schemas/FanSetTimerParams/properties/timerMode",
		TypeExpression: "FanSetTimerParamsTimerMode", Value: `"ON"`,
	}

	for _, source := range []string{
		`package wire; const FanSetTimerParamsTimerModeON FanSetTimerParamsTimerMode = "NOVEL_FIXED_VALUE"`,
		`package wire; const FanSetTimerParamsTimerModeON AudioCodec = "ON"`,
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "approved.gen.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}

		err = registerCatalogEntry(map[string]ast.Expr{}, file, entry, "")
		if err == nil {
			t.Fatal("registered generated constant source/value mutation accepted")
		}
	}
}
