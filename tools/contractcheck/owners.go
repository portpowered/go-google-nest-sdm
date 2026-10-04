package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func schemaOwner(root, path, name, kind string) (string, error) {
	schemaPath := modelSchema(path)
	if name == "SdmBearerAuthScopes" && schemaPath == "api/openapi.yaml" {
		return schemaPath + "#/components/securitySchemes/sdmBearerAuth", nil
	}

	if name == "PubsubBearerAuthScopes" && schemaPath == pubsubSchemaPath {
		return schemaPath + "#/components/securitySchemes/pubsubBearerAuth", nil
	}

	if strings.Contains(path, "internal/protocol/") {
		return protocolOwner(name), nil
	}

	if schemaPath == "" {
		return "", fmt.Errorf("%w: schema owner missing for %s", errContract, path)
	}
	// #nosec G304 -- schemaPath is from the fixed model generation manifest.
	data, err := os.ReadFile(filepath.Join(root, schemaPath))
	if err != nil {
		return "", fmt.Errorf("read inventory owner: %w", err)
	}

	var document contractDocument

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return "", fmt.Errorf("decode inventory owner: %w", err)
	}

	if _, exists := document.Components.Schemas[name]; exists {
		return schemaPath + "#/components/schemas/" + name, nil
	}

	owner, err := propertyOwner(document, name)
	if err != nil {
		return "", err
	}

	if owner != "" {
		return schemaPath + owner, nil
	}

	if kind == "type" {
		owner = operationOwner(document, name)
		if owner != "" {
			return schemaPath + owner, nil
		}
	}

	return "", fmt.Errorf("%w: owner not resolved for %s %s", errContract, path, name)
}

func propertyOwner(document contractDocument, name string) (string, error) {
	for component, schema := range document.Components.Schemas {
		var details schemaDetails

		err := schema.Decode(&details)
		if err != nil {
			return "", fmt.Errorf("decode inventory property: %w", err)
		}

		for property := range details.Properties {
			if component+strings.ToUpper(property[:1])+property[1:] == name {
				return "#/components/schemas/" + component + "/properties/" + property, nil
			}
		}
	}

	return "", nil
}

func operationOwner(document contractDocument, name string) string {
	for route, methods := range document.Paths {
		for method, operation := range methods {
			fragment := ""

			switch name {
			case operation.OperationID + "Params":
				fragment = "parameters"
			case operation.OperationID + "JSONRequestBody", operation.OperationID + "FormdataRequestBody":
				fragment = "requestBody"
			}

			if fragment != "" {
				return "#/paths/" + strings.ReplaceAll(route, "/", "~1") + "/" + method + "/" + fragment
			}
		}
	}

	return ""
}

func protocolOwner(name string) string {
	if name == "ChannelSDMEvents" {
		return "api/asyncapi.yaml#/channels/sdmEvents/address"
	}

	if name == "ChannelSDMEventsName" {
		return "api/asyncapi.yaml#/channels/sdmEvents"
	}

	for _, prefix := range []string{"Method", "Path"} {
		if after, ok := strings.CutPrefix(name, prefix); ok {
			operation := after
			schemaPath := sdmSchemaPath

			switch operation {
			case "Pull", "Acknowledge", "ModifyAckDeadline":
				schemaPath = pubsubSchemaPath
			case "OAuthToken":
				schemaPath = oauthSchemaPath
			case "DownloadImage", "DownloadClipPreview":
				schemaPath = "api/external/media.openapi.yaml"
			}

			return schemaPath + "#operationId=" + operation
		}
	}

	switch name {
	case "SDMBaseURL":
		return "api/openapi.yaml#/servers/0/url"
	case "PubSubBaseURL":
		return "api/external/pubsub.openapi.yaml#/servers/0/url"
	case "OAuthBaseURL":
		return "api/external/oauth.openapi.yaml#/servers/0/url"
	}

	return "api/protocol.openapi.yaml#/components/schemas/ProtocolValues/properties/" + name
}
