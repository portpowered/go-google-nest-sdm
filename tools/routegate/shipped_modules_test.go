package main

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestShippedExamplesDefaultCommand(t *testing.T) {
	t.Parallel()
	root := testRootCopy(t)
	path := filepath.Join(root, "examples/basic/main.go")
	baseline := requestRead(t, path)
	requestCommand(t, root, true, "run", "./tools/routegate")

	probe := strings.Replace(baseline, `"context"`, "\"context\"\n\"net/http\"", 1)
	probe += "\nfunc unregisteredExampleEdge(){_,_=http.Get(\"https://example.invalid\")}\n"
	requestWrite(t, path, probe)
	requestCommand(t, root, true, "test", "./examples/basic", "-run", "^$")
	requestCommand(t, root, false, "run", "./tools/routegate")
}

func TestBodyBackingDefaultCommand(t *testing.T) {
	t.Parallel()
	root := testRootCopy(t)
	path := filepath.Join(root, "pkg/dependencies/httptransport/exchange.go")
	baseline := requestRead(t, path)
	immutable := strings.Replace(baseline, "bytes.NewReader(body)", "strings.NewReader(string(body))", 1)
	requestWrite(t, path, immutable)
	requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
	requestCommand(t, root, true, "run", "./tools/routegate")

	probe := strings.Replace(baseline, "request, err := http.NewRequestWithContext(ctx, method, endpoint, body)",
		"backing := []byte(token); reader := bytes.NewReader(backing)\n"+
			"request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)", 1)
	probe = strings.Replace(probe, "response, err := client.httpClient.Do(request)",
		"backing[0] = 'X'\nresponse, err := client.httpClient.Do(request)", 1)
	requestWrite(t, path, probe)
	requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
	requestCommand(t, root, false, "run", "./tools/routegate")
}

type bodyProofTransport func(*http.Request) (*http.Response, error)

func (transport bodyProofTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestBackingMutationChangesTheBodyAtSend(t *testing.T) {
	t.Parallel()

	for _, immutable := range []bool{false, true} {
		backing := []byte("original")

		var reader io.Reader = bytes.NewReader(backing)

		if immutable {
			reader = strings.NewReader(string(backing))
		}

		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.invalid", reader)
		if err != nil {
			t.Fatal(err)
		}

		backing[0] = 'X'
		client := new(http.Client)
		client.Transport = bodyProofTransport(func(actual *http.Request) (*http.Response, error) {
			data, readErr := io.ReadAll(actual.Body)
			if readErr != nil {
				t.Fatal(readErr)
			}

			expected := "Xriginal"
			if immutable {
				expected = "original"
			}

			if string(data) != expected {
				t.Fatalf("actual sent body %q, expected %q", data, expected)
			}

			response := new(http.Response)
			response.StatusCode = http.StatusOK
			response.Body = http.NoBody

			return response, nil
		})

		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}

		_ = response.Body.Close()
	}
}
