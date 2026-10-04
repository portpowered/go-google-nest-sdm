package sdm

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
)

// ValidateCommandParams checks generated command fields and SDM semantic constraints.
// It rejects unknown outbound fields; future inbound values remain open.
func ValidateCommandParams(command CommandName, data json.RawMessage) error {
	params, _, found := commandModels(command)
	if !found {
		return invalidCommand(errors.New("unsupported command"))
	}
	component := params.Name()
	if command == SdmDevicesCommandsCameraLiveStreamGenerateRtspStream {
		component = "CameraLiveStreamGenerateRtspStreamParams"
	}
	if err := contracts.Validate("client-models.openapi.yaml", component, data); err != nil {
		return invalidCommand(err)
	}
	value := reflect.New(params)
	if err := json.Unmarshal(data, value.Interface()); err != nil {
		return invalidCommand(err)
	}
	if err := validateCommandSemantics(value.Elem().Interface()); err != nil {
		return invalidCommand(err)
	}
	return nil
}

// ValidateCommandResults validates the response shape for the command that was sent.
// The service does not repeat the command name in its response envelope.
func ValidateCommandResults(command CommandName, data json.RawMessage) error {
	_, results, found := commandModels(command)
	if !found {
		return invalidResponse("command", errors.New("unsupported command"))
	}
	if err := contracts.Validate("client-models.openapi.yaml", results.Name(), data); err != nil {
		return invalidResponse("command", err)
	}
	return nil
}

func invalidCommand(cause error) error {
	return &Error{Kind: ErrorInvalidRequest, Operation: "command", Cause: cause}
}

func validateCommandSemantics(params any) error {
	switch params := params.(type) {
	case ThermostatTemperatureSetpointSetRangeParams:
		if params.HeatCelsius >= params.CoolCelsius {
			return errors.New("heat setpoint must be below cool setpoint")
		}
	case CameraLiveStreamGenerateWebRtcStreamParams:
		if !bytes.HasPrefix([]byte(params.OfferSdp), []byte("v=0")) {
			return errors.New("SDP offer must start with v=0")
		}
		if !strings.Contains(params.OfferSdp, "m=audio ") || !strings.Contains(params.OfferSdp, "m=video ") || !strings.Contains(params.OfferSdp, "m=application ") {
			return errors.New("SDP offer requires audio, video, and application sections")
		}
	}
	return rejectEmptyRequiredStrings(params)
}

func rejectEmptyRequiredStrings(params any) error {
	value := reflect.ValueOf(params)
	if value.Kind() != reflect.Struct {
		return nil
	}
	for index := range value.NumField() {
		field := value.Field(index)
		if field.Kind() == reflect.String && strings.TrimSpace(field.String()) == "" {
			return errors.New("required command value must not be empty")
		}
	}
	return nil
}

func commandModels(command CommandName) (reflect.Type, reflect.Type, bool) {
	switch command {
	case SdmDevicesCommandsFanSetTimer:
		return reflect.TypeFor[FanSetTimerParams](), reflect.TypeFor[FanSetTimerResults](), true
	case SdmDevicesCommandsThermostatEcoSetMode:
		return reflect.TypeFor[ThermostatEcoSetModeParams](), reflect.TypeFor[ThermostatEcoSetModeResults](), true
	case SdmDevicesCommandsThermostatModeSetMode:
		return reflect.TypeFor[ThermostatModeSetModeParams](), reflect.TypeFor[ThermostatModeSetModeResults](), true
	case SdmDevicesCommandsThermostatTemperatureSetpointSetHeat:
		return reflect.TypeFor[ThermostatTemperatureSetpointSetHeatParams](), reflect.TypeFor[ThermostatTemperatureSetpointSetHeatResults](), true
	case SdmDevicesCommandsThermostatTemperatureSetpointSetCool:
		return reflect.TypeFor[ThermostatTemperatureSetpointSetCoolParams](), reflect.TypeFor[ThermostatTemperatureSetpointSetCoolResults](), true
	case SdmDevicesCommandsThermostatTemperatureSetpointSetRange:
		return reflect.TypeFor[ThermostatTemperatureSetpointSetRangeParams](), reflect.TypeFor[ThermostatTemperatureSetpointSetRangeResults](), true
	case SdmDevicesCommandsCameraEventImageGenerateImage:
		return reflect.TypeFor[CameraEventImageGenerateImageParams](), reflect.TypeFor[CameraEventImageGenerateImageResults](), true
	case SdmDevicesCommandsCameraLiveStreamGenerateRtspStream:
		return reflect.TypeFor[CameraLiveStreamGenerateRtspStreamParams](), reflect.TypeFor[CameraLiveStreamGenerateRtspStreamResults](), true
	case SdmDevicesCommandsCameraLiveStreamExtendRtspStream:
		return reflect.TypeFor[CameraLiveStreamExtendRtspStreamParams](), reflect.TypeFor[CameraLiveStreamExtendRtspStreamResults](), true
	case SdmDevicesCommandsCameraLiveStreamStopRtspStream:
		return reflect.TypeFor[CameraLiveStreamStopRtspStreamParams](), reflect.TypeFor[CameraLiveStreamStopRtspStreamResults](), true
	case SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream:
		return reflect.TypeFor[CameraLiveStreamGenerateWebRtcStreamParams](), reflect.TypeFor[CameraLiveStreamGenerateWebRtcStreamResults](), true
	case SdmDevicesCommandsCameraLiveStreamExtendWebRtcStream:
		return reflect.TypeFor[CameraLiveStreamExtendWebRtcStreamParams](), reflect.TypeFor[CameraLiveStreamExtendWebRtcStreamResults](), true
	case SdmDevicesCommandsCameraLiveStreamStopWebRtcStream:
		return reflect.TypeFor[CameraLiveStreamStopWebRtcStreamParams](), reflect.TypeFor[CameraLiveStreamStopWebRtcStreamResults](), true
	}
	return nil, nil, false
}
