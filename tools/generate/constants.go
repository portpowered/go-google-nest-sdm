package main

import (
	"fmt"
	"go/format"
	"os"
	"sort"
	"strconv"
	"strings"
)

func object(value any) map[string]any           { result, _ := value.(map[string]any); return result }
func schemas(doc map[string]any) map[string]any { return object(object(doc["components"])["schemas"]) }

func constants() error {
	values := map[string]string{}
	for _, source := range []struct{ path, prefix string }{{"api/openapi.yaml", "SDM"}, {"api/external/pubsub.openapi.yaml", "PubSub"}, {"api/external/oauth.openapi.yaml", "OAuth"}} {
		doc, err := document(source.path)
		if err != nil {
			return err
		}
		servers, _ := doc["servers"].([]any)
		values[source.prefix+"BaseURL"], _ = object(servers[0])["url"].(string)
		for path, raw := range object(doc["paths"]) {
			for method, operation := range object(raw) {
				op := object(operation)
				id, ok := op["operationId"].(string)
				if !ok {
					continue
				}
				values["Method"+id] = strings.ToUpper(method)
				values["Path"+id] = pathTemplate(path)
			}
		}
	}
	doc, err := document("api/protocol.openapi.yaml")
	if err != nil {
		return err
	}
	for name, raw := range object(object(schemas(doc)["ProtocolValues"])["properties"]) {
		items, _ := object(raw)["enum"].([]any)
		values[name], _ = items[0].(string)
	}
	if err = writeConstants("internal/protocol/routes.gen.go", "protocol", values); err != nil {
		return err
	}
	mediaDoc, err := document("api/external/media.openapi.yaml")
	if err != nil {
		return err
	}
	mediaValues := map[string]string{}
	for path, raw := range object(mediaDoc["paths"]) {
		for method, operation := range object(raw) {
			id, _ := object(operation)["operationId"].(string)
			if id != "" {
				mediaValues["Method"+id] = strings.ToUpper(method)
				mediaValues["Path"+id] = pathTemplate(path)
			}
		}
	}
	if err = writeConstants("internal/protocol/media.gen.go", "protocol", mediaValues); err != nil {
		return err
	}
	traitDoc, err := document("api/traits.openapi.yaml")
	if err != nil {
		return err
	}
	for _, target := range []struct{ path, pkg string }{{"pkg/dependencymodels/trait-values.gen.go", "dependencymodels"}, {"pkg/sdm/trait-values.gen.go", "sdm"}} {
		if err = writeEnums(target.path, target.pkg, schemas(traitDoc)); err != nil {
			return err
		}
	}
	resourceDoc, err := document("api/openapi.yaml")
	if err != nil {
		return err
	}
	return writeEnums("pkg/dependencymodels/device-values.gen.go", "dependencymodels", schemas(resourceDoc))
}

func pathTemplate(path string) string {
	for strings.Contains(path, "{") {
		start := strings.Index(path, "{")
		end := strings.Index(path[start:], "}") + start
		path = path[:start] + "%s" + path[end+1:]
	}
	return path
}

func writeConstants(path, pkg string, values map[string]string) error {
	var body strings.Builder
	body.WriteString("// Code generated from checked-in API schemas. DO NOT EDIT.\npackage " + pkg + "\nconst (\n")
	keys := sortedKeys(values)
	for _, name := range keys {
		fmt.Fprintf(&body, "%s = %s\n", name, strconv.Quote(values[name]))
	}
	body.WriteString(")\n")
	return writeGo(path, body.String())
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeEnums(path, pkg string, models map[string]any) error {
	var body strings.Builder
	body.WriteString("// Code generated from open enum inventories. DO NOT EDIT.\npackage " + pkg + "\nconst (\n")
	for _, name := range sortedKeys(models) {
		items, _ := object(models[name])["x-known-values"].([]any)
		for _, item := range items {
			value, _ := item.(string)
			suffix := value
			if index := strings.LastIndex(value, "."); index >= 0 {
				suffix = value[index+1:]
			}
			fmt.Fprintf(&body, "%s%s %s = %s\n", name, strings.ReplaceAll(suffix, "_", ""), name, strconv.Quote(value))
		}
	}
	body.WriteString(")\n")
	return writeGo(path, body.String())
}

func writeGo(path, source string) error {
	data, err := format.Source([]byte(source))
	if err != nil {
		return fmt.Errorf("format generated constants: %w", err)
	}
	if err = os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write generated constants: %w", err)
	}
	return nil
}
