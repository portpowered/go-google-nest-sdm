// Command docsbundle generates one documentation view of the checked-in REST contracts.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const outputFile = "docs/api-reference.openapi.json"

type object map[string]any

type source struct {
	filename string
	prefix   string
}

type bundleError struct {
	operation string
	cause     error
}

func (err bundleError) Error() string {
	if err.cause == nil {
		return err.operation
	}

	return err.operation + ": " + err.cause.Error()
}
func (err bundleError) Unwrap() error { return err.cause }

func main() {
	err := generate(".", outputFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func inputs() []source {
	return []source{
		{filename: "api/openapi.yaml", prefix: "SDM"},
		{filename: "api/client-models.openapi.yaml", prefix: "SDK"},
		{filename: "api/external/pubsub.openapi.yaml", prefix: "PubSub"},
		{filename: "api/external/oauth.openapi.yaml", prefix: "OAuth"},
		{filename: "api/external/media.openapi.yaml", prefix: "Media"},
	}
}

func generate(root, output string) error {
	bundle, err := combine(root, output, inputs())
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return bundleError{operation: "encode documentation contract", cause: err}
	}

	const fileMode = 0o600
	// #nosec G703 -- output is the repository-owned fixed generated documentation path.
	err = os.WriteFile(filepath.Join(root, output), append(encoded, '\n'), fileMode)
	if err != nil {
		return bundleError{operation: "write documentation contract", cause: err}
	}

	return nil
}
