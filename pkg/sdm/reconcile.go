package sdm

import (
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

	prior, err := json.Marshal(device.Traits)
	if err != nil {
		return fmt.Errorf("encode prior traits: %w", err)
	}

	changes, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("encode changed traits: %w", err)
	}

	var existing map[string]json.RawMessage
	err = json.Unmarshal(prior, &existing)
	if err != nil {
		return fmt.Errorf("decode prior traits: %w", err)
	}

	var incoming map[string]json.RawMessage
	err = json.Unmarshal(changes, &incoming)
	if err != nil {
		return fmt.Errorf("decode changed traits: %w", err)
	}

	merged, err := mergeObjects(existing, incoming)
	if err != nil {
		return err
	}

	var traits Traits

	err = contracts.Validate("traits.openapi.yaml", "Traits", merged)
	if err != nil {
		return fmt.Errorf("validate merged traits: %w", err)
	}

	err = json.Unmarshal(merged, &traits)
	if err != nil {
		return fmt.Errorf("decode merged traits: %w", err)
	}

	device.Traits = &traits

	return nil
}

func mergeObjects(existing, incoming map[string]json.RawMessage) ([]byte, error) {
	if existing == nil {
		existing = make(map[string]json.RawMessage)
	}

	for name, raw := range incoming {
		var previous, update map[string]json.RawMessage

		priorErr := json.Unmarshal(existing[name], &previous)

		updateErr := json.Unmarshal(raw, &update)

		if priorErr == nil && updateErr == nil && previous != nil && update != nil {
			merged, err := mergeObjects(previous, update)
			if err != nil {
				return nil, err
			}

			existing[name] = merged
		} else {
			existing[name] = raw
		}
	}

	merged, err := json.Marshal(existing)
	if err != nil {
		return nil, fmt.Errorf("encode merged traits: %w", err)
	}
	return merged, nil
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
