package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// The audit has no depth cutoff. This chain keeps a future cutoff from silently
// treating unresolved library values as proven caller input.
const deepWireHelperCount = 200

func TestWireDeepAndRecursiveProvenanceDefaultCommand(t *testing.T) {
	t.Parallel()

	root := testRootCopy(t)
	directory := filepath.Join(root, "pkg/dependencies/httptransport")
	caller := `package httptransport
import wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
func gateDeepPayload(input string) wire.FanSetTimerParams {
 return wire.FanSetTimerParams{TimerMode: wire.FanSetTimerParamsTimerMode(gateDeep0(input))}
}`

	for _, item := range []struct {
		name     string
		helper   string
		accepted bool
	}{
		{name: "deep proven caller", helper: deepWireHelpers("input"), accepted: true},
		{name: "deep fixed fallback", helper: deepWireHelpers(`"NOVEL_FIXED_VALUE"`), accepted: false},
		{name: "recursive fixed fallback", helper: `func gateDeep0(input string) string {
 if input != "" { return gateDeep0(input[1:]) }
 return "NOVEL_FIXED_VALUE"
}`, accepted: false},
		{name: "unproved recursive caller", helper: `func gateDeep0(input string) string {
 if input != "" { return gateDeep0(input[1:]) }
 return input
}`, accepted: false},
	} {
		t.Log(item.name)
		requestWrite(t, filepath.Join(directory, "gate_deep_caller.go"), caller)
		requestWrite(t, filepath.Join(directory, "gate_deep_helper.go"), "package httptransport\n"+item.helper)
		requestCommand(t, root, true, "test", "./pkg/dependencies/httptransport", "-run", "^$")
		requestCommand(t, root, item.accepted, "run", "./tools/routegate")
	}
}

func deepWireHelpers(result string) string {
	declarations := make([]string, 0, deepWireHelperCount+1)
	for index := range deepWireHelperCount {
		declarations = append(declarations, fmt.Sprintf(
			"func gateDeep%d(input string) string { return gateDeep%d(input) }", index, index+1))
	}

	declarations = append(declarations, fmt.Sprintf(
		"func gateDeep%d(input string) string { return %s }", deepWireHelperCount, result))

	return strings.Join(declarations, "\n")
}
