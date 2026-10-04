package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHeaderAliasesDefaultCommand(t *testing.T) {
	t.Parallel()

	root := testRootCopy(t)
	path := filepath.Join(root, "pkg/dependencies/httptransport/exchange.go")
	baseline := requestRead(t, path)

	const anchor = "request.Header.Set(protocol.HeaderAccept, protocol.MIMEApplicationJSON)"

	const positive = `headers := http.Header(request.Header)
 (headers).Set(protocol.HeaderAccept, protocol.MIMEApplicationJSON)`

	for name, test := range map[string]struct {
		source   string
		accepted bool
	}{
		"converted parenthesized alias": {source: positive, accepted: true},
		"second alias": {
			source: `headers := http.Header(request.Header)
 alias := headers
 (alias).Set(protocol.HeaderAccept, protocol.MIMEApplicationJSON)`, accepted: true,
		},
		"unknown key": {source: strings.Replace(positive, "protocol.HeaderAccept", `"Unregistered"`, 1),
			accepted: false},
		"method value escape": {source: positive + "\n_ = headers.Set", accepted: false},
		"alias replacement": {
			source: positive + "\nheaders = http.Header{}", accepted: false,
		},
		"helper escape": {source: positive + "\nfunc(value http.Header) {}(headers)", accepted: false},
		"asynchronous write": {
			source: strings.Replace(positive, "(headers).Set", "go (headers).Set", 1), accepted: false,
		},
	} {
		t.Log(name)

		probe := strings.Replace(baseline, anchor, test.source, 1)
		if probe == baseline {
			t.Fatal("missing Accept header anchor")
		}

		requestWrite(t, path, probe)
		requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
		requestCommand(t, root, test.accepted, "run", "./tools/routegate")
	}
}
