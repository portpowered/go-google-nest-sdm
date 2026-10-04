package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConsentBoundaryDefaultCommand(t *testing.T) {
	t.Parallel()
	root := testRootCopy(t)
	path := filepath.Join(root, "pkg/dependencies/authorization/callback.go")
	baseline := requestRead(t, path)
	requestCommand(t, root, true, "run", "./tools/routegate")

	for name, mutation := range map[string][2]string{
		"foreign authority": {"protocol.PCMBaseURL +", `"https://example.invalid" +`},
		"altered path":      {"protocol.PathPCMConsent", "protocol.PathListDevices"},
		"extra query": {"values.Set(protocol.QueryParamState, state)",
			"values.Set(protocol.QueryParamState, state); values.Set(protocol.QueryParamCode, state)"},
		"wrong scope":   {"protocol.OAuthScopeSDM", "protocol.OAuthResponseTypeCode"},
		"path mutation": {`"?" + values.Encode()`, `"/unregistered?" + values.Encode()`},
		"lexical shadow": {"return protocol.PCMBaseURL", `protocol := struct{ PCMBaseURL, PathPCMConsent string }{
 PCMBaseURL: "https://example.invalid", PathPCMConsent: "/%s"}; return protocol.PCMBaseURL`},
	} {
		t.Log(name)
		requestWrite(t, path, strings.Replace(baseline, mutation[0], mutation[1], 1))
		requestCommand(t, root, true, "test", "./pkg/dependencies/authorization", "-run", "^$")
		requestCommand(t, root, false, "run", "./tools/routegate")
	}

	requestWrite(t, path, baseline)
	requestCommand(t, root, true, "run", "./tools/routegate")
}
