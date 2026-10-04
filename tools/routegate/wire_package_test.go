package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWirePackageProvenance(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		caller   string
		helper   string
		accepted bool
	}{
		{name: "fixed scalar sibling", caller: `func payload()wire.FanSetTimerParams {
 return wire.FanSetTimerParams{TimerMode: wire.FanSetTimerParamsTimerMode(mode())}
}`, helper: `func mode()(result string){ result="NOVEL_FIXED_VALUE"; return }`, accepted: false},
		{name: "caller decoder sibling", caller: `func payload(input []byte)(result wire.EmptyResults){
 decode(input,&result);return}`,
			helper: `import "encoding/json"
 func decode(input []byte,result any){_ = json.Unmarshal(input,result)}`, accepted: true},
		{name: "fixed decoder sibling", caller: `func payload()(result wire.EmptyResults){
 decode([]byte("{\"NOVEL_FIXED_KEY\":true}"),&result);return}`,
			helper: `import "encoding/json"
 func decode(input []byte,result any){_ = json.Unmarshal(input,result)}`, accepted: false},
		{name: "mutated decoder sibling", caller: `func payload(input []byte)(result wire.EmptyResults){
 decode(input,&result);return}`,
			helper: `import "encoding/json"
 func decode(input []byte,result any){_ = json.Unmarshal(input,result)
 (*result.(*map[string]json.RawMessage))["NOVEL_FIXED_KEY"]=nil}`, accepted: false},

		{name: "caller scalar sibling", caller: `func payload(input string)wire.FanSetTimerParams {
 return wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(mode(input))}
}`, helper: `func mode(input string)(result string){result=input;return}`, accepted: true},
		{name: "map mutation sibling", caller: `func payload()(result wire.EmptyResults){
 result=wire.EmptyResults{};mutate(result);return}`,
			helper: `import alternate "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
 func mutate(value alternate.EmptyResults){value["NOVEL_FIXED_KEY"]=nil}`, accepted: false},
		{name: "returned map mutation sibling", caller: `func payload()(result wire.EmptyResults){
 result=makeMap();result["NOVEL_FIXED_KEY"]=nil;return}`,
			helper: `import alternate "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
 func makeMap()alternate.EmptyResults{return alternate.EmptyResults{}}`, accepted: false},
		{name: "caller map sibling", caller: `func payload(input wire.EmptyResults)wire.EmptyResults{return forward(input)}`,
			helper: `import alternate "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
 func forward(input alternate.EmptyResults)alternate.EmptyResults{return input}`, accepted: true},
		{name: "sibling import identity", caller: `func payload()wire.FanSetTimerParams{return mode()}`,
			helper: `import alternate "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
 func mode()alternate.FanSetTimerParams{return alternate.FanSetTimerParams{
 TimerMode:alternate.FanSetTimerParamsTimerModeON}}`, accepted: true},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()

			caller := `package probe
import wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
` + item.caller
			helper := "package probe\n" + item.helper
			files := parsePackageProbe(t, caller, helper)
			compilePackageProbe(t, caller, helper)

			models, err := loadWireModels("../..")
			if err != nil {
				t.Fatal(err)
			}

			err = auditWirePackage(files, models)
			if (err == nil) != item.accepted {
				t.Fatalf("unexpected package provenance: %v", err)
			}

			imports, err := auditImports(files[0])
			if err != nil {
				t.Fatal(err)
			}

			if imports["wire"] != wireModelImport {
				t.Fatal("package audit mutated original imports")
			}
		})
	}
}

func parsePackageProbe(t *testing.T, sources ...string) []*ast.File {
	t.Helper()

	files := make([]*ast.File, 0, len(sources))

	for index, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("probe", string(rune('a'+index))+".go"), source, 0)
		if err != nil {
			t.Fatal(err)
		}

		files = append(files, file)
	}

	return files
}

func compilePackageProbe(t *testing.T, caller, helper string) {
	t.Helper()

	directory := t.TempDir()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	manifest := fmt.Sprintf("module probe\n\ngo 1.26\n\nrequire %s v0.0.0\nreplace %s => %s\n",
		module, module, filepath.ToSlash(root))

	err = os.WriteFile(filepath.Join(directory, "go.mod"), []byte(manifest), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	for name, source := range map[string]string{"caller.go": caller, "helper.go": helper} {
		err := os.WriteFile(filepath.Join(directory, name), []byte(source), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	command := exec.CommandContext(t.Context(), "go", "test", "-mod=mod", "-run", "^$", ".")
	command.Dir = directory

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("package probe must compile: %v\n%s", err, output)
	}
}

func TestWirePackageDefaultCommand(t *testing.T) {
	t.Parallel()
	root := testRootCopy(t)
	directory := filepath.Join(root, "pkg/dependencies/httptransport")

	cases := []struct {
		name     string
		caller   string
		helper   string
		accepted bool
	}{
		{name: "caller scalar", caller: `func gatePayload(input string)wire.FanSetTimerParams{
 return wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(gateMode(input))}}`,
			helper: `func gateMode(input string)(result string){result=input;return}`, accepted: true},
		{name: "fixed named scalar", caller: `func gatePayload()wire.FanSetTimerParams{
 return wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(gateMode())}}`,
			helper: `func gateMode()(result string){result="NOVEL_FIXED_VALUE";return}`, accepted: false},
		{name: "returned map mutation", caller: `func gatePayload()(result wire.EmptyResults){
 result=gateMap();result["NOVEL_FIXED_KEY"]=nil;return}`,
			helper: `import alternate "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
 func gateMap()alternate.EmptyResults{return alternate.EmptyResults{}}`, accepted: false},
		{name: "forged generated sibling mutation", caller: `func gatePayload(value wire.EmptyResults){gateMutate(value)}`,
			helper: `import alternate "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
 func gateMutate(value alternate.EmptyResults){value["NOVEL_FIXED_KEY"]=nil}`, accepted: false},
		{name: "caller map", caller: `func gatePayload(input wire.EmptyResults)wire.EmptyResults{return gateMap(input)}`,
			helper: `import alternate "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
 func gateMap(input alternate.EmptyResults)alternate.EmptyResults{return input}`, accepted: true},
	}
	for _, item := range cases {
		t.Log(item.name)
		caller := `package httptransport
 import wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
 ` + item.caller
		helper := `// Code generated by imaginary generator. DO NOT EDIT.
 package httptransport
 ` + item.helper

		requestWrite(t, filepath.Join(directory, "gate_probe_caller.go"), caller)
		requestWrite(t, filepath.Join(directory, "gate_probe_helper.gen.go"), helper)
		requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
		requestCommand(t, root, item.accepted, "run", "./tools/routegate")
	}
}
