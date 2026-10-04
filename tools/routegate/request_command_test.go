package main

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const requestCommandTimeout = time.Minute

// These controls execute the command used by CI from its default repository root.
// Each mutated transport also compiles: rejection cannot rely on an invalid probe.
func TestRequestBoundaryDefaultCommand(t *testing.T) {
	t.Parallel()

	root := testRootCopy(t)
	path := filepath.Join(root, "pkg/dependencies/httptransport/exchange.go")
	baseline := requestRead(t, path)
	requestCommand(t, root, true, "run", "./tools/routegate")
	requestWrite(t, path, strings.ReplaceAll(baseline, "request.Header.Set", "((request).Header).Set"))
	requestCommand(t, root, true, "run", "./tools/routegate")

	for name, mutation := range requestMutations() {
		t.Log(name)

		probe := strings.Replace(baseline, "response, err := client.httpClient.Do(request)",
			mutation+"\nresponse, err := client.httpClient.Do(request)", 1)
		probe = strings.Replace(probe, `"net/http"`, "\"net/http\"\n\"net/url\"", 1)
		probe += "\nvar _ = url.User\n"
		requestWrite(t, path, probe)
		requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")

		output := requestCommand(t, root, false, "run", "./tools/routegate")
		if name == "URL path" && !strings.Contains(output, "request, target or headers mutate") {
			t.Fatalf("path mutation was not rejected at the actual request boundary: %s", output)
		}
	}

	requestWrite(t, path, baseline)
	requestCommand(t, root, true, "run", "./tools/routegate")
	requestInputs(t, root, path, baseline)
}

func requestInputs(t *testing.T, root, path, baseline string) {
	t.Helper()

	for _, mutation := range []string{
		`body = strings.NewReader("unregistered")`,
		`func(value io.Reader) {}(body)`,
		`alias := body; _ = alias`,
	} {
		probe := strings.Replace(baseline, "request, err := http.NewRequestWithContext",
			mutation+"\nrequest, err := http.NewRequestWithContext", 1)
		requestWrite(t, path, probe)
		requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
		requestCommand(t, root, false, "run", "./tools/routegate")
	}

	file, err := parser.ParseFile(token.NewFileSet(), "exchange.go", baseline, 0)
	if err != nil {
		t.Fatal(err)
	}

	function := namedFunction(file, exchangeHelper)
	if function == nil {
		t.Fatal("missing inventoried exchange helper")
	}

	duplicate := baseline[int(function.Pos())-1 : int(function.End())-1]
	duplicate = strings.Replace(duplicate, "*Client", "*Bogus", 1)
	duplicate += "\ntype Bogus struct { httpClient *http.Client; oauthBaseURL string }\n"
	requestWrite(t, path, baseline+"\n"+duplicate)
	requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
	requestCommand(t, root, false, "run", "./tools/routegate")
}

func requestMutations() map[string]string {
	return map[string]string{
		"method":                  `request.Method = http.MethodPost`,
		"URL origin":              `request.URL.Host = "attacker.invalid"`,
		"URL path":                `request.URL.Path = "/unregistered"`,
		"URL raw path":            `request.URL.RawPath = "/unregistered"`,
		"URL query":               `request.URL.RawQuery = "unregistered=value"`,
		"URL user info":           `request.URL.User = url.UserPassword("attacker", "secret")`,
		"URL alias":               `target := request.URL; target.Path = "/unregistered"`,
		"request alias":           `alias := request; alias.Method = http.MethodPost`,
		"request pointer":         `pointer := &request; (*pointer).Method = http.MethodPost`,
		"clone":                   `request = request.Clone(ctx)`,
		"request method value":    `_ = request.Clone`,
		"request helper escape":   `func(value *http.Request) { value.Method = http.MethodPost }(request)`,
		"replace injected client": `client.httpClient = &http.Client{}`,
		"fixed accept value":      `request.Header.Set(protocol.HeaderAccept, "NOVEL_FIXED_VALUE")`,
		"content type overwrite":  `contentType = "NOVEL_FIXED_VALUE"`,
		"injected client pointer": `_ = &client.httpClient`,
		"injected client alias":   `alias := client; alias.httpClient = &http.Client{}`,
		"injected client escape":  `func(value *Client) { value.httpClient = &http.Client{} }(client)`,
		"aggregate":               `_ = []*http.Request{request}`,
		"raw header":              `request.Header.Set("Unregistered", "value")`,
		"header alias":            `headers := request.Header; headers.Set("Unregistered", "value")`,
		"header convert":          `headers := http.Header(request.Header); headers.Set("Unregistered", "value")`,
		"body":                    `request.Body = io.NopCloser(strings.NewReader("unregistered"))`,
		"body factory": `request.GetBody = func() (io.ReadCloser, error) {
		 return io.NopCloser(strings.NewReader("unregistered")), nil
		}`,
		"content length":    `request.ContentLength = 1`,
		"transfer encoding": `request.TransferEncoding = []string{"chunked"}`,
		"host override":     `request.Host = "attacker.invalid"`,
		"trailer":           `request.Trailer = http.Header{"Unregistered": []string{"value"}}`,
		"close":             `request.Close = true`,
		"deferred header":   `defer request.Header.Set(protocol.HeaderAccept, protocol.MIMEApplicationJSON)`,
		"async header":      `go request.Header.Set(protocol.HeaderAccept, protocol.MIMEApplicationJSON)`,
		"callback header": `func() {
		 request.Header.Set(protocol.HeaderAccept, protocol.MIMEApplicationJSON)
		}()`,
	}
}

func testRootCopy(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	for _, tree := range []string{"api", "pkg", "internal", "tools/routegate", "docs", cliRoot} {
		err := filepath.WalkDir(filepath.Join("../..", tree), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			relative, err := filepath.Rel("../..", path)
			if err != nil {
				return fmt.Errorf("resolve copied request tree: %w", err)
			}

			target := filepath.Join(root, relative)
			if entry.IsDir() {
				return os.MkdirAll(target, 0o700)
			}

			requestWrite(t, target, requestRead(t, path))

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"go.mod", "go.sum"} {
		requestWrite(t, filepath.Join(root, name), requestRead(t, filepath.Join("../..", name)))
	}

	return root
}

func requestRead(t *testing.T, path string) string {
	t.Helper()

	// #nosec G304 -- fixed repository files or private test mirror paths.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func requestWrite(t *testing.T, path, source string) {
	t.Helper()

	// #nosec G703 -- all destinations belong to the test's private copied tree.
	err := os.WriteFile(path, []byte(source), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

func requestCommand(t *testing.T, root string, accepted bool, arguments ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestCommandTimeout)
	defer cancel()

	// #nosec G204 -- executable and arguments are fixed Go gate/build commands supplied by these tests.
	command := exec.CommandContext(ctx, "go", arguments...)
	command.Dir = root

	output, err := command.CombinedOutput()
	if (err == nil) != accepted {
		t.Fatalf("default command acceptance %t, expected %t: %s\n%v", err == nil, accepted, output, err)
	}

	if !accepted && !strings.Contains(string(output), errRouteInvalid.Error()) {
		t.Fatalf("command failed without route rejection: %s", output)
	}

	return string(output)
}
