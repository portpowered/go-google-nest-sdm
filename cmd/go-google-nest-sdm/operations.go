package main

import (
	"context"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func (app application) execute(
	ctx context.Context, group, operation string, options invocation, account credentials,
) (any, error) {
	if group == "auth" {
		return app.authenticate(ctx, operation, options, account)
	}

	if group == "events" {
		return app.events(ctx, operation, options, account)
	}

	if account.AccessToken == "" {
		return nil, errAccessTokenRequired
	}

	if options.resource == "" {
		return nil, errResourceRequired
	}

	auth := sdm.AuthContext{AccessToken: account.AccessToken}

	switch group {
	case "fan":
		return app.fan(ctx, operation, options, auth)
	case "thermostat":
		return app.thermostat(ctx, operation, options, auth)
	case "camera":
		result, err := app.camera(ctx, operation, options, auth)
		if err != nil {
			return nil, wrapError(err)
		}

		if !options.export {
			return mediaAcknowledgement{Acknowledged: true, Operation: operation}, nil
		}

		return result, nil
	default:
		return app.discovery(ctx, group, operation, options.resource, auth)
	}
}
func (app application) discovery(
	ctx context.Context, group, operation, resource string, auth sdm.AuthContext,
) (any, error) {
	switch group + "/" + operation {
	case "devices/list":
		return sdkResult(app.client.ListDevices(ctx, sdm.ListDevicesRequest{Auth: auth, Parent: resource, Filter: nil}))
	case "devices/get":
		return sdkResult(app.client.GetDevice(ctx, sdm.GetDeviceRequest{Auth: auth, Name: resource}))
	case "structures/list":
		return sdkResult(app.client.ListStructures(ctx, sdm.ListStructuresRequest{Auth: auth, Parent: resource}))
	case "structures/get":
		return sdkResult(app.client.GetStructure(ctx, sdm.GetStructureRequest{Auth: auth, Name: resource}))
	case "rooms/list":
		return sdkResult(app.client.ListRooms(ctx, sdm.ListRoomsRequest{Auth: auth, Parent: resource}))
	case "rooms/get":
		return sdkResult(app.client.GetRoom(ctx, sdm.GetRoomRequest{Auth: auth, Name: resource}))
	default:
		return nil, errUnknownCommand
	}
}

type mediaAcknowledgement struct {
	Acknowledged bool   `json:"acknowledged"`
	Operation    string `json:"operation"`
}
