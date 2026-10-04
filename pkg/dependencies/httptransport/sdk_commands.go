package httptransport

import (
	"context"
	"encoding/json"

	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func (client *sdkClient) SetFanTimer(
	ctx context.Context,
	request sdm.SetFanTimerRequest,
) (sdm.SetFanTimerResult, error) {
	results,
		err := executeTyped[sdm.EmptyResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsFanSetTimer,
		request.Params,
	)
	if err != nil {
		return sdm.SetFanTimerResult{}, err
	}

	return sdm.SetFanTimerResult{Results: results}, nil
}
func (client *sdkClient) SetThermostatEcoMode(
	ctx context.Context,
	request sdm.SetThermostatEcoModeRequest,
) (sdm.SetThermostatEcoModeResult, error) {
	results,
		err := executeTyped[sdm.EmptyResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsThermostatEcoSetMode,
		request.Params,
	)
	if err != nil {
		return sdm.SetThermostatEcoModeResult{}, err
	}

	return sdm.SetThermostatEcoModeResult{Results: results}, nil
}
func (client *sdkClient) SetThermostatMode(
	ctx context.Context,
	request sdm.SetThermostatModeRequest,
) (sdm.SetThermostatModeResult, error) {
	results,
		err := executeTyped[sdm.EmptyResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsThermostatModeSetMode,
		request.Params,
	)
	if err != nil {
		return sdm.SetThermostatModeResult{}, err
	}

	return sdm.SetThermostatModeResult{Results: results}, nil
}
func (client *sdkClient) SetHeat(ctx context.Context, request sdm.SetHeatRequest) (sdm.SetHeatResult, error) {
	results,
		err := executeTyped[sdm.EmptyResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat,
		request.Params,
	)
	if err != nil {
		return sdm.SetHeatResult{}, err
	}

	return sdm.SetHeatResult{Results: results}, nil
}
func (client *sdkClient) SetCool(ctx context.Context, request sdm.SetCoolRequest) (sdm.SetCoolResult, error) {
	results,
		err := executeTyped[sdm.EmptyResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetCool,
		request.Params,
	)
	if err != nil {
		return sdm.SetCoolResult{}, err
	}

	return sdm.SetCoolResult{Results: results}, nil
}
func (client *sdkClient) SetRange(ctx context.Context, request sdm.SetRangeRequest) (sdm.SetRangeResult, error) {
	results,
		err := executeTyped[sdm.EmptyResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetRange,
		request.Params,
	)
	if err != nil {
		return sdm.SetRangeResult{}, err
	}

	return sdm.SetRangeResult{Results: results}, nil
}
func (client *sdkClient) GenerateImage(
	ctx context.Context,
	request sdm.GenerateImageRequest,
) (sdm.GenerateImageResult, error) {
	results,
		err := executeTyped[sdm.CameraEventImageGenerateImageResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsCameraEventImageGenerateImage,
		request.Params,
	)
	if err != nil {
		return sdm.GenerateImageResult{}, err
	}

	return sdm.GenerateImageResult{Results: results}, nil
}
func (client *sdkClient) GenerateRtspStream(
	ctx context.Context,
	request sdm.GenerateRtspStreamRequest,
) (sdm.GenerateRtspStreamResult, error) {
	results,
		err := executeTyped[sdm.CameraLiveStreamGenerateRtspStreamResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsCameraLiveStreamGenerateRtspStream,
		request.Params,
	)
	if err != nil {
		return sdm.GenerateRtspStreamResult{}, err
	}

	return sdm.GenerateRtspStreamResult{Results: results}, nil
}
func (client *sdkClient) ExtendRtspStream(
	ctx context.Context,
	request sdm.ExtendRtspStreamRequest,
) (sdm.ExtendRtspStreamResult, error) {
	results,
		err := executeTyped[sdm.CameraLiveStreamExtendRtspStreamResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsCameraLiveStreamExtendRtspStream,
		request.Params,
	)
	if err != nil {
		return sdm.ExtendRtspStreamResult{}, err
	}

	return sdm.ExtendRtspStreamResult{Results: results}, nil
}
func (client *sdkClient) StopRtspStream(
	ctx context.Context,
	request sdm.StopRtspStreamRequest,
) (sdm.StopRtspStreamResult, error) {
	results,
		err := executeTyped[sdm.EmptyResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsCameraLiveStreamStopRtspStream,
		request.Params,
	)
	if err != nil {
		return sdm.StopRtspStreamResult{}, err
	}

	return sdm.StopRtspStreamResult{Results: results}, nil
}
func (client *sdkClient) GenerateWebRtcStream(
	ctx context.Context,
	request sdm.GenerateWebRtcStreamRequest,
) (sdm.GenerateWebRtcStreamResult, error) {
	results,
		err := executeTyped[sdm.CameraLiveStreamGenerateWebRtcStreamResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream,
		request.Params,
	)
	if err != nil {
		return sdm.GenerateWebRtcStreamResult{}, err
	}

	return sdm.GenerateWebRtcStreamResult{Results: results}, nil
}
func (client *sdkClient) ExtendWebRtcStream(
	ctx context.Context,
	request sdm.ExtendWebRtcStreamRequest,
) (sdm.ExtendWebRtcStreamResult, error) {
	results,
		err := executeTyped[sdm.CameraLiveStreamExtendWebRtcStreamResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsCameraLiveStreamExtendWebRtcStream,
		request.Params,
	)
	if err != nil {
		return sdm.ExtendWebRtcStreamResult{}, err
	}

	return sdm.ExtendWebRtcStreamResult{Results: results}, nil
}
func (client *sdkClient) StopWebRtcStream(
	ctx context.Context,
	request sdm.StopWebRtcStreamRequest,
) (sdm.StopWebRtcStreamResult, error) {
	results,
		err := executeTyped[sdm.EmptyResults](
		ctx,
		client,
		request.Auth,
		request.DeviceName,
		sdm.SdmDevicesCommandsCameraLiveStreamStopWebRtcStream,
		request.Params,
	)
	if err != nil {
		return sdm.StopWebRtcStreamResult{}, err
	}

	return sdm.StopWebRtcStreamResult{Results: results}, nil
}

//nolint:ireturn // Generated result models retain their distinct public identities.
func executeTyped[Result any](
	ctx context.Context,
	client *sdkClient,
	auth sdm.AuthContext,
	name string,
	command sdm.CommandName,
	params any,
) (Result, error) {
	var result Result

	encoded, err := json.Marshal(params)
	if err == nil {
		err = sdm.ValidateCommandParams(command, encoded)
	}

	if err != nil {
		return result, &sdm.Error{Operation: string(command), Kind: sdm.ErrorInvalidRequest, StatusCode: 0, Cause: err}
	}

	response,
		err := client.transport.ExecuteCommand(
		ctx,
		auth.AccessToken,
		name,
		wire.ExecuteCommandRequest{
			Command: wire.CommandName(
				command,
			),
			Params: encoded,
		},
	)
	if err != nil {
		return result, publicError(err)
	}

	var payload json.RawMessage

	if response.Results == nil {
		if _, empty := any(result).(sdm.EmptyResults); !empty {
			return result, publicDecodeError(string(command), nil)
		}

		payload = json.RawMessage("{}")
	} else {
		payload = *response.Results
	}

	err = sdm.ValidateCommandResults(command, payload)
	if err == nil {
		err = json.Unmarshal(payload, &result)
	}

	if err != nil {
		return result, publicDecodeError(string(command), err)
	}

	return result, nil
}
