package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
)

// ListDevices retrieves the devices shared by the given enterprise.
func (client *Client) ListDevices(ctx context.Context, token, parent string) (wire.ListDevicesResponse, error) {
	var result wire.ListDevicesResponse

	path, err := resourcePath(parent, protocol.PathListDevices, protocol.CollectionEnterprises)
	if err != nil {
		return result, err
	}

	err = client.exchange(ctx, "ListDevices", protocol.MethodListDevices, client.sdmBaseURL+path, token, "", nil, &result)

	return result, err
}

// GetDevice retrieves a single shared device.
func (client *Client) GetDevice(ctx context.Context, token, name string) (wire.Device, error) {
	var result wire.Device

	path, err := resourcePath(name, protocol.PathGetDevice, protocol.CollectionEnterprises, protocol.CollectionDevices)
	if err != nil {
		return result, err
	}

	err = client.exchange(ctx, "GetDevice", protocol.MethodGetDevice, client.sdmBaseURL+path, token, "", nil, &result)

	return result, err
}

// ListStructures retrieves structures shared by the given enterprise.
func (client *Client) ListStructures(ctx context.Context, token, parent string) (wire.ListStructuresResponse, error) {
	var result wire.ListStructuresResponse

	path, err := resourcePath(parent, protocol.PathListStructures, protocol.CollectionEnterprises)
	if err != nil {
		return result, err
	}

	err = client.exchange(
		ctx,
		"ListStructures",
		protocol.MethodListStructures,
		client.sdmBaseURL+path,
		token,
		"",
		nil,
		&result,
	)

	return result, err
}

// GetStructure retrieves one shared structure.
func (client *Client) GetStructure(ctx context.Context, token, name string) (wire.Structure, error) {
	var result wire.Structure

	path, err := resourcePath(
		name,
		protocol.PathGetStructure,
		protocol.CollectionEnterprises,
		protocol.CollectionStructures,
	)
	if err != nil {
		return result, err
	}

	err = client.exchange(
		ctx,
		"GetStructure",
		protocol.MethodGetStructure,
		client.sdmBaseURL+path,
		token,
		"",
		nil,
		&result,
	)

	return result, err
}

// ListRooms retrieves rooms in a shared structure.
func (client *Client) ListRooms(ctx context.Context, token, parent string) (wire.ListRoomsResponse, error) {
	var result wire.ListRoomsResponse

	path, err := resourcePath(
		parent,
		protocol.PathListRooms,
		protocol.CollectionEnterprises,
		protocol.CollectionStructures,
	)
	if err != nil {
		return result, err
	}

	err = client.exchange(ctx, "ListRooms", protocol.MethodListRooms, client.sdmBaseURL+path, token, "", nil, &result)

	return result, err
}

// GetRoom retrieves one room in a shared structure.
func (client *Client) GetRoom(ctx context.Context, token, name string) (wire.Room, error) {
	var result wire.Room

	path, err := resourcePath(
		name,
		protocol.PathGetRoom,
		protocol.CollectionEnterprises,
		protocol.CollectionStructures,
		protocol.CollectionRooms,
	)
	if err != nil {
		return result, err
	}

	err = client.exchange(ctx, "GetRoom", protocol.MethodGetRoom, client.sdmBaseURL+path, token, "", nil, &result)

	return result, err
}

// ExecuteCommand sends exactly one command. A successful response acknowledges acceptance;
// it does not prove physical completion. Uncertain commands are never retried.
func (client *Client) ExecuteCommand(
	ctx context.Context, token, name string, input wire.ExecuteCommandRequest,
) (wire.ExecuteCommandResponse, error) {
	var result wire.ExecuteCommandResponse

	path, err := resourcePath(
		name,
		protocol.PathExecuteCommand,
		protocol.CollectionEnterprises,
		protocol.CollectionDevices,
	)
	if err != nil {
		return result, err
	}

	var payload json.RawMessage

	err = client.exchangeJSON(
		ctx,
		"ExecuteCommand",
		protocol.MethodExecuteCommand,
		client.sdmBaseURL+path,
		token,
		input,
		&payload,
	)
	if err != nil {
		return result, err
	}

	var fields map[string]json.RawMessage

	err = json.Unmarshal(payload, &fields)

	value, supplied := fields[protocol.KeyResults]

	if err != nil || (supplied && bytes.Equal(bytes.TrimSpace(value), []byte("null"))) {
		return result, fail("ExecuteCommand", ErrorInvalidResponse, err)
	}

	err = decodeJSON("ExecuteCommand", payload, &result)

	return result, err
}

func resourcePath(name, template string, collections ...string) (string, error) {
	parts := strings.Split(name, "/")
	if len(parts) != len(collections)*2 {
		return "", fail("ResourceName", ErrorInvalidRequest, nil)
	}

	values := make([]any, len(collections))

	for index, collection := range collections {
		id := parts[index*2+1]
		if parts[index*2] != collection || id == "" || id == "." || id == ".." || strings.ContainsAny(id, "?#%\\\r\n") {
			return "", fail("ResourceName", ErrorInvalidRequest, nil)
		}

		values[index] = url.PathEscape(id)
	}

	return fmt.Sprintf(template, values...), nil
}
