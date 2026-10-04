package sdm

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
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
	if event.EventId == "" || event.Timestamp.IsZero() || event.UserId == "" {
		return result, invalidResponse("reconcile", errors.New("event identity, user, and timestamp are required"))
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
	if event.ResourceUpdate != nil {
		if err := mergeTraits(&result.State.Device, event.ResourceUpdate.Traits); err != nil {
			return result, invalidResponse("reconcile", err)
		}
	}
	if event.RelationUpdate != nil && !applyRelation(&result.State, *event.RelationUpdate) {
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
		return DeviceState{}, err
	}
	var clone DeviceState
	err = json.Unmarshal(data, &clone)
	return clone, err
}

func targetsDevice(event EventEnvelope, name string) bool {
	return event.ResourceUpdate != nil && event.ResourceUpdate.Name == name || event.RelationUpdate != nil && event.RelationUpdate.Object == name
}

func mergeTraits(device *Device, update *Traits) error {
	if update == nil {
		return nil
	}
	prior, err := json.Marshal(device.Traits)
	if err != nil {
		return err
	}
	changes, err := json.Marshal(update)
	if err != nil {
		return err
	}
	var existing map[string]json.RawMessage
	if err := json.Unmarshal(prior, &existing); err != nil {
		return err
	}
	var incoming map[string]json.RawMessage
	if err := json.Unmarshal(changes, &incoming); err != nil {
		return err
	}
	merged, err := mergeObjects(existing, incoming)
	if err != nil {
		return err
	}
	var traits Traits
	if err := contracts.Validate("traits.openapi.yaml", "Traits", merged); err != nil {
		return err
	}
	if err := json.Unmarshal(merged, &traits); err != nil {
		return err
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
	return json.Marshal(existing)
}

func applyRelation(state *DeviceState, relation ResourceRelation) bool {
	switch relation.Type {
	case RelationTypeDELETED:
		state.Deleted = true
	case RelationTypeCREATED, RelationTypeUPDATED:
		state.Deleted = false
		parent := relation.Subject
		parents := []ParentRelation{{Parent: &parent}}
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
