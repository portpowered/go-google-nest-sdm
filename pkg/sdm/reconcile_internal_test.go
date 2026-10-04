package sdm

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestMergeTraitsRejectsInvalidSnapshotAndChanges(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		prior   *Traits
		changes *Traits
		message string
	}{
		{"invalid prior JSON", malformedTraits(), new(Traits), "read prior traits"},
		{"invalid changed JSON", nil, malformedTraits(), "read changed traits"},
		{"invalid known trait", nil, invalidKnownTrait(), "merge traits"},
		{"invalid prior known trait", invalidKnownTrait(), new(Traits), "merge traits"},
		{"infinite prior number", humidityTraits(math.Inf(1)), new(Traits), "read prior traits"},
		{"infinite changed number", nil, humidityTraits(math.Inf(1)), "read changed traits"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var device Device

			device.Traits = test.prior

			err := mergeTraits(&device, test.changes)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected %s error, got %v", test.message, err)
			}
		})
	}
}

func malformedTraits() *Traits {
	traits := new(Traits)
	traits.AdditionalProperties = map[string]json.RawMessage{"caller.extension": json.RawMessage("invalid")}

	return traits
}

func humidityTraits(value float64) *Traits {
	traits := new(Traits)
	traits.SdmDevicesTraitsHumidity = &Humidity{AmbientHumidityPercent: &value, AdditionalProperties: nil}

	return traits
}

func invalidKnownTrait() *Traits {
	traits := new(Traits)
	traits.AdditionalProperties = map[string]json.RawMessage{
		"sdm.devices.traits.Temperature": json.RawMessage(`null`),
	}

	return traits
}

func TestMergeTraitsRejectsUnrepresentableKnownNumber(t *testing.T) {
	t.Parallel()

	var device Device

	changes := new(Traits)
	changes.AdditionalProperties = map[string]json.RawMessage{
		"sdm.devices.traits.Temperature": json.RawMessage(`{"ambientTemperatureCelsius":1e400}`),
	}

	err := mergeTraits(&device, changes)
	if err == nil || !strings.Contains(err.Error(), "merge traits") {
		t.Fatalf("unrepresentable known number accepted: %v", err)
	}
}
