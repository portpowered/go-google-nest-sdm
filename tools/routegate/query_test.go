package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

const queryProtocolAlias = "protocol"

func TestQueryProvenance(t *testing.T) {
	t.Parallel()

	for name, source := range queryPositiveControls() {
		t.Run(name, func(t *testing.T) { t.Parallel(); queryAcceptance(t, source, true) })
	}

	for name, source := range queryNegativeControls() {
		t.Run(name, func(t *testing.T) { t.Parallel(); queryAcceptance(t, source, false) })
	}
}

func queryPositiveControls() map[string]string {
	return map[string]string{
		"parenthesized receiver": `params := url.Values{};
(params).Set(protocol.QueryWidth,"1");`,
		"local alias": `params := url.Values{};
alias := params;
alias.Add(protocol.QueryWidth,"1");`,
		"var alias": `params := url.Values{};
var alias = params;
alias.Set(protocol.QueryWidth,"1");`,
		"parenthesized alias": `params := url.Values{};
alias := (params);
(alias)[(protocol.QueryWidth)] = []string{"1"};`,
		"literal": `params := url.Values{protocol.QueryWidth: {"1"}};`,
		"branch safe keys": `params := url.Values{};
if enabled { params.Set(protocol.QueryWidth,"1") } else { params.Add(protocol.QueryHeight,"2") };`,
		"trusted reassignment": `params := url.Values{};
alias := params;
params = url.Values{protocol.QueryWidth: {"1"}};
alias.Set(protocol.QueryHeight,"2");`,
		"lexical shadow": `params := url.Values{};
{ params := url.Values{};
params.Set("unregistered","1") };
params.Set(protocol.QueryWidth,"1");`,
	}
}

func queryNegativeControls() map[string]string {
	return map[string]string{
		"alias raw setter": `params := url.Values{};
alias := params;
(alias).Set("unregistered","1");`,
		"parenthesized raw index": `params := url.Values{};
(params)[("unregistered")] = []string{"1"};`,
		"literal raw key": `params := url.Values{"unregistered": {"1"}};`,
		"query setter alias": `params := url.Values{};
setter := (params).Set;
setter(protocol.QueryWidth,"1");`,
		"query helper argument": `params := url.Values{};
mutate(params);`,
		"map return": `params := url.Values{};
defer func() url.Values { return params }();`,
		"pointer escape": `params := url.Values{};
mutate(&params);`,
		"aggregate store": `params := url.Values{};
holder := struct{ Values url.Values }{params};
_ = holder;`,
		"package store": `params := url.Values{};
saved = params;`,
		"parse query reassign": `params := url.Values{};
params, _ = url.ParseQuery("unregistered=1");`,
		"URL query reassign": `params := url.Values{};
params = target.Query();`,
		"alias prior untrusted": `params := target.Query();
alias := params;
alias.Set(protocol.QueryWidth,"1");`,
		"branch taint": `params := url.Values{};
if enabled { params = target.Query() };`,
		"escaped alias": `params := url.Values{};
alias := params;
mutate(alias);`,
		"late taint": `params := url.Values{};
alias := params;
alias.Set(protocol.QueryWidth,"1");
params = target.Query();`,
	}
}

func queryAcceptance(t *testing.T, setup string, expected bool) {
	t.Helper()

	source := `package transport
 import "net/url"
 import protocol "github.com/portpowered/go-google-nest-sdm/internal/protocol"
 func query() { ` + setup + ` path += "?"+params.Encode() }`

	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}

	function, recognized := file.Decls[len(file.Decls)-1].(*ast.FuncDecl)
	if !recognized {
		t.Fatal("query function missing")
	}

	statement, recognized := function.Body.List[len(function.Body.List)-1].(*ast.AssignStmt)
	if !recognized {
		t.Fatal("query append missing")
	}

	imports := map[string]string{
		"url": "net/url", queryProtocolAlias: module + "/internal/protocol",
		generatedBindingPrefix + "QueryWidth": "width", generatedBindingPrefix + "QueryHeight": "height",
	}
	if actual := safeQueryAppend(statement.Rhs[0], function, imports); actual != expected {
		t.Fatalf("query acceptance %t, want %t", actual, expected)
	}
}

func TestQueryDefaultCommand(t *testing.T) {
	t.Parallel()
	root := testRootCopy(t)
	path := filepath.Join(root, "pkg/dependencies/httptransport/resources.go")
	baseline := requestRead(t, path) + "\nvar queryState url.Values\n"
	requestWrite(t, filepath.Join(root, "internal/protocol/query_probe.go"),
		"package protocol\nconst QueryNovelValue = \"unregistered\"\n")

	const marker = `err = client.exchange(ctx, "GetDevice"`

	controls := map[string]struct {
		Setup    string
		Accepted bool
	}{
		"parenthesized local alias": {`params := url.Values{};
alias := (params);
(alias).Set(protocol.QueryWidth,"1");
path += "?"+params.Encode()`, true},
		"generated literal and index": {`params := url.Values{protocol.QueryWidth: {"1"}};
(params)[protocol.QueryHeight] = []string{"2"};
path += "?"+params.Encode()`, true},
		"raw direct index": {`params := url.Values{};
(params)[("unregistered")] = []string{"1"};
path += "?"+params.Encode()`, false},
		"raw map literal": {`params := url.Values{"unregistered": {"1"}};
path += "?"+params.Encode()`, false},
		"package map storage": {`params := url.Values{};
queryState = params;
path += "?"+params.Encode()`, false},
		"pointer helper escape": {`params := url.Values{};
func(values *url.Values) { values.Set("unregistered","1") }(&params);
path += "?"+params.Encode()`, false},
		"returned map escape": {`params := url.Values{};
_ = func() url.Values { return params }();
path += "?"+params.Encode()`, false},
		"ParseQuery reassignment": {`params := url.Values{};
params, _ = url.ParseQuery("unregistered=1");
path += "?"+params.Encode()`, false},
		"counterfeit generated prefix": {`params := url.Values{};
params.Set(protocol.QueryNovelValue,"1");
path += "?"+params.Encode()`, false},
		"raw alias key": {`params := url.Values{};
alias := params;
(alias).Set("unregistered","1");
path += "?"+params.Encode()`, false},
		"URL query reassignment": {`params := url.Values{};
target := &url.URL{};
params = target.Query();
path += "?"+params.Encode()`, false},
		"map helper escape": {`params := url.Values{};
func(values url.Values) { values.Set("unregistered","1") }(params);
path += "?"+params.Encode()`, false},
		"aggregate escape": {`params := url.Values{};
holder := []url.Values{params};
_ = holder;
path += "?"+params.Encode()`, false},
		"query setter alias": {`params := url.Values{};
setter := params.Set;
setter(protocol.QueryWidth,"1");
path += "?"+params.Encode()`, false},
	}
	for name, control := range controls {
		t.Log(name)
		requestWrite(t, path, strings.Replace(baseline, marker, control.Setup+"\n"+marker, 1))
		requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
		requestCommand(t, root, control.Accepted, "run", "./tools/routegate")
	}
}
