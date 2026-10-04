// Command generate rebuilds schema-owned models and protocol constants.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
)

var errUnsupportedGenerator = errors.New("unsupported generator executable")

const (
	oapiVersion       = "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1"
	wireConfig        = "wire.yaml"
	generatedFileMode = 0o600
)

type generation struct{ config, output, schema string }

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	err := command("node", "tools/generate/modelina.mjs")
	if err != nil {
		return err
	}

	err = projection("traits.openapi.yaml", "pkg/sdm/traits.gen.go")
	if err != nil {
		return err
	}

	err = projection("commands.openapi.yaml", "pkg/sdm/commands.gen.go")
	if err != nil {
		return err
	}

	jobs := []generation{
		{wireConfig, "pkg/dependencymodels/traits.gen.go", "traits.openapi.yaml"},
		{"commands-wire.yaml", "pkg/dependencymodels/commands.gen.go", "commands.openapi.yaml"},
		{"resources.yaml", "pkg/dependencymodels/devices.gen.go", "openapi.yaml"},
		{wireConfig, "pkg/dependencymodels/errors.gen.go", "errors.openapi.yaml"},
		{"pubsub-wire.yaml", "pkg/dependencymodels/pubsub.gen.go", "external/pubsub.openapi.yaml"},
		{wireConfig, "pkg/dependencymodels/oauth.gen.go", "external/oauth.openapi.yaml"},
		{"media-public.yaml", "pkg/sdm/media.gen.go", "client-media.openapi.yaml"},
		{"public.yaml", "pkg/sdm/provider-errors.gen.go", "client-errors.openapi.yaml"},
	}
	for _, job := range jobs {
		err = generateModels(job)
		if err != nil {
			return err
		}
	}

	err = constants()
	if err != nil {
		return err
	}

	err = command("go", "run", oapiVersion, "--config",
		"api/client-resources.codegen.yaml", "api/client-resources.openapi.yaml")
	if err != nil {
		return err
	}

	return runtimeSchemas()
}

func generateModels(job generation) error {
	return command("go", "run", oapiVersion, "--config",
		filepath.Join("tools/generate", job.config), "-o", job.output, filepath.Join("api", job.schema))
}

func command(name string, arguments ...string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var cmd *exec.Cmd

	switch name {
	case "go":
		//nolint:gosec // G204: the pinned generator and repository-owned argument inventory are the only callers.
		cmd = exec.CommandContext(ctx, "go", arguments...)
	case "node":
		//nolint:gosec // G204: only the checked-in Modelina script is executed with repository-owned arguments.
		cmd = exec.CommandContext(ctx, "node", arguments...)
	default:
		return fmt.Errorf("%w: %q", errUnsupportedGenerator, name)
	}

	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	err := cmd.Run()
	if err != nil {
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

	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("decode schema %s: %w", path, err)
	}

	return result, nil
}
