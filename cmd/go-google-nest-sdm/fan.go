package main

import (
	"context"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func (app application) fan(
	ctx context.Context,
	operation string,
	options invocation,
	auth sdm.AuthContext,
) (any, error) {
	switch operation {
	case "timer":
		params, err := readCommandParams[sdm.FanSetTimerParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsFanSetTimer,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.SetFanTimer(ctx, sdm.SetFanTimerRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	default:
		return nil, errFanOperation
	}
}
