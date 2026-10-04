package sdm

import (
	"encoding/json"
	"reflect"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
)

// DecodeDevice validates a snapshot and preserves unknown resource and trait fields.
func DecodeDevice(data []byte) (Device, error) { return decode[Device](data) }

// DecodeStructure validates a structure snapshot without discarding unknown fields.
func DecodeStructure(data []byte) (Structure, error) { return decode[Structure](data) }

// DecodeRoom validates a room snapshot without discarding unknown fields.
func DecodeRoom(data []byte) (Room, error) { return decode[Room](data) }

// DecodeEvent validates the SDM event, including each known trait and camera event.
// Unknown event and trait names remain available in generated AdditionalProperties.
func DecodeEvent(data []byte) (EventEnvelope, error) { return decode[EventEnvelope](data) }

func decode[T any](data []byte) (T, error) {
	var result T
	document := "client-resources.openapi.yaml"
	if reflect.TypeFor[T]().Name() == "EventEnvelope" {
		document = "events.openapi.yaml"
	}
	if err := contracts.Validate(document, reflect.TypeFor[T]().Name(), data); err != nil {
		return result, invalidResponse("decode", err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, invalidResponse("decode", err)
	}
	return result, nil
}
