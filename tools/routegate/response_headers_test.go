package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestResponseHeaderDestinationControls(t *testing.T) {
	t.Parallel()

	source := strings.Replace(adapterSource, "import protocol", "import \"net/http\"\nimport protocol", 1)
	source = strings.Replace(source, "path, err :=", "var headers http.Header\n path, err :=", 1)
	//nolint:dupword // Separate nil arguments preserve the request body and decoded-result positions.
	source = strings.Replace(source, `nil, nil)`, `nil, nil, &headers)`, 1)

	for _, testCase := range []struct {
		name, source string
		valid        bool
	}{
		{name: "local", source: source, valid: true},
		{name: "global", source: strings.Replace(strings.Replace(source,
			"var headers http.Header\n ", "", 1), "func (client", "var headers http.Header\nfunc (client", 1), valid: false},
		{name: "field", source: strings.Replace(source, "&headers", "&client.headers", 1), valid: false},
		{name: "wrong type",
			source: strings.Replace(source, "var headers http.Header", "var headers string", 1), valid: false},
		{name: "multiple", source: strings.Replace(source, "&headers)", "&headers, &headers)", 1), valid: false},
		{name: "ellipsis", source: strings.Replace(source, "&headers)", "headers...)", 1), valid: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			file, err := parser.ParseFile(token.NewFileSet(), "probe.go", testCase.source, 0)
			if err != nil {
				t.Fatal(err)
			}

			err = auditFile(file, "pkg/dependencies/httptransport/resources.go")
			if (err == nil) != testCase.valid {
				t.Fatalf("valid=%v, audit=%v", testCase.valid, err)
			}
		})
	}
}
