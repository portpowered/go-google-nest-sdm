package sdm

import (
	"encoding/json"
	"reflect"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
)

// DecodeDevice validates a snapshot and preserves unknown resource and trait fields.
func DecodeDevice(data []byte) (Device, error) {
	var result Device

	err := decode(data, &result)

	return result, err
}

// DecodeStructure validates a structure snapshot without discarding unknown fields.
func DecodeStructure(data []byte) (Structure, error) {
	var result Structure

	err := decode(data, &result)

	return result, err
}

// DecodeRoom validates a room snapshot without discarding unknown fields.
func DecodeRoom(data []byte) (Room, error) {
	var result Room

	err := decode(data, &result)

	return result, err
}

// DecodeEvent validates the SDM event, including each known trait and camera event.
// Unknown event and trait names remain available in generated AdditionalProperties.
func DecodeEvent(data []byte) (EventEnvelope, error) {
	var result EventEnvelope

	err := decode(data, &result)

	return result, err
}

func decode(data []byte, result any) error {
	model := reflect.TypeOf(result).Elem()

	document := "client-resources.openapi.yaml"

	if model.Name() == "EventEnvelope" {
		document = "events.openapi.yaml"
	}

	err := contracts.Validate(document, model.Name(), data)
	if err != nil {
		return invalidResponse("decode", err)
	}

	err = json.Unmarshal(data, result)
	if err != nil {
		return invalidResponse("decode", err)
	}

	return nil
}
