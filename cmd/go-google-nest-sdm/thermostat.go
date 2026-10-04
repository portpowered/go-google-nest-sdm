package main

import (
	"context"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func (app application) thermostat(
	ctx context.Context,
	operation string,
	options invocation,
	auth sdm.AuthContext,
) (any, error) {
	switch operation {
	case "mode":
		params, err := readCommandParams[sdm.ThermostatModeSetModeParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsThermostatModeSetMode,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.SetThermostatMode(ctx, sdm.SetThermostatModeRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "eco":
		params, err := readCommandParams[sdm.ThermostatEcoSetModeParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsThermostatEcoSetMode,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.SetThermostatEcoMode(ctx, sdm.SetThermostatEcoModeRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "heat":
		params, err := readCommandParams[sdm.ThermostatTemperatureSetpointSetHeatParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.SetHeat(ctx, sdm.SetHeatRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "cool":
		params, err := readCommandParams[sdm.ThermostatTemperatureSetpointSetCoolParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetCool,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.SetCool(ctx, sdm.SetCoolRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "range":
		params, err := readCommandParams[sdm.ThermostatTemperatureSetpointSetRangeParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetRange,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.SetRange(ctx, sdm.SetRangeRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	default:
		return nil, errThermostatOperation
	}
}
