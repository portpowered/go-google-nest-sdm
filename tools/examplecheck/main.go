// Command examplecheck validates canonical schema examples without network access.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var errExample = errors.New("schema example verification failed")

const (
	exampleKey  = "example"
	examplesKey = "examples"
)

func main() {
	err := checkRepository(".")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}
}

func checkRepository(root string) error {
	loader, err := newRepositoryLoader(root)
	if err != nil {
		return err
	}

	err = filepath.WalkDir(filepath.Join(loader.root, "api"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("scan canonical schemas: %w", walkErr)
		}

		if entry.IsDir() {
			if entry.Name() == "contracts" {
				return filepath.SkipDir
			}

			return nil
		}

		if !schemaFile(path) {
			return nil
		}

		location := fileURL(path)
		compiler := jsonschema.NewCompiler()
		compiler.DefaultDraft(jsonschema.Draft7)
		compiler.AssertFormat()
		compiler.UseLoader(loader)

		document, loadErr := loader.Load(location)
		if loadErr != nil {
			return loadErr
		}

		addErr := compiler.AddResource(location, document)
		if addErr != nil {
			return fmt.Errorf("register schema document: %w", addErr)
		}

		return walkExamples(compiler, location, "", document, standaloneSchema(document))
	})
	if err != nil {
		return fmt.Errorf("check canonical examples: %w", err)
	}

	return nil
}

func standaloneSchema(document any) bool {
	object, ok := document.(map[string]any)
	if !ok {
		return false
	}

	_, openapi := object["openapi"]
	_, asyncapi := object["asyncapi"]

	return !openapi && !asyncapi
}

func schemaFile(path string) bool {
	if strings.HasSuffix(path, ".codegen.yaml") || strings.HasSuffix(path, ".codegen.yml") {
		return false
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func validateExample(compiler *jsonschema.Compiler, location, owner, label string, value any) error {
	schema, err := compiler.Compile(location + "#" + owner)
	if err != nil {
		return fmt.Errorf("%w: compile %s#%s: %w", errExample, location, owner, err)
	}

	err = schema.Validate(value)
	if err != nil {
		return fmt.Errorf("%w: %s#%s (%s): %w", errExample, location, owner, label, err)
	}

	return nil
}

func pointerPart(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}
