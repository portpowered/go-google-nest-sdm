package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const validControl = "valid"

const resourceHelperSource = `package transport
 import "fmt"
 import "net/url"
 func resourcePath(name, template string, collections ...string) (string, error) {
  values := make([]any, len(collections))
  for index := range collections { id := name; values[index] = url.PathEscape(id) }
  return fmt.Sprintf(template, values...), nil
 }`

func TestResourceHelperProvenance(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"suffix": strings.ReplaceAll(resourceHelperSource, "fmt.Sprintf(template, values...)",
			`fmt.Sprintf(template, values...)+"/novel"`),
		"wrapper": strings.ReplaceAll(resourceHelperSource, "fmt.Sprintf(template, values...)",
			`wrap(fmt.Sprintf(template, values...))`),
		"template mutation": strings.ReplaceAll(resourceHelperSource, "values :=", `template += "/novel"; values :=`),
		"template escape":   strings.ReplaceAll(resourceHelperSource, "values :=", `mutate(&template); values :=`),
		"raw segment":       strings.ReplaceAll(resourceHelperSource, "url.PathEscape(id)", "id"),
		"fmt shadow":        strings.ReplaceAll(resourceHelperSource, "values :=", `fmt := attacker; values :=`),
		"url shadow":        strings.ReplaceAll(resourceHelperSource, "values :=", `url := attacker; values :=`),
		"values alias": strings.ReplaceAll(resourceHelperSource, "return fmt.Sprintf",
			`alias := values; alias[0] = "/novel"; return fmt.Sprintf`),
		"early success": strings.ReplaceAll(resourceHelperSource, "values :=",
			`if enabled { return "/novel", nil }; values :=`),
	}

	checkResourceHelper(t, resourceHelperSource, true)

	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkResourceHelper(t, source, false)
		})
	}
}

func checkResourceHelper(t *testing.T, source string, expected bool) {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}

	imports, err := auditImports(file)
	if err != nil {
		t.Fatal(err)
	}

	function := namedFunction(file, "resourcePath")
	if function == nil {
		t.Fatal("missing helper")
	}

	if safeResourceHelper(function, imports) != expected {
		t.Fatal("unexpected helper provenance result")
	}
}

func TestResourceHelperShadow(t *testing.T) {
	t.Parallel()

	source := strings.ReplaceAll(adapterSource, "path, err :=",
		`resourcePath := attacker; path, err :=`)

	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = auditFile(file, "pkg/dependencies/httptransport/resources.go")
	if err == nil {
		t.Fatal("shadowed route constructor accepted")
	}
}

func TestMediaPatternProvenance(t *testing.T) {
	t.Parallel()

	const source = `package media
 import "regexp"
 import protocol "github.com/portpowered/go-google-nest-sdm/internal/protocol"
 var imageURL = regexp.MustCompile(protocol.MediaImageURLPattern)
 var clipURL = regexp.MustCompile(protocol.MediaClipURLPattern)`

	cases := map[string]string{
		validControl:          source,
		"raw pattern":         strings.ReplaceAll(source, "protocol.MediaImageURLPattern", `".*"`),
		"wrong pattern":       strings.ReplaceAll(source, "protocol.MediaImageURLPattern", "protocol.MediaClipURLPattern"),
		"counterfeit package": strings.ReplaceAll(source, module+"/internal/protocol", "example.invalid/protocol"),
	}
	for name, probe := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			file, err := parser.ParseFile(token.NewFileSet(), "probe.go", probe, 0)
			if err != nil {
				t.Fatal(err)
			}

			imports, err := auditImports(file)
			if err != nil {
				t.Fatal(err)
			}

			err = auditMediaPatterns(file, imports)
			if (err == nil) != (name == validControl) {
				t.Fatal("unexpected pattern provenance result")
			}
		})
	}
}

func TestMediaConstructorShadow(t *testing.T) {
	t.Parallel()

	const source = `package media
 import "regexp"
 import protocol "github.com/portpowered/go-google-nest-sdm/internal/protocol"
 var imageURL = regexp.MustCompile(protocol.MediaImageURLPattern)
 func probe() { REPLACEMENT; return checkedURL(input.URL, imageURL) }`

	cases := map[string]string{
		validControl:     "",
		"helper shadow":  "checkedURL := attacker",
		"pattern shadow": `imageURL := regexp.MustCompile(".*")`,
		"pattern alias":  "imageURL := attacker",
	}
	for name, replacement := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			probe := strings.ReplaceAll(source, "REPLACEMENT", replacement)

			file, err := parser.ParseFile(token.NewFileSet(), "probe.go", probe, 0)
			if err != nil {
				t.Fatal(err)
			}

			function := namedFunction(file, "probe")
			if function == nil {
				t.Fatal("missing function")
			}

			terminal, recognized := function.Body.List[len(function.Body.List)-1].(*ast.ReturnStmt)
			if !recognized {
				t.Fatal("missing terminal return")
			}

			accepted := checkedMediaURL(terminal.Results[0], imageOperation)
			if accepted != (name == validControl) {
				t.Fatal("unexpected media constructor provenance result")
			}
		})
	}
}
