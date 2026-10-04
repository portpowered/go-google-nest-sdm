package main

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestWireValueNegativeControls(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"named result":  `func payload()(result wire.FanSetTimerParams){result.TimerMode="NOVEL_FIXED_VALUE";return}`,
		"pointer alias": `func payload(){value:=wire.FanSetTimerParams{};alias:=&value;alias.TimerMode="NOVEL_FIXED_VALUE"}`,
		"local indexed value": `func payload(){values:=map[string]string{"mode":"NOVEL_FIXED_VALUE"};
 value:=wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(values["mode"])};_ = value}`,
		"intermediate indexed map": `func payload(value wire.EmptyResults){
 holder:=[]wire.EmptyResults{value};holder[0]["unregistered"]=nil}`,
		"forged model constant": `func payload(){value:=wire.FanSetTimerParams{TimerMode:wire.NOVEL_FIXED_VALUE};_ = value}`,
		"constant alias": `const mode = "NOVEL_FIXED_VALUE";
 func payload(){value:=wire.FanSetTimerParams{TimerMode:mode};_ = value}`,
		"later scalar mutation": `func payload(input string){mode:=input;mode="NOVEL_FIXED_VALUE";
   value:=wire.FanSetTimerParams{TimerMode:mode};_ = value}`,
		"helper result": `func mode()string{return "NOVEL_FIXED_VALUE"};
   func payload(){value:=wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(mode())};_ = value}`,
		"named helper result": `func mode()(result string){result="NOVEL_FIXED_VALUE";return};
   func payload(){value:=wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(mode())};_ = value}`,
		"callback result": `func mode()func()string{return func()string{return "NOVEL_FIXED_VALUE"}};
   func payload(){value:=wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(mode()())};_ = value}`,
		"nested payload": `func payload(){value:=wire.ExecuteCommandRequest{Params:
   json.RawMessage(` + "`" + `{"unregistered":"NOVEL_FIXED_VALUE"}` + "`" + `)};_ = value}`,
		"map literal":       `func payload(){value:=wire.EmptyResults{"unregistered":nil};_ = value}`,
		"map key alias":     `func payload(){key:="unregistered";value:=wire.EmptyResults{key:nil};_ = value}`,
		"map mutation":      `func payload(key string){value:=wire.EmptyResults{};value["unregistered"]=nil;_ = key}`,
		"map pointer alias": `func payload(){value:=wire.EmptyResults{};alias:=&value;(*alias)["unregistered"]=nil}`,
		"named map helper escape": `func values()(result wire.EmptyResults){return};func mutate(value any){};
   func payload(){value:=values();mutate(value)}`,
		"callback map escape": `func payload()(result wire.EmptyResults){callback:=func(){mutate(result)};
 callback();return}`,
		"map aggregate escape": `func payload(){value:=wire.EmptyResults{};
 mutate(struct{Value wire.EmptyResults}{Value:value})}`,
		"local generic helper escape": `func mutate(value any){};func payload(){value:=wire.EmptyResults{};mutate(value)}`,
		"diagnostic separate": `func mode()(string,error){return "NOVEL_FIXED_VALUE",errors.New("diagnostic")};
   func payload(){actual,_:=mode();
 value:=wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(actual)};_ = value}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkWireSource(t, body, false)
		})
	}
}

func TestWireValuePositiveControls(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"caller scalar": `func payload(mode wire.FanSetTimerParamsTimerMode)wire.FanSetTimerParams{
   return wire.FanSetTimerParams{TimerMode:mode}}`,
		"generated constant": `func payload()wire.FanSetTimerParams{
 return wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerModeON}}`,
		"caller open key": `func payload(key string, value json.RawMessage)wire.EmptyResults{
   result:=wire.EmptyResults{};result[key]=value;return result}`,
		"named caller result": `func payload(mode wire.FanSetTimerParamsTimerMode)(result wire.FanSetTimerParams){
   result.TimerMode=mode;return}`,
		"typed local map helper": `func forward(value wire.EmptyResults)wire.EmptyResults{return value};
   func payload(value wire.EmptyResults)wire.EmptyResults{return forward(value)}`,
		"caller helper diagnostic": `func mode(input wire.FanSetTimerParamsTimerMode)(wire.FanSetTimerParamsTimerMode,error){
   return input,errors.New("diagnostic")};func payload(input wire.FanSetTimerParamsTimerMode){
   actual,_:=mode(input);value:=wire.FanSetTimerParams{TimerMode:actual};_ = value}`,
		"ignored helper diagnostic": `func mode(input wire.FanSetTimerParamsTimerMode,
 diagnostic string) wire.FanSetTimerParamsTimerMode{return input};func payload(input wire.FanSetTimerParamsTimerMode){
 value:=wire.FanSetTimerParams{TimerMode:mode(input,"diagnostic")};_ = value}`,
		"diagnostic only": `func failure()error{return errors.New("NOVEL_FIXED_VALUE")}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkWireSource(t, body, true)
		})
	}
}

func checkWireSource(t *testing.T, body string, expected bool) {
	t.Helper()

	source := `package probe
 import wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
 import "encoding/json"
 import "errors"
 ` + body

	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}

	models, err := loadWireModels("../..")
	if err != nil {
		t.Fatal(err)
	}

	err = auditFile(file, "pkg/dependencies/httptransport/sdk_commands.go", models)
	if (err == nil) != expected {
		t.Fatalf("unexpected wire provenance: %v", err)
	}
}
