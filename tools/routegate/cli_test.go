package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCLINetworkBoundariesDefaultCommand(t *testing.T) {
	t.Parallel()
	root := testRootCopy(t)
	path := filepath.Join(root, cliRoot, "login.go")
	baseline := requestRead(t, path)
	modulePath := filepath.Join(root, cliRoot, "go.mod")
	requestWrite(t, modulePath, requestRead(t, modulePath)+
		"\nreplace "+module+" => ../..\n")
	requestCommand(t, root, true, "run", "./tools/routegate")

	for name, mutation := range map[string][2]string{
		"novel outbound HTTP":       {"package main", "package main"},
		"unsafe listener authority": {"address := redirect.Host", `address := "0.0.0.0:8080"`},
		"shell browser":             {`"xdg-open", address`, `"sh", "-c", address`},
		"altered browser target": {"launch(ctx, session.AuthorizationURL())",
			`launch(ctx, session.AuthorizationURL()+"/unregistered")`},
		"extra listener":        {"server.Serve(listener)", "server.ListenAndServe()"},
		"listener handoff lost": {"server.Serve(listener)", "server.Serve(nil)"},
		"browser launcher alias": {"err := launch(ctx, session.AuthorizationURL())",
			"alias := launch; err := alias(ctx, session.AuthorizationURL()); _ = launch(ctx, session.AuthorizationURL())"},
	} {
		t.Log(name)

		probe := strings.Replace(baseline, mutation[0], mutation[1], 1)
		if name == "novel outbound HTTP" {
			probe += "\nfunc unregisteredHTTP() { _, _ = http.Get(\"https://example.invalid\") }\n"
		}

		requestWrite(t, path, probe)
		requestCommand(t, root, true, "-C", cliRoot, "test", "-mod=mod", "-run", "^$", ".")
		requestCommand(t, root, false, "run", "./tools/routegate")
	}

	requestWrite(t, path, baseline)
	requestCommand(t, root, true, "run", "./tools/routegate")
}
