package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type contractOperation struct {
	OperationID string `yaml:"operationId"`
}
type contractComponents struct {
	Schemas map[string]yaml.Node `yaml:"schemas"`
}
type contractDocument struct {
	Paths      map[string]map[string]contractOperation `yaml:"paths"`
	Components contractComponents                      `yaml:"components"`
}
type schemaProperty struct {
	Enum  []yaml.Node      `yaml:"enum"`
	AllOf []schemaProperty `yaml:"allOf"`
}
type schemaDetails struct {
	Properties map[string]schemaProperty `yaml:"properties"`
}

func loadComponents(root string) (map[string]bool, error) {
	components := map[string]bool{}

	err := filepath.WalkDir(filepath.Join(root, "api"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("scan contracts: %w", walkErr)
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".yaml") || strings.Contains(path, "codegen") {
			return nil
		}

		return collectComponents(path, components)
	})
	if err != nil {
		return nil, fmt.Errorf("load model contracts: %w", err)
	}

	if len(components) == 0 {
		return nil, fmt.Errorf("%w: schema model inventory empty", errContract)
	}

	return components, nil
}

func collectComponents(path string, components map[string]bool) error {
	// #nosec G304 -- path comes only from the repository api tree walk.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read contract: %w", err)
	}

	var document contractDocument

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return fmt.Errorf("decode contract: %w", err)
	}

	for name, schema := range document.Components.Schemas {
		components[name] = true

		var details schemaDetails

		err = schema.Decode(&details)
		if err != nil {
			return fmt.Errorf("decode component: %w", err)
		}

		for property, value := range details.Properties {
			if property != "" && propertyEnum(value) {
				components[name+strings.ToUpper(property[:1])+property[1:]] = true
			}
		}
	}

	for _, operations := range document.Paths {
		for _, operation := range operations {
			if operation.OperationID == "" {
				continue
			}

			components[operation.OperationID+"Params"] = true
			components[operation.OperationID+"JSONRequestBody"] = true
			components[operation.OperationID+"FormdataRequestBody"] = true
		}
	}

	return nil
}

func propertyEnum(property schemaProperty) bool {
	if len(property.Enum) > 0 {
		return true
	}

	return slices.ContainsFunc(property.AllOf, propertyEnum)
}
