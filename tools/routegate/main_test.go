package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const adapterSource = `package httptransport
import protocol "github.com/portpowered/go-google-nest-sdm/internal/protocol"
func (client *Client) GetDevice(ctx Context, name string) error {
 path, err := resourcePath(name, protocol.PathGetDevice, "enterprises", "devices")
 if err != nil { return err }
 return client.exchange(ctx, "GetDevice", protocol.MethodGetDevice, client.sdmBaseURL+path, "token", "", nil, nil)
}`

func TestRouteNegativeControls(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"method mismatch":       strings.ReplaceAll(adapterSource, "MethodGetDevice", "MethodListDevices"),
		"raw method":            strings.ReplaceAll(adapterSource, "protocol.MethodGetDevice", `"GET"`),
		"raw route":             strings.ReplaceAll(adapterSource, "protocol.PathGetDevice", `"/novel"`),
		"appended path":         replaceEndpoint(`client.sdmBaseURL+path+"/novel"`),
		"wrapped path":          replaceEndpoint(`client.sdmBaseURL+wrap(path)`),
		"path mutation":         replaceExchange(`path = path+"/novel"; return client.exchange`),
		"helper escape":         replaceExchange(`mutate(path); return client.exchange`),
		"pointer escape":        replaceExchange(`mutate(&path); return client.exchange`),
		"shadow receiver":       replaceExchange(`{client := attacker; return client.exchange`) + "}",
		"untrusted origin":      replaceEndpoint(`attacker.origin+path`),
		"counterfeit qualifier": strings.ReplaceAll(adapterSource, module+"/internal/protocol", "example.invalid/protocol"),
		"method value":          replaceExchange(`saved := client.exchange; return saved`),
		"conditional route":     conditionalRouteSource(),
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			file, err := parser.ParseFile(token.NewFileSet(), "probe.go", source, 0)
			if err != nil {
				t.Fatal(err)
			}

			err = auditFile(file, "pkg/dependencies/httptransport/resources.go")
			if err == nil {
				t.Fatal("unsafe route accepted")
			}
		})
	}
}

func TestRoutePositiveControl(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", adapterSource, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = auditFile(file, "pkg/dependencies/httptransport/resources.go")
	if err != nil {
		t.Fatal(err)
	}
}

func replaceExchange(replacement string) string {
	return strings.ReplaceAll(adapterSource, "return client.exchange", replacement)
}

func conditionalRouteSource() string {
	source := strings.ReplaceAll(adapterSource, "path, err := resourcePath",
		`var path string; var err error; if enabled { path, err = resourcePath`)

	return strings.ReplaceAll(source, `"devices")`, `"devices") }`)
}

func TestGlobalNetworkPrimitive(t *testing.T) {
	t.Parallel()

	source := `package transport
 import "net/http"
 var leaked = http.Get
 func fetch() { leaked("https://example.invalid") }`

	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = auditFile(file, "pkg/dependencies/httptransport/resources.go")
	if err == nil {
		t.Fatal("file-scope network primitive accepted")
	}
}

func TestQueryNegativeControls(t *testing.T) {
	t.Parallel()

	const source = `package transport
 import "net/url"
 import protocol "github.com/portpowered/go-google-nest-sdm/internal/protocol"
 func query() { params := url.Values{}; params.Set(protocol.QueryPageSize, "1"); path += "?"+params.Encode() }`

	cases := map[string]string{
		"index mutation": `params["unregistered"] = []string{"value"};`,
		"alias":          `alias := params; alias.Set("unregistered", "value");`,
		"helper":         `mutate(params);`,
		"aggregate":      `holder := []url.Values{params}; _ = holder;`,
		"pointer":        `mutate(&params);`,
	}
	for name, mutation := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			probe := strings.ReplaceAll(source, `params.Set`, mutation+` params.Set`)

			file, err := parser.ParseFile(token.NewFileSet(), "probe.go", probe, 0)
			if err != nil {
				t.Fatal(err)
			}

			function, recognized := file.Decls[len(file.Decls)-1].(*ast.FuncDecl)
			if !recognized {
				t.Fatal("probe function missing")
			}

			appendStatement, recognized := function.Body.List[len(function.Body.List)-1].(*ast.AssignStmt)
			if !recognized {
				t.Fatal("append statement missing")
			}

			imports := map[string]string{"url": "net/url", "protocol": module + "/internal/protocol"}
			if safeQueryAppend(appendStatement.Rhs[0], function, imports) {
				t.Fatal("unsafe query accepted")
			}
		})
	}
}

func replaceEndpoint(replacement string) string {
	return strings.ReplaceAll(adapterSource, "client.sdmBaseURL+path", replacement)
}
