package sdm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
)

var (
	errEventIdentityUserAndTimestamp = errors.New("event identity, user, and timestamp are required")
)

// RecentEventLimit bounds redelivery history in each caller-owned device state.
// Deduplication outside this window remains the caller's persistence policy.
const RecentEventLimit = 256

// Reconcile applies an event to a caller-owned device snapshot without mutating
// the input. It binds the state to the first event user, rejects other accounts,
// ignores duplicates and strictly older timestamps, and merges partial trait
// fields. Equal timestamps follow delivery order. Unknown relation types leave
// the snapshot unchanged. The caller persists the returned state.
func Reconcile(request ReconcileRequest) (ReconcileResult, error) {
	state, err := cloneState(request.State)
	if err != nil {
		return ReconcileResult{}, invalidResponse("reconcile", err)
	}

	result := ReconcileResult{State: state, Disposition: ReconcileUnrelated}
	event := request.Event

	eventData, err := json.Marshal(event)
	if err != nil {
		return result, invalidResponse("reconcile", err)
	}

	err = contracts.Validate("events.openapi.yaml", "EventEnvelope", eventData)
	if err != nil {
		return result, invalidResponse("reconcile", err)
	}

	if event.EventId == "" || event.Timestamp.IsZero() || event.UserId == "" {
		return result, invalidResponse("reconcile", errEventIdentityUserAndTimestamp)
	}

	if state.UserId != nil && *state.UserId != event.UserId {
		return result, nil
	}

	if !targetsDevice(event, state.Device.Name) {
		return result, nil
	}

	if slices.Contains(state.RecentEventIds, event.EventId) {
		result.Disposition = ReconcileDuplicate

		return result, nil
	}

	if event.Timestamp.Before(state.UpdatedAt) {
		result.Disposition = ReconcileStale

		return result, nil
	}

	if event.ResourceUpdate != nil && event.ResourceUpdate.Name == state.Device.Name {
		err := mergeTraits(&result.State.Device, event.ResourceUpdate.Traits)
		if err != nil {
			return result, invalidResponse("reconcile", err)
		}
	}

	if event.RelationUpdate != nil && event.RelationUpdate.Object == state.Device.Name &&
		!applyRelation(&result.State, *event.RelationUpdate) {
		result.Disposition = ReconcileUnsupported

		return result, nil
	}

	result.State.UserId = &event.UserId
	result.State.UpdatedAt = event.Timestamp

	result.State.RecentEventIds = append(result.State.RecentEventIds, event.EventId)

	if len(result.State.RecentEventIds) > RecentEventLimit {
		result.State.RecentEventIds = result.State.RecentEventIds[len(result.State.RecentEventIds)-RecentEventLimit:]
	}

	result.Disposition = ReconcileApplied

	return result, nil
}

func cloneState(state DeviceState) (DeviceState, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return DeviceState{}, fmt.Errorf("encode device state: %w", err)
	}

	var clone DeviceState

	err = json.Unmarshal(data, &clone)
	if err != nil {
		return clone, fmt.Errorf("decode device state: %w", err)
	}

	return clone, nil
}

func targetsDevice(event EventEnvelope, name string) bool {
	return event.ResourceUpdate != nil && event.ResourceUpdate.Name == name ||
		event.RelationUpdate != nil && event.RelationUpdate.Object == name
}

func mergeTraits(device *Device, update *Traits) error {
	if update == nil {
		return nil
	}

	existing, err := traitObject(device.Traits)
	if err != nil {
		return fmt.Errorf("read prior traits: %w", err)
	}

	incoming, err := traitObject(update)
	if err != nil {
		return fmt.Errorf("read changed traits: %w", err)
	}

	var traits Traits

	merged, err := json.Marshal(mergeObjects(existing, incoming))
	if err == nil {
		err = contracts.Validate("traits.openapi.yaml", "Traits", merged)
	}

	if err == nil {
		err = json.Unmarshal(merged, &traits)
	}

	if err != nil {
		return fmt.Errorf("merge traits: %w", err)
	}

	device.Traits = &traits

	return nil
}

// traitObject retains opaque numbers exactly while exposing objects for partial merging.
func traitObject(traits *Traits) (map[string]any, error) {
	var object map[string]any

	data, err := json.Marshal(traits)
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		err = decoder.Decode(&object)
	}

	if err != nil {
		return nil, fmt.Errorf("read trait object: %w", err)
	}

	return object, nil
}

func mergeObjects(existing, incoming map[string]any) map[string]any {
	if existing == nil {
		existing = make(map[string]any)
	}

	for name, value := range incoming {
		previous, priorObject := existing[name].(map[string]any)
		update, updateObject := value.(map[string]any)

		if priorObject && updateObject && previous != nil && update != nil {
			existing[name] = mergeObjects(previous, update)
		} else {
			existing[name] = value
		}
	}

	return existing
}

func applyRelation(state *DeviceState, relation ResourceRelation) bool {
	switch relation.Type {
	case RelationTypeDELETED:
		state.Deleted = true
	case RelationTypeCREATED, RelationTypeUPDATED:
		state.Deleted = false
		parent := relation.Subject
		parents := []ParentRelation{{Parent: &parent, DisplayName: nil, AdditionalProperties: nil}}

		if state.Device.ParentRelations != nil {
			for _, existing := range *state.Device.ParentRelations {
				if existing.Parent != nil && *existing.Parent == parent {
					parents[0] = existing
				}
			}
		}

		state.Device.ParentRelations = &parents
	default:
		return false
	}

	return true
}
