package sdm_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestCommandValidation(t *testing.T) {
	t.Parallel()
	valid := []string{`{"timerMode":"ON","duration":"900s"}`, `{"mode":"OFF"}`, `{"mode":"HEAT"}`, `{"heatCelsius":20}`, `{"coolCelsius":22}`, `{"heatCelsius":20,"coolCelsius":22}`, `{"eventId":"event"}`, `{}`, `{"streamExtensionToken":"token"}`, `{"streamExtensionToken":"token"}`, `{"offerSdp":"v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=mid:0\r\na=recvonly\r\na=rtpmap:111 opus/48000/2\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\na=mid:1\r\nm=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\na=mid:2\r\n"}`, `{"mediaSessionId":"session"}`, `{"mediaSessionId":"session"}`}
	for index, command := range allCommands() {
		if err := sdm.ValidateCommandParams(command, json.RawMessage(valid[index])); err != nil {
			t.Errorf("valid %s: %v", command, err)
		}
		for _, bad := range []string{`null`, `[]`, `{"unknown":true}`} {
			if err := sdm.ValidateCommandParams(command, json.RawMessage(bad)); err == nil {
				t.Errorf("%s accepted %s", command, bad)
			}
		}
	}
	cases := []struct {
		command sdm.CommandName
		data    string
	}{
		{sdm.SdmDevicesCommandsFanSetTimer, `{"timerMode":"FUTURE"}`},
		{sdm.SdmDevicesCommandsFanSetTimer, `{"timerMode":"ON","duration":"0s"}`},
		{sdm.SdmDevicesCommandsFanSetTimer, `{"timerMode":"ON","duration":"43201s"}`},
		{sdm.SdmDevicesCommandsFanSetTimer, `{"timerMode":"ON","duration":"1m"}`},
		{sdm.SdmDevicesCommandsThermostatEcoSetMode, `{"mode":"FUTURE"}`},
		{sdm.SdmDevicesCommandsThermostatModeSetMode, `{"mode":"FUTURE"}`},
		{sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetRange, `{"heatCelsius":22,"coolCelsius":20}`},
		{sdm.SdmDevicesCommandsCameraEventImageGenerateImage, `{"eventId":""}`},
		{sdm.SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream, `{"offerSdp":"v=0"}`},
		{sdm.SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream, `{"offerSdp":"invalid"}`},
		{"future.command", `{}`},
	}
	for _, test := range cases {
		var typed *sdm.Error
		err := sdm.ValidateCommandParams(test.command, json.RawMessage(test.data))
		if !errors.As(err, &typed) || typed.Kind != sdm.ErrorInvalidRequest {
			t.Errorf("invalid %s accepted %s", test.command, test.data)
		}
	}
}

func TestCommandResultValidation(t *testing.T) {
	t.Parallel()
	valid := []string{`{}`, `{}`, `{}`, `{}`, `{}`, `{}`, `{"url":"https://example.com/image","token":"token"}`, `{"streamUrls":{"rtspUrl":"rtsps://example.com/live"},"streamToken":"token","streamExtensionToken":"extension","expiresAt":"2026-10-04T00:00:00Z"}`, `{"streamToken":"token","streamExtensionToken":"extension","expiresAt":"2026-10-04T00:00:00Z"}`, `{}`, `{"answerSdp":"v=0\n","mediaSessionId":"session","expiresAt":"2026-10-04T00:00:00Z"}`, `{"mediaSessionId":"session","expiresAt":"2026-10-04T00:00:00Z"}`, `{}`}
	for index, command := range allCommands() {
		if err := sdm.ValidateCommandResults(command, json.RawMessage(valid[index])); err != nil {
			t.Errorf("valid %s: %v", command, err)
		}
		if err := sdm.ValidateCommandResults(command, json.RawMessage(`null`)); err == nil {
			t.Errorf("accepted null %s", command)
		}
	}
	if err := sdm.ValidateCommandResults(sdm.SdmDevicesCommandsCameraEventImageGenerateImage, json.RawMessage(`{}`)); err == nil {
		t.Fatal("accepted missing image results")
	}
	if err := sdm.ValidateCommandResults("future.command", json.RawMessage(`{}`)); err == nil {
		t.Fatal("accepted unknown command")
	}
}

func TestPreflightUsesCapabilitiesAndKnownState(t *testing.T) {
	t.Parallel()
	device, err := sdm.DecodeDevice([]byte(`{"name":"enterprises/example/devices/device","traits":{"sdm.devices.traits.ThermostatTemperatureSetpoint":{},"sdm.devices.traits.ThermostatMode":{"mode":"HEAT"},"sdm.devices.traits.ThermostatEco":{"mode":"OFF"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	request := sdm.CheckCommandRequest{Device: device, Command: sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat}
	if err := sdm.CheckCommand(request); err != nil {
		t.Fatal(err)
	}
	request.Command = sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetCool
	if err := sdm.CheckCommand(request); err == nil {
		t.Fatal("wrong mode allowed")
	}
	eco := sdm.EcoModeMANUALECO
	device.Traits.SdmDevicesTraitsThermostatEco.Mode = &eco
	request.Device = device
	request.Command = sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat
	if err := sdm.CheckCommand(request); err == nil {
		t.Fatal("manual Eco allowed")
	}
	request.Command = sdm.SdmDevicesCommandsFanSetTimer
	if err := sdm.CheckCommand(request); err == nil {
		t.Fatal("missing capability allowed")
	}
}
