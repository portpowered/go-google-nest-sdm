package sdm_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestCommandValidation(t *testing.T) {
	t.Parallel()

	valid := []string{
		`{"timerMode":"ON","duration":"900s"}`,
		`{"mode":"OFF"}`,
		`{"mode":"HEAT"}`,
		`{"heatCelsius":20}`,
		`{"coolCelsius":22}`,
		`{"heatCelsius":20,"coolCelsius":22}`,
		`{"eventId":"event"}`,
		`{}`,
		`{"streamExtensionToken":"token"}`,
		`{"streamExtensionToken":"token"}`,
		offerParamsJSON(t, validSyntheticOffer),
		`{"mediaSessionId":"session"}`,
		`{"mediaSessionId":"session"}`}
	for index, command := range allCommands() {
		err := sdm.ValidateCommandParams(command, json.RawMessage(valid[index]))
		if err != nil {
			t.Errorf("valid %s: %v", command, err)
		}

		for _, bad := range []string{nullJSON, `[]`, `{"unknown":true}`} {
			err := sdm.ValidateCommandParams(command, json.RawMessage(bad))
			if err == nil {
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

	valid := []string{
		`{}`,
		`{}`,
		`{}`,
		`{}`,
		`{}`,
		`{}`,
		`{"url":"https://example.com/image","token":"token"}`,
		`{
  "streamUrls": {
    "rtspUrl": "rtsps://example.com/live"
  },
  "streamToken": "token",
  "streamExtensionToken": "extension",
  "expiresAt": "2026-10-04T00:00:00Z"
}`,
		`{
  "streamToken": "token",
  "streamExtensionToken": "extension",
  "expiresAt": "2026-10-04T00:00:00Z"
}`,
		`{}`,
		`{"answerSdp":"v=0\n","mediaSessionId":"session","expiresAt":"2026-10-04T00:00:00Z"}`,
		`{"mediaSessionId":"session","expiresAt":"2026-10-04T00:00:00Z"}`,
		`{}`}
	for index, command := range allCommands() {
		err := sdm.ValidateCommandResults(command, json.RawMessage(valid[index]))
		if err != nil {
			t.Errorf("valid %s: %v", command, err)
		}

		err = sdm.ValidateCommandResults(command, json.RawMessage(`null`))
		if err == nil {
			t.Errorf("accepted null %s", command)
		}
	}

	err := sdm.ValidateCommandResults(sdm.SdmDevicesCommandsCameraEventImageGenerateImage, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("accepted missing image results")
	}

	err = sdm.ValidateCommandResults("future.command", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("accepted unknown command")
	}
}

func TestPreflightUsesCapabilitiesAndKnownState(t *testing.T) {
	t.Parallel()

	device, err := sdm.DecodeDevice([]byte(`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.ThermostatTemperatureSetpoint": {},
    "sdm.devices.traits.ThermostatMode": {
      "mode": "HEAT"
    },
    "sdm.devices.traits.ThermostatEco": {
      "mode": "OFF"
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	request := sdm.CheckCommandRequest{Device: device, Command: sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat}

	err = sdm.CheckCommand(request)
	if err != nil {
		t.Fatal(err)
	}

	request.Command = sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetCool

	err = sdm.CheckCommand(request)
	if err == nil {
		t.Fatal("wrong mode allowed")
	}

	eco := sdm.EcoModeMANUALECO
	device.Traits.SdmDevicesTraitsThermostatEco.Mode = &eco
	request.Device = device

	request.Command = sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat

	err = sdm.CheckCommand(request)
	if err == nil {
		t.Fatal("manual Eco allowed")
	}

	request.Command = sdm.SdmDevicesCommandsFanSetTimer

	err = sdm.CheckCommand(request)
	if err == nil {
		t.Fatal("missing capability allowed")
	}
}

// validSyntheticOffer exercises the documented media shape without a live session.
const validSyntheticOffer = "v=0\r\n" +
	"m=audio 9 UDP/TLS/RTP/SAVPF 111\r\n" +
	"a=mid:0\r\n" +
	"a=recvonly\r\n" +
	"a=rtpmap:111 opus/48000/2\r\n" +
	"m=video 9 UDP/TLS/RTP/SAVPF 96\r\n" +
	"a=mid:1\r\n" +
	"m=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\n" +
	"a=mid:2\r\n"

func offerParamsJSON(t *testing.T, offer string) string {
	t.Helper()

	data, err := json.Marshal(sdm.CameraLiveStreamGenerateWebRtcStreamParams{OfferSdp: offer})
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func TestSDPOfferContract(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing newline":      strings.TrimSuffix(validSyntheticOffer, "\r\n"),
		"wrong media order":    strings.ReplaceAll(validSyntheticOffer, "m=audio", "m=video"),
		"sending audio":        strings.ReplaceAll(validSyntheticOffer, "a=recvonly", "a=sendrecv"),
		"missing direction":    strings.ReplaceAll(validSyntheticOffer, "a=recvonly\r\n", ""),
		"non Opus audio":       strings.ReplaceAll(validSyntheticOffer, "opus/48000/2", "PCMU/8000"),
		"missing codec":        strings.ReplaceAll(validSyntheticOffer, "a=rtpmap:111 opus/48000/2\r\n", ""),
		"duplicate mid":        strings.ReplaceAll(validSyntheticOffer, "a=mid:1", "a=mid:0"),
		"missing mid":          strings.ReplaceAll(validSyntheticOffer, "a=mid:1\r\n", ""),
		"missing data channel": strings.Split(validSyntheticOffer, "m=application")[0],
	}

	for name, offer := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := sdm.ValidateCommandParams(sdm.SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream,
				json.RawMessage(offerParamsJSON(t, offer)))

			var typed *sdm.Error

			if !errors.As(err, &typed) || typed.Kind != sdm.ErrorInvalidRequest {
				t.Fatalf("accepted malformed SDP: %v", err)
			}
		})
	}
}
