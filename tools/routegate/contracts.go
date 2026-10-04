package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func operationNames() []string {
	return []string{
		"ListDevices", "GetDevice", "ExecuteCommand", "ListStructures", "GetStructure", "ListRooms", "GetRoom",
		"OAuthToken", "Pull", "Acknowledge", "ModifyAckDeadline", "DownloadImage", "DownloadClipPreview",
	}
}

func knownOperation(name string) bool {
	for _, operation := range operationNames() {
		if operation == name {
			return true
		}
	}

	return false
}

func auditProtocol(root string) error {
	constants, err := protocolConstants(root)
	if err != nil {
		return err
	}

	seen := map[string]bool{}

	err = filepath.WalkDir(filepath.Join(root, "api"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		return auditSchema(path, constants, seen)
	})
	if err != nil {
		return fmt.Errorf("walk schemas: %w", err)
	}

	for _, name := range operationNames() {
		if !seen[name] {
			return fmt.Errorf("%w: operation %s absent from schema inventory", errRouteInvalid, name)
		}
	}

	return nil
}

func protocolConstants(root string) (map[string]string, error) {
	constants := map[string]string{}

	files, err := filepath.Glob(filepath.Join(root, "internal/protocol/*.gen.go"))
	if err != nil {
		return nil, fmt.Errorf("find protocol sources: %w", err)
	}

	for _, path := range files {
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return nil, fmt.Errorf("parse protocol source: %w", parseErr)
		}

		parseErr = collectConstants(file, constants)
		if parseErr != nil {
			return nil, parseErr
		}
	}

	if len(constants) == 0 {
		return nil, fmt.Errorf("%w: generated protocol inventory missing", errRouteInvalid)
	}

	return constants, nil
}

func collectConstants(file *ast.File, constants map[string]string) error {
	for _, declaration := range file.Decls {
		group, recognized := declaration.(*ast.GenDecl)
		if !recognized || group.Tok != token.CONST {
			continue
		}

		for _, spec := range group.Specs {
			value, recognized := spec.(*ast.ValueSpec)
			if !recognized {
				continue
			}

			err := collectConstantValues(value, constants)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func collectConstantValues(value *ast.ValueSpec, constants map[string]string) error {
	for index, name := range value.Names {
		if index >= len(value.Values) {
			return fmt.Errorf("%w: implicit generated route value", errRouteInvalid)
		}

		literal, recognized := value.Values[index].(*ast.BasicLit)
		if !recognized {
			return fmt.Errorf("%w: nonliteral generated route value", errRouteInvalid)
		}

		text, err := strconv.Unquote(literal.Value)
		if err != nil {
			return fmt.Errorf("unquote generated route: %w", err)
		}

		constants[name.Name] = text
	}

	return nil
}

type schemaOperation struct {
	OperationID string `yaml:"operationId"`
}
type routeSchema struct {
	Paths map[string]map[string]schemaOperation `yaml:"paths"`
}

func auditSchema(path string, constants map[string]string, seen map[string]bool) error {
	// The gate intentionally reads schemas selected by its repository-root walk.
	data, err := os.ReadFile(path) //nolint:gosec // G304: repository-owned schema input, never a network request.
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}

	var document routeSchema

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return fmt.Errorf("decode schema: %w", err)
	}

	placeholder := regexp.MustCompile(`\{[^{}]+\}`)

	for route, methods := range document.Paths {
		for method, operation := range methods {
			if operation.OperationID == "" {
				continue
			}

			name := operation.OperationID
			methodMatches := constants["Method"+name] == strings.ToUpper(method)

			pathMatches := constants["Path"+name] == placeholder.ReplaceAllString(route, "%s")

			if !methodMatches || !pathMatches {
				return fmt.Errorf("%w: generated method/path %s differs from schema", errRouteInvalid, name)
			}

			seen[name] = true
		}
	}

	return nil
}
