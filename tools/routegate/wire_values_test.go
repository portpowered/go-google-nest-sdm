package main

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestWireValueNegativeControls(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"repeated helper caller then fixed": `func forward(value string)string{return value};
 func payload(input string){value:=wire.CameraLiveStreamGenerateRtspStreamResults{
 StreamToken:forward(input),StreamExtensionToken:forward("NOVEL_FIXED_VALUE")};_ = value}`,

		"unresolved cross-file helper": `func payload(){value:=wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(novel())};_ = value}`,
		"unresolved cross-file constant": `func payload(){value:=wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(novel)};_ = value}`,
		"wrong primitive field constant": `func payload(){value:=wire.OAuthTokenRequest{
 ClientId:protocol.HeaderAccept};_ = value}`,
		"wrong enum domain": `func payload(){value:=wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(wire.AudioCodecOPUS)};_ = value}`,
		"unknown public constant": `func payload(){value:=wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(sdm.NovelMode)};_ = value}`,
		"unknown protocol constant": `func payload(){value:=wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(protocol.NovelMode)};_ = value}`,
		"literal decoder input": `func payload()(result wire.EmptyResults){
 json.Unmarshal([]byte(` + "`" + `{"NOVEL_FIXED_KEY":true}` + "`" + `),&result);return}`,
		"native pointer helper": `func mutate(value *string){*value="NOVEL_FIXED_VALUE"};
 func payload()(result wire.FanSetTimerParams){mutate((*string)(&result.TimerMode));return}`,
		"outer caller request mutation": `func payload(input sdm.SetFanTimerRequest)wire.FanSetTimerParams{
 input.Params.TimerMode="NOVEL_FIXED_VALUE";return wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(input.Params.TimerMode)}}`,
		"caller projection pointer alias": `func payload(input sdm.SetFanTimerRequest)wire.FanSetTimerParams{
 alias:=&input.Params;alias.TimerMode="NOVEL_FIXED_VALUE";return wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(input.Params.TimerMode)}}`,
		"caller projection mutation": `func payload(input sdm.FanSetTimerParams)wire.FanSetTimerParams{
 input.TimerMode="NOVEL_FIXED_VALUE";return wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(input.TimerMode)}}`,
		"inferred nested generated value": `func payload(){values:=map[string]wire.FanSetTimerParams{
 "caller-key":{TimerMode:"NOVEL_FIXED_VALUE"}};_ = values}`,
		"second tuple scalar": `func mode()(error,string){return nil,"NOVEL_FIXED_VALUE"};
 func payload(){_,actual:=mode();value:=wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(actual)};_ = value}`,
		"second tuple model mutation": `func model()(error,wire.FanSetTimerParams){return nil,wire.FanSetTimerParams{}};
 func payload(){_,value:=model();value.TimerMode="NOVEL_FIXED_VALUE"}`,
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
		"local callback result": `func payload(){callback:=func()string{return "NOVEL_FIXED_VALUE"};
 value:=wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(callback())};_ = value}`,
		"helper function alias": `func mode()string{return "NOVEL_FIXED_VALUE"};
 func payload(){saved:=mode;value:=wire.FanSetTimerParams{
 TimerMode:wire.FanSetTimerParamsTimerMode(saved())};_ = value}`,
		"callback result": `func mode()func()string{return func()string{return "NOVEL_FIXED_VALUE"}};
   func payload(){value:=wire.FanSetTimerParams{TimerMode:wire.FanSetTimerParamsTimerMode(mode()())};_ = value}`,
		"nested payload": `func payload(){value:=wire.ExecuteCommandRequest{Params:
   json.RawMessage(` + "`" + `{"unregistered":"NOVEL_FIXED_VALUE"}` + "`" + `)};_ = value}`,
		"wrong generated map key": `func payload(){value:=wire.EmptyResults{protocol.HeaderAccept:nil};_ = value}`,
		"map literal":             `func payload(){value:=wire.EmptyResults{"unregistered":nil};_ = value}`,
		"map key alias":           `func payload(){key:="unregistered";value:=wire.EmptyResults{key:nil};_ = value}`,
		"map mutation":            `func payload(key string){value:=wire.EmptyResults{};value["unregistered"]=nil;_ = key}`,
		"map pointer alias":       `func payload(){value:=wire.EmptyResults{};alias:=&value;(*alias)["unregistered"]=nil}`,
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
		"caller decoder input": `func payload(input []byte)(result wire.EmptyResults){
 json.Unmarshal(input,&result);return}`,
		"inferred nested caller value": `func payload(input wire.FanSetTimerParamsTimerMode){
 values:=map[string]wire.FanSetTimerParams{"caller-key":{TimerMode:input}};_ = values}`,
		"inferred nested generated constant": `func payload(){values:=map[string]wire.FanSetTimerParams{
 "caller-key":{TimerMode:wire.FanSetTimerParamsTimerModeON}};_ = values}`,
		"second tuple caller value": `func mode(input wire.FanSetTimerParamsTimerMode)(error,wire.FanSetTimerParamsTimerMode){
 return errors.New("diagnostic"),input};func payload(input wire.FanSetTimerParamsTimerMode){
 _,actual:=mode(input);value:=wire.FanSetTimerParams{TimerMode:actual};_ = value}`,
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
		"caller callback alias": `func payload(input wire.FanSetTimerParamsTimerMode){
 callback:=func(value wire.FanSetTimerParamsTimerMode)wire.FanSetTimerParamsTimerMode{return value};
 result:=wire.FanSetTimerParams{TimerMode:callback(input)};_ = result}`,
		"helper alias diagnostic": `func mode(input wire.FanSetTimerParamsTimerMode,
 diagnostic string) wire.FanSetTimerParamsTimerMode{return input};func payload(input wire.FanSetTimerParamsTimerMode){
 saved:=mode;value:=wire.FanSetTimerParams{TimerMode:saved(input,"diagnostic")};_ = value}`,
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
 import sdm "github.com/portpowered/go-google-nest-sdm/pkg/sdm"
 import protocol "github.com/portpowered/go-google-nest-sdm/internal/protocol"
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
