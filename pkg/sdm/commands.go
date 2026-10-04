package sdm

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
)

var (
	errUnsupportedCommand             = errors.New("unsupported command")
	errHeatSetpointMustBeBelow        = errors.New("heat setpoint must be below cool setpoint")
	errSdpOfferMustEndWith            = errors.New("SDP offer must end with a newline")
	errSdpOfferRequiresVersionZero    = errors.New("SDP offer requires version zero")
	errSdpMediaOrderMustBe            = errors.New("SDP media order must be audio, video, application")
	errSdpUnifiedPlanRequiresDistinct = errors.New("SDP Unified Plan requires distinct media identifiers")
	errSdpAudioMustBeReceive          = errors.New("SDP audio must be receive only")
	errSdpAudioCodecMappingIs         = errors.New("SDP audio codec mapping is malformed")
	errSdpAudioSupportsOnlyOpus       = errors.New("SDP audio supports only Opus")
	errSdpOfferRequiresThreeMedia     = errors.New("SDP offer requires three media sections and receive-only Opus audio")
	errSdpUnifiedPlanRequiresOne      = errors.New("SDP Unified Plan requires one media identifier per section")
	errRequiredCommandValueMustNot    = errors.New("required command value must not be empty")
)

// ValidateCommandParams checks generated command fields and SDM semantic constraints.
// It rejects unknown outbound fields; future inbound values remain open.
func ValidateCommandParams(command CommandName, data json.RawMessage) error {
	params, _, found := commandModels(command)
	if !found {
		return invalidCommand(errUnsupportedCommand)
	}

	component := params.Name()
	if command == SdmDevicesCommandsCameraLiveStreamGenerateRtspStream {
		component = "CameraLiveStreamGenerateRtspStreamParams"
	}
	err := contracts.Validate("client-models.openapi.yaml", component, data)

	if err != nil {
		return invalidCommand(err)
	}

	value := reflect.New(params)
	err = json.Unmarshal(data, value.Interface())
	if err != nil {
		return invalidCommand(err)
	}
	err = validateCommandSemantics(value.Elem().Interface())

	if err != nil {
		return invalidCommand(err)
	}

	return nil
}

// ValidateCommandResults validates the response shape for the command that was sent.
// The service does not repeat the command name in its response envelope.
func ValidateCommandResults(command CommandName, data json.RawMessage) error {
	_, results, found := commandModels(command)
	if !found {
		return invalidResponse("command", errUnsupportedCommand)
	}
	err := contracts.Validate("client-models.openapi.yaml", results.Name(), data)

	if err != nil {
		return invalidResponse("command", err)
	}

	return nil
}

func invalidCommand(cause error) error {
	return &Error{Kind: ErrorInvalidRequest, Operation: "command", Cause: cause, StatusCode: 0}
}

func validateCommandSemantics(params any) error {
	switch params := params.(type) {
	case ThermostatTemperatureSetpointSetRangeParams:
		if params.HeatCelsius >= params.CoolCelsius {
			return errHeatSetpointMustBeBelow
		}
	case CameraLiveStreamGenerateWebRtcStreamParams:
		return validateSDPOffer(params.OfferSdp)
	}

	return rejectEmptyRequiredStrings(params)
}

func validateSDPOffer(offer string) error {
	if !strings.HasSuffix(offer, protocol.SDPLineSeparator) {
		return errSdpOfferMustEndWith
	}

	lines := strings.Split(strings.ReplaceAll(offer, protocol.SDPCarriageReturn, ""), protocol.SDPLineSeparator)
	if lines[0] != protocol.SDPSessionVersion {
		return errSdpOfferRequiresVersionZero
	}

	expected := []string{protocol.SDPAudioPrefix, protocol.SDPVideoPrefix, protocol.SDPApplicationPrefix}
	media := splitSDPMedia(lines)
	if len(media) != len(expected) {
		return errSdpOfferRequiresThreeMedia
	}

	mids := make(map[string]bool)
	for index, section := range media {
		if !strings.HasPrefix(section[0], expected[index]) {
			return errSdpMediaOrderMustBe
		}
		err := validateSDPMid(section, mids)
		if err != nil {
			return err
		}
	}

	return validateSDPAudio(media[0])
}

func splitSDPMedia(lines []string) [][]string {
	var media [][]string
	for _, line := range lines {
		if strings.HasPrefix(line, protocol.SDPMediaPrefix) {
			media = append(media, []string{line})
		} else if len(media) > 0 {
			index := len(media) - 1
			media[index] = append(media[index], line)
		}
	}
	return media
}

func validateSDPMid(lines []string, mids map[string]bool) error {
	count := 0
	for _, line := range lines {
		if !strings.HasPrefix(line, protocol.SDPMidPrefix) {
			continue
		}
		mid := strings.TrimPrefix(line, protocol.SDPMidPrefix)
		if mid == "" || mids[mid] {
			return errSdpUnifiedPlanRequiresDistinct
		}
		mids[mid] = true
		count++
	}
	if count != 1 {
		return errSdpUnifiedPlanRequiresOne
	}
	return nil
}

func validateSDPAudio(lines []string) error {
	receiveOnly, opus := false, false
	for _, line := range lines {

		if line == protocol.SDPReceiveOnlyLine {
			receiveOnly = true
		}

		if line == protocol.SDPDirectionSendReceiveLine ||
			line == protocol.SDPSendOnlyLine || line == protocol.SDPInactiveLine {
			return errSdpAudioMustBeReceive
		}

		if strings.HasPrefix(line, protocol.SDPRtpMapPrefix) {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				return errSdpAudioCodecMappingIs
			}

			codec := strings.ToLower(fields[1])
			if codec != protocol.SDPOpusCodec && !strings.HasPrefix(codec, protocol.SDPOpusCodec+"/") {
				return errSdpAudioSupportsOnlyOpus
			}

			opus = true
		}
	}

	if !receiveOnly || !opus {
		return errSdpOfferRequiresThreeMedia
	}

	return nil
}

func rejectEmptyRequiredStrings(params any) error {
	value := reflect.ValueOf(params)
	if value.Kind() != reflect.Struct {
		return nil
	}

	for index := range value.NumField() {
		field := value.Field(index)
		if field.Kind() == reflect.String && strings.TrimSpace(field.String()) == "" {
			return errRequiredCommandValueMustNot
		}
	}

	return nil
}

func commandModels(command CommandName) (reflect.Type, reflect.Type, bool) {
	switch command {
	case SdmDevicesCommandsFanSetTimer:
		return reflect.TypeFor[FanSetTimerParams](),
			reflect.TypeFor[FanSetTimerResults](),
			true
	case SdmDevicesCommandsThermostatEcoSetMode:
		return reflect.TypeFor[ThermostatEcoSetModeParams](),
			reflect.TypeFor[ThermostatEcoSetModeResults](),
			true
	case SdmDevicesCommandsThermostatModeSetMode:
		return reflect.TypeFor[ThermostatModeSetModeParams](),
			reflect.TypeFor[ThermostatModeSetModeResults](),
			true
	case SdmDevicesCommandsThermostatTemperatureSetpointSetHeat:
		return reflect.TypeFor[ThermostatTemperatureSetpointSetHeatParams](),
			reflect.TypeFor[ThermostatTemperatureSetpointSetHeatResults](),
			true
	case SdmDevicesCommandsThermostatTemperatureSetpointSetCool:
		return reflect.TypeFor[ThermostatTemperatureSetpointSetCoolParams](),
			reflect.TypeFor[ThermostatTemperatureSetpointSetCoolResults](),
			true
	case SdmDevicesCommandsThermostatTemperatureSetpointSetRange:
		return reflect.TypeFor[ThermostatTemperatureSetpointSetRangeParams](),
			reflect.TypeFor[ThermostatTemperatureSetpointSetRangeResults](),
			true
	case SdmDevicesCommandsCameraEventImageGenerateImage:
		return reflect.TypeFor[CameraEventImageGenerateImageParams](),
			reflect.TypeFor[CameraEventImageGenerateImageResults](),
			true
	case SdmDevicesCommandsCameraLiveStreamGenerateRtspStream:
		return reflect.TypeFor[CameraLiveStreamGenerateRtspStreamParams](),
			reflect.TypeFor[CameraLiveStreamGenerateRtspStreamResults](),
			true
	case SdmDevicesCommandsCameraLiveStreamExtendRtspStream:
		return reflect.TypeFor[CameraLiveStreamExtendRtspStreamParams](),
			reflect.TypeFor[CameraLiveStreamExtendRtspStreamResults](),
			true
	case SdmDevicesCommandsCameraLiveStreamStopRtspStream:
		return reflect.TypeFor[CameraLiveStreamStopRtspStreamParams](),
			reflect.TypeFor[CameraLiveStreamStopRtspStreamResults](),
			true
	case SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream:
		return reflect.TypeFor[CameraLiveStreamGenerateWebRtcStreamParams](),
			reflect.TypeFor[CameraLiveStreamGenerateWebRtcStreamResults](),
			true
	case SdmDevicesCommandsCameraLiveStreamExtendWebRtcStream:
		return reflect.TypeFor[CameraLiveStreamExtendWebRtcStreamParams](),
			reflect.TypeFor[CameraLiveStreamExtendWebRtcStreamResults](),
			true
	case SdmDevicesCommandsCameraLiveStreamStopWebRtcStream:
		return reflect.TypeFor[CameraLiveStreamStopWebRtcStreamParams](),
			reflect.TypeFor[CameraLiveStreamStopWebRtcStreamResults](),
			true
	}

	return nil, nil, false
}
