package main

import (
	"context"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func (app application) camera(
	ctx context.Context,
	operation string,
	options invocation,
	auth sdm.AuthContext,
) (any, error) {
	switch operation {
	case "image":
		params, err := readCommandParams[sdm.CameraEventImageGenerateImageParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsCameraEventImageGenerateImage,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.GenerateImage(ctx, sdm.GenerateImageRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "rtsp-start":
		params, err := readCommandParams[sdm.CameraLiveStreamGenerateRtspStreamParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsCameraLiveStreamGenerateRtspStream,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.GenerateRtspStream(ctx, sdm.GenerateRtspStreamRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "rtsp-extend":
		params, err := readCommandParams[sdm.CameraLiveStreamExtendRtspStreamParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsCameraLiveStreamExtendRtspStream,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.ExtendRtspStream(ctx, sdm.ExtendRtspStreamRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "rtsp-stop":
		params, err := readCommandParams[sdm.CameraLiveStreamStopRtspStreamParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsCameraLiveStreamStopRtspStream,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.StopRtspStream(ctx, sdm.StopRtspStreamRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "webrtc-start":
		params, err := readCommandParams[sdm.CameraLiveStreamGenerateWebRtcStreamParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.GenerateWebRtcStream(ctx, sdm.GenerateWebRtcStreamRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "webrtc-extend":
		params, err := readCommandParams[sdm.CameraLiveStreamExtendWebRtcStreamParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsCameraLiveStreamExtendWebRtcStream,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.ExtendWebRtcStream(ctx, sdm.ExtendWebRtcStreamRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	case "webrtc-stop":
		params, err := readCommandParams[sdm.CameraLiveStreamStopWebRtcStreamParams](
			options.paramsPath,
			app.in,
			sdm.SdmDevicesCommandsCameraLiveStreamStopWebRtcStream,
		)
		if err != nil {
			return nil, wrapError(err)
		}

		return sdkResult(app.client.StopWebRtcStream(ctx, sdm.StopWebRtcStreamRequest{
			Auth:       auth,
			DeviceName: options.resource,
			Params:     params,
		}))
	default:
		return nil, errCameraOperation
	}
}
