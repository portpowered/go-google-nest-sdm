package httptransport

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func (client *sdkClient) ListDevices(
	ctx context.Context,
	request sdm.ListDevicesRequest,
) (sdm.ListDevicesResult, error) {
	path, err := resourcePath(request.Parent, protocol.PathListDevices, protocol.CollectionEnterprises)
	if err != nil {
		return sdm.ListDevicesResult{}, publicError(err)
	}

	if request.Filter != nil {
		query := url.Values{}
		query.Set(protocol.QueryFilter, *request.Filter)
		path += "?" + query.Encode()
	}

	var payload json.RawMessage

	err = client.transport.exchange(
		ctx,
		"ListDevices",
		protocol.MethodListDevices,
		client.transport.sdmBaseURL+path,
		request.Auth.AccessToken,
		"",
		nil,
		&payload,
	)
	if err != nil {
		return sdm.ListDevicesResult{}, publicError(err)
	}

	values, err := decodeResourceList(payload, protocol.KeyDevices, "ListDevices", sdm.DecodeDevice)
	if err != nil {
		return sdm.ListDevicesResult{}, publicError(err)
	}

	return sdm.ListDevicesResult{Devices: values}, nil
}

func (client *sdkClient) GetDevice(ctx context.Context, request sdm.GetDeviceRequest) (sdm.GetDeviceResult, error) {
	payload,
		err := client.resource(
		ctx,
		request.Auth.AccessToken,
		request.Name,
		"GetDevice",
		protocol.MethodGetDevice,
		protocol.PathGetDevice,
		protocol.CollectionEnterprises,
		protocol.CollectionDevices,
	)
	if err != nil {
		return sdm.GetDeviceResult{}, err
	}

	value, err := sdm.DecodeDevice(payload)
	if err != nil {
		return sdm.GetDeviceResult{}, publicDecodeError("GetDevice", err)
	}

	return sdm.GetDeviceResult{Device: value}, nil
}
func (client *sdkClient) ListStructures(
	ctx context.Context,
	request sdm.ListStructuresRequest,
) (sdm.ListStructuresResult, error) {
	payload,
		err := client.resource(
		ctx,
		request.Auth.AccessToken,
		request.Parent,
		"ListStructures",
		protocol.MethodListStructures,
		protocol.PathListStructures,
		protocol.CollectionEnterprises,
	)
	if err != nil {
		return sdm.ListStructuresResult{}, err
	}

	values, err := decodeResourceList(payload, protocol.KeyStructures, "ListStructures", sdm.DecodeStructure)
	if err != nil {
		return sdm.ListStructuresResult{}, err
	}

	return sdm.ListStructuresResult{Structures: values}, nil
}

func (client *sdkClient) GetStructure(
	ctx context.Context,
	request sdm.GetStructureRequest,
) (sdm.GetStructureResult, error) {
	payload,
		err := client.resource(
		ctx,
		request.Auth.AccessToken,
		request.Name,
		"GetStructure",
		protocol.MethodGetStructure,
		protocol.PathGetStructure,
		protocol.CollectionEnterprises,
		protocol.CollectionStructures,
	)
	if err != nil {
		return sdm.GetStructureResult{}, err
	}

	value, err := sdm.DecodeStructure(payload)
	if err != nil {
		return sdm.GetStructureResult{}, publicDecodeError("GetStructure", err)
	}

	return sdm.GetStructureResult{Structure: value}, nil
}
func (client *sdkClient) ListRooms(ctx context.Context, request sdm.ListRoomsRequest) (sdm.ListRoomsResult, error) {
	payload,
		err := client.resource(
		ctx,
		request.Auth.AccessToken,
		request.Parent,
		"ListRooms",
		protocol.MethodListRooms,
		protocol.PathListRooms,
		protocol.CollectionEnterprises,
		protocol.CollectionStructures,
	)
	if err != nil {
		return sdm.ListRoomsResult{}, err
	}

	values, err := decodeResourceList(payload, protocol.KeyRooms, "ListRooms", sdm.DecodeRoom)
	if err != nil {
		return sdm.ListRoomsResult{}, err
	}

	return sdm.ListRoomsResult{Rooms: values}, nil
}

func (client *sdkClient) GetRoom(ctx context.Context, request sdm.GetRoomRequest) (sdm.GetRoomResult, error) {
	payload,
		err := client.resource(
		ctx,
		request.Auth.AccessToken,
		request.Name,
		"GetRoom",
		protocol.MethodGetRoom,
		protocol.PathGetRoom,
		protocol.CollectionEnterprises,
		protocol.CollectionStructures,
		protocol.CollectionRooms,
	)
	if err != nil {
		return sdm.GetRoomResult{}, err
	}

	value, err := sdm.DecodeRoom(payload)
	if err != nil {
		return sdm.GetRoomResult{}, publicDecodeError("GetRoom", err)
	}

	return sdm.GetRoomResult{Room: value}, nil
}

//nolint:unparam // Schema-generated method remains paired with its route at every call site.
func (client *sdkClient) resource(
	ctx context.Context,
	token,
	name,
	operation,
	method,
	template string,
	collections ...string,
) (json.RawMessage, error) {
	path, err := resourcePath(name, template, collections...)
	if err != nil {
		return nil, publicError(err)
	}

	var payload json.RawMessage

	err = client.transport.exchange(
		ctx, operation, method, client.transport.sdmBaseURL+path, token, "", nil, &payload,
	)

	return payload, publicError(err)
}

func decodeResourceList[Resource any](
	payload []byte,
	key,
	operation string,
	decode func([]byte) (Resource, error),
) ([]Resource, error) {
	var envelope map[string]json.RawMessage

	err := json.Unmarshal(payload, &envelope)
	if err != nil {
		return nil, publicDecodeError(operation, err)
	}

	if envelope == nil {
		return nil, publicDecodeError(operation, nil)
	}

	result := make([]Resource, 0)

	raw, exists := envelope[key]

	if !exists {
		return result, nil
	}

	var entries []json.RawMessage

	err = json.Unmarshal(raw, &entries)
	if err != nil {
		return nil, publicDecodeError(operation, err)
	}

	if entries == nil {
		return nil, publicDecodeError(operation, nil)
	}

	for _, entry := range entries {
		value, err := decode(entry)
		if err != nil {
			return nil, publicDecodeError(operation, err)
		}

		result = append(result, value)
	}

	return result, nil
}

func publicDecodeError(operation string, cause error) error {
	return &sdm.Error{Operation: operation, Kind: sdm.ErrorInvalidResponse, StatusCode: 0, Cause: cause}
}
