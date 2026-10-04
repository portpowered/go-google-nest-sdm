package sdm_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestDecodePreservesPresenceAndUnknownFields(t *testing.T) {
	t.Parallel()

	data := []byte(`{
  "name": "enterprises/example/devices/device",
  "future": 9007199254740993,
  "traits": {
    "sdm.devices.traits.CameraMotion": {},
    "sdm.devices.traits.ThermostatTemperatureSetpoint": {
      "heatCelsius": 0,
      "future": false
    },
    "sdm.devices.traits.ThermostatMode": {
      "mode": "FUTURE"
    },
    "future.trait": {
      "value": null
    }
  }
}`)

	device, err := sdm.DecodeDevice(data)
	if err != nil {
		t.Fatal(err)
	}

	if device.Traits.SdmDevicesTraitsCameraMotion == nil {
		t.Fatal("present empty capability disappeared")
	}

	setpoint := device.Traits.SdmDevicesTraitsThermostatTemperatureSetpoint
	if setpoint.HeatCelsius == nil || *setpoint.HeatCelsius != 0 || setpoint.CoolCelsius != nil {
		t.Fatal("zero value and absent setpoint conflated")
	}

	if string(device.AdditionalProperties["future"]) != "9007199254740993" {
		t.Fatal("unknown integer lost precision")
	}

	if *device.Traits.SdmDevicesTraitsThermostatMode.Mode != "FUTURE" {
		t.Fatal("open enum lost")
	}

	encoded, err := json.Marshal(device)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(encoded, []byte(`9007199254740993`)) || !bytes.Contains(encoded, []byte(`"value":null`)) {
		t.Fatal("round trip discarded unknown data")
	}

	if device.SupportsCommand(sdm.SdmDevicesCommandsCameraEventImageGenerateImage) {
		t.Fatal("device category invented capability")
	}
}

func TestDecodeRejectsMalformedKnownPayloads(t *testing.T) {
	t.Parallel()

	cases := []string{
		nullJSON, `[]`, `{}`,
		`{"name":null}`,
		`{"name":1}`,
		`{"name":"enterprises/example/devices/device","traits":null}`,
		`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.Fan": null
  }
}`,
		`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.CameraMotion": []
  }
}`,
		`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.Humidity": {
      "ambientHumidityPercent": "50"
    }
  }
}`,
		`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.ThermostatMode": {
      "mode": 1
    }
  }
}`,
		`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.Fan": {
      "timerTimeout": "invalid"
    }
  }
}`,
	}
	for _, data := range cases {
		_, err := sdm.DecodeDevice([]byte(data))

		var typed *sdm.Error

		if !errors.As(err, &typed) || typed.Kind != sdm.ErrorInvalidResponse {
			t.Errorf("accepted malformed payload %s: %v", data, err)
		}
	}
}

func TestDecodeAllTraitFamilies(t *testing.T) {
	t.Parallel()

	data := []byte(`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.Connectivity": {
      "status": "ONLINE"
    },
    "sdm.devices.traits.Fan": {
      "timerMode": "OFF",
      "timerTimeout": "2026-10-04T00:00:00Z"
    },
    "sdm.devices.traits.Humidity": {
      "ambientHumidityPercent": 50
    },
    "sdm.devices.traits.Info": {
      "customName": "Nest"
    },
    "sdm.devices.traits.Settings": {
      "temperatureScale": "CELSIUS"
    },
    "sdm.devices.traits.Temperature": {
      "ambientTemperatureCelsius": 20
    },
    "sdm.devices.traits.ThermostatEco": {
      "mode": "OFF",
      "heatCelsius": 10,
      "coolCelsius": 25
    },
    "sdm.devices.traits.ThermostatHvac": {
      "status": "OFF"
    },
    "sdm.devices.traits.ThermostatMode": {
      "mode": "HEAT",
      "availableModes": [
        "HEAT",
        "COOL"
      ]
    },
    "sdm.devices.traits.ThermostatTemperatureSetpoint": {
      "heatCelsius": 21
    },
    "sdm.devices.traits.CameraClipPreview": {},
    "sdm.devices.traits.CameraEventImage": {},
    "sdm.devices.traits.CameraImage": {
      "maxImageResolution": {
        "width": 640,
        "height": 480
      }
    },
    "sdm.devices.traits.CameraLiveStream": {
      "supportedProtocols": [
        "RTSP",
        "WEB_RTC"
      ],
      "audioCodecs": [
        "OPUS"
      ],
      "videoCodecs": [
        "H264"
      ],
      "maxVideoResolution": {
        "width": 1920,
        "height": 1080
      }
    },
    "sdm.devices.traits.CameraMotion": {},
    "sdm.devices.traits.CameraPerson": {},
    "sdm.devices.traits.CameraSound": {},
    "sdm.devices.traits.DoorbellChime": {},
    "sdm.structures.traits.Info": {
      "customName": "Home"
    },
    "sdm.structures.traits.RoomInfo": {
      "customName": "Room"
    }
  }
}`)

	device, err := sdm.DecodeDevice(data)
	if err != nil {
		t.Fatal(err)
	}

	for _, command := range allCommands() {
		if !device.SupportsCommand(command) {
			t.Errorf("missing capability for %s", command)
		}
	}

	if device.SupportsCommand("future.command") {
		t.Fatal("invented future command")
	}

	var empty sdm.Device

	if empty.SupportsCommand(sdm.SdmDevicesCommandsFanSetTimer) {
		t.Fatal("absent traits invented capability")
	}
}

func TestDecodeStructureAndRoom(t *testing.T) {
	t.Parallel()

	_, err := sdm.DecodeStructure([]byte(`{
  "name": "enterprises/example/structures/home",
  "traits": {
    "sdm.structures.traits.Info": {
      "customName": "Home"
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	_, err = sdm.DecodeRoom([]byte(`{
  "name": "enterprises/example/structures/home/rooms/room",
  "traits": {
    "sdm.structures.traits.RoomInfo": {
      "customName": "Room"
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
}

func allCommands() []sdm.CommandName {
	return []sdm.CommandName{
		sdm.SdmDevicesCommandsFanSetTimer,
		sdm.SdmDevicesCommandsThermostatEcoSetMode,
		sdm.SdmDevicesCommandsThermostatModeSetMode,
		sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetHeat,
		sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetCool,
		sdm.SdmDevicesCommandsThermostatTemperatureSetpointSetRange,
		sdm.SdmDevicesCommandsCameraEventImageGenerateImage,
		sdm.SdmDevicesCommandsCameraLiveStreamGenerateRtspStream,
		sdm.SdmDevicesCommandsCameraLiveStreamExtendRtspStream,
		sdm.SdmDevicesCommandsCameraLiveStreamStopRtspStream,
		sdm.SdmDevicesCommandsCameraLiveStreamGenerateWebRtcStream,
		sdm.SdmDevicesCommandsCameraLiveStreamExtendWebRtcStream,
		sdm.SdmDevicesCommandsCameraLiveStreamStopWebRtcStream,
	}
}
