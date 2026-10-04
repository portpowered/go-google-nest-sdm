// Command generate rebuilds schema-owned models and protocol constants.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const oapiVersion = "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1"

type generation struct{ config, output, schema string }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	jobs := []generation{
		{"wire.yaml", "pkg/dependencymodels/traits.gen.go", "traits.openapi.yaml"},
		{"commands-wire.yaml", "pkg/dependencymodels/commands.gen.go", "commands.openapi.yaml"},
		{"resources.yaml", "pkg/dependencymodels/devices.gen.go", "openapi.yaml"},
		{"wire.yaml", "pkg/dependencymodels/pubsub.gen.go", "external/pubsub.openapi.yaml"},
		{"wire.yaml", "pkg/dependencymodels/oauth.gen.go", "external/oauth.openapi.yaml"},
	}
	if err := projection("traits.openapi.yaml", "pkg/sdm/traits.gen.go"); err != nil {
		return err
	}
	if err := projection("commands.openapi.yaml", "pkg/sdm/commands.gen.go"); err != nil {
		return err
	}
	for _, job := range jobs {
		if err := command("go", "run", oapiVersion, "--config", filepath.Join("tools/generate", job.config), "-o", job.output, filepath.Join("api", job.schema)); err != nil {
			return err
		}
	}
	if err := constants(); err != nil {
		return err
	}
	if err := command("node", "tools/generate/modelina.mjs"); err != nil {
		return err
	}
	if err := command("go", "run", oapiVersion, "--config", "api/client-resources.codegen.yaml", "api/client-resources.openapi.yaml"); err != nil {
		return err
	}
	if err := command("go", "run", oapiVersion, "--config", "tools/generate/media-public.yaml", "-o", "pkg/sdm/media.gen.go", "api/client-media.openapi.yaml"); err != nil {
		return err
	}
	return runtimeSchemas()
}

func command(name string, arguments ...string) error {
	cmd := exec.Command(name, arguments...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}

func document(path string) (map[string]any, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	var result map[string]any
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode schema %s: %w", path, err)
	}
	return result, nil
}
