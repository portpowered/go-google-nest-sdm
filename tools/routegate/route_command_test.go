package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const routeBranchSource = `package httptransport
import (
 "context"
 "github.com/portpowered/go-google-nest-sdm/internal/protocol"
 wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
)
func (client *Client) verificationRoute(ctx context.Context, token, parent string, selected bool) error {
 var path string
 var err error
 if selected {
  path, err = resourcePath(parent, protocol.PathListDevices, protocol.CollectionEnterprises)
 } else {
  path, err = resourcePath(parent, protocol.PathListDevices, protocol.CollectionEnterprises)
 }
 if err != nil { return err }
 var result wire.ListDevicesResponse
 return client.exchange(ctx, "ListDevices", protocol.MethodListDevices, client.sdmBaseURL+path, token, "", nil, &result)
}
`

func TestRouteBranchesDefaultCommand(t *testing.T) {
	t.Parallel()

	root := testRootCopy(t)
	path := filepath.Join(root, "pkg/dependencies/httptransport/verification_route.go")
	requestWrite(t, path, routeBranchSource)
	requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
	requestCommand(t, root, true, "run", "./tools/routegate")

	for name, source := range map[string]string{
		"uninitialized branch": strings.Replace(routeBranchSource,
			"} else {\n  path, err = resourcePath(parent, protocol.PathListDevices, protocol.CollectionEnterprises)",
			"} else {", 1),
		"different route": strings.Replace(routeBranchSource,
			"} else {\n  path, err = resourcePath(parent, protocol.PathListDevices",
			"} else {\n  path, err = resourcePath(parent, protocol.PathListStructures", 1),
	} {
		t.Log(name)
		requestWrite(t, path, source)
		requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
		requestCommand(t, root, false, "run", "./tools/routegate")
	}
}

func TestJSONForwardingDefaultCommand(t *testing.T) {
	t.Parallel()

	root := testRootCopy(t)
	path := filepath.Join(root, "pkg/dependencies/httptransport/exchange.go")
	baseline := requestRead(t, path)
	requestCommand(t, root, true, "run", "./tools/routegate")

	// #nosec G101 -- deliberately invalid synthetic token used by a source-gate negative control.
	for name, mutation := range map[string]string{
		"endpoint mutation": `endpoint += "/unregistered"`,
		"body replacement":  `body = []byte("unregistered")`,
		"token mutation":    `token = "unregistered"`,
	} {
		t.Log(name)

		const anchor = "return client.exchange(ctx, operation, method, endpoint, token,"

		probe := strings.Replace(baseline, anchor, mutation+"\n"+anchor, 1)
		if probe == baseline {
			t.Fatal("JSON helper forwarding anchor missing")
		}

		requestWrite(t, path, probe)
		requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
		requestCommand(t, root, false, "run", "./tools/routegate")
	}
}
