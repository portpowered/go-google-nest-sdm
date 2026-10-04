package sdm

import (
	"errors"
	"slices"
)

var (
	errRequiredTraitOrStreamProtocol = errors.New("required trait or stream protocol absent")
	errSetpointsCannotChangeInManual = errors.New("setpoints cannot change in manual Eco mode")
	errSetpointCommandDoesNotMatch   = errors.New("setpoint command does not match current thermostat mode")
)

var (
	errCapabilityAbsent = errRequiredTraitOrStreamProtocol
	errManualEco        = errSetpointsCannotChangeInManual
	errSetpointMode     = errSetpointCommandDoesNotMatch
)

// SupportsCommand derives capabilities from returned traits and stream protocols.
// It never infers a capability from the device category or current operating state.
func (device Device) SupportsCommand(command CommandName) bool {
	traits := device.Traits
	if traits == nil {
		return false
	}

	switch command {
	case SdmDevicesCommandsFanSetTimer:
		return traits.SdmDevicesTraitsFan != nil
	case SdmDevicesCommandsThermostatEcoSetMode:
		return traits.SdmDevicesTraitsThermostatEco != nil
	case SdmDevicesCommandsThermostatModeSetMode:
		return traits.SdmDevicesTraitsThermostatMode != nil
	case SdmDevicesCommandsThermostatTemperatureSetpointSetHeat,
		SdmDevicesCommandsThermostatTemperatureSetpointSetCool,
		SdmDevicesCommandsThermostatTemperatureSetpointSetRange:
		return traits.SdmDevicesTraitsThermostatTemperatureSetpoint != nil
	case SdmDevicesCommandsCameraEventImageGenerateImage:
		return traits.SdmDevicesTraitsCameraEventImage != nil
	case SdmDevicesCommandsCameraLiveStreamGenerateRtspStream,
		SdmDevicesCommandsCameraLiveStreamExtendRtspStream,
		SdmDevicesCommandsCameraLiveStreamStopRtspStream:
		return supportsProtocol(traits, StreamProtocolRTSP)
	case SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream,
		SdmDevicesCommandsCameraLiveStreamExtendWebRtcStream,
		SdmDevicesCommandsCameraLiveStreamStopWebRtcStream:
		return supportsProtocol(traits, StreamProtocolWEBRTC)
	}

	return false
}

func supportsProtocol(traits *Traits, protocol StreamProtocol) bool {
	stream := traits.SdmDevicesTraitsCameraLiveStream

	return stream != nil && stream.SupportedProtocols != nil && slices.Contains(*stream.SupportedProtocols, protocol)
}

// CheckCommand checks capabilities and known thermostat preconditions in a caller
// supplied snapshot. It performs no network I/O; the service remains authoritative
// because device state can change after this optional preflight check.
func CheckCommand(request CheckCommandRequest) error {
	if !request.Device.SupportsCommand(request.Command) {
		return &Error{Kind: ErrorUnsupported, Operation: "preflight", Cause: errCapabilityAbsent, StatusCode: 0}
	}

	expectedModes := map[CommandName]ThermostatModeValue{
		SdmDevicesCommandsThermostatTemperatureSetpointSetHeat:  ThermostatModeValueHEAT,
		SdmDevicesCommandsThermostatTemperatureSetpointSetCool:  ThermostatModeValueCOOL,
		SdmDevicesCommandsThermostatTemperatureSetpointSetRange: ThermostatModeValueHEATCOOL,
	}

	expected, isSetpoint := expectedModes[request.Command]
	if !isSetpoint {
		return nil
	}

	traits := request.Device.Traits
	if eco := traits.SdmDevicesTraitsThermostatEco; eco != nil && eco.Mode != nil && *eco.Mode == EcoModeMANUALECO {
		return invalidCommand(errManualEco)
	}

	if mode := traits.SdmDevicesTraitsThermostatMode; mode != nil && mode.Mode != nil && *mode.Mode != expected {
		return invalidCommand(errSetpointMode)
	}

	return nil
}
