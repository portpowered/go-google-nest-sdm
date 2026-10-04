package sdm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

type eventContextKey struct{}

var (
	errApplication  = errors.New("application failed")
	errPrivateToken = errors.New("private token")
)

const outerEventID = "outer"

const eventJSON = `{
  "eventId": "outer",
  "timestamp": "2026-10-04T00:00:00Z",
  "userId": "user",
  "eventThreadId": "thread",
  "eventThreadState": "STARTED",
  "resourceGroup": [
    "enterprises/example/devices/device"
  ],
  "resourceUpdate": {
    "name": "enterprises/example/devices/device",
    "events": {
      "sdm.devices.events.CameraMotion.Motion": {
        "eventId": "image",
        "eventSessionId": "session"
      },
      "sdm.devices.events.CameraPerson.Person": {
        "eventSessionId": "session"
      },
      "sdm.devices.events.CameraSound.Sound": {
        "eventId": "sound"
      },
      "sdm.devices.events.DoorbellChime.Chime": {
        "eventSessionId": "session"
      },
      "sdm.devices.events.CameraClipPreview.ClipPreview": {
        "eventSessionId": "session",
        "previewUrl": "https://example.com/clip"
      },
      "future.event": {
        "value": null
      }
    }
  },
  "future": 9007199254740993
}`

func TestEventIdentifiersAndFamilies(t *testing.T) {
	t.Parallel()

	event, err := sdm.DecodeEvent([]byte(eventJSON))
	if err != nil {
		t.Fatal(err)
	}

	if event.EventId != outerEventID ||
		*event.ResourceUpdate.Events.CameraMotion.EventId != "image" ||
		*event.ResourceUpdate.Events.CameraMotion.EventSessionId != "session" ||
		*event.EventThreadId != "thread" {
		t.Fatal("different identifiers collapsed")
	}

	if event.ResourceUpdate.Events.CameraClipPreview.EventSessionId != "session" ||
		event.ResourceUpdate.Events.CameraClipPreview.PreviewUrl != "https://example.com/clip" {
		t.Fatal("clip preview lost")
	}

	if string(event.AdditionalProperties["future"]) != "9007199254740993" ||
		len(event.ResourceUpdate.Events.AdditionalProperties) != 1 {
		t.Fatal("unknown envelope/event values lost")
	}

	_, err = sdm.DecodeEvent([]byte(`{
  "eventId": "relation",
  "timestamp": "2026-10-04T00:00:00Z",
  "userId": "user",
  "relationUpdate": {
    "type": "CREATED",
    "subject": "",
    "object": "enterprises/example/devices/device"
  }
}`))
	if err != nil {
		t.Fatal("documented empty subject rejected", err)
	}
}

func TestEventRejectsKnownContractViolations(t *testing.T) {
	t.Parallel()

	cases := []string{
		`{}`,
		`{"eventId":"","timestamp":"2026-10-04T00:00:00Z","userId":"user"}`,
		`{"eventId":"event","timestamp":"invalid","userId":"user"}`,
		`{
  "eventId": "event",
  "timestamp": "2026-10-04T00:00:00Z",
  "userId": "",
  "relationUpdate": {
    "type": "CREATED",
    "subject": "",
    "object": "enterprises/example/devices/device"
  }
}`,
		`{
  "eventId": "event",
  "timestamp": "2026-10-04T00:00:00Z",
  "userId": "user",
  "resourceUpdate": {
    "name": "enterprises/example/devices/device",
    "events": {
      "sdm.devices.events.CameraClipPreview.ClipPreview": {
        "eventSessionId": "session"
      }
    }
  }
}`,
		`{
  "eventId": "event",
  "timestamp": "2026-10-04T00:00:00Z",
  "userId": "user",
  "resourceUpdate": {
    "name": "enterprises/example/devices/device",
    "events": {
      "sdm.devices.events.CameraMotion.Motion": null
    }
  }
}`,
		`{
  "eventId": "event",
  "timestamp": "2026-10-04T00:00:00Z",
  "userId": "user",
  "resourceUpdate": {
    "name": "enterprises/example/devices/device",
    "traits": {
      "sdm.devices.traits.Humidity": {
        "ambientHumidityPercent": "wrong"
      }
    }
  }
}`,
	}
	for _, data := range cases {
		_, err := sdm.DecodeEvent([]byte(data))
		if err == nil {
			t.Errorf("accepted %s", data)
		}
	}
}

func TestStatelessEventHandler(t *testing.T) {
	t.Parallel()

	key := eventContextKey{}
	ctx := context.WithValue(context.Background(), key, "value")
	called := false

	handler := func(got context.Context, event sdm.EventEnvelope) error {
		called = true

		if got.Value(key) != "value" || event.EventId != outerEventID {
			t.Fatal("context or decoded event lost")
		}

		return nil
	}

	result, err := sdm.HandleEvent(ctx, sdm.HandleEventRequest{Data: []byte(eventJSON)}, handler)
	if err != nil || !called || result.Event.EventId != outerEventID {
		t.Fatalf("handler: %v", err)
	}

	wanted := errApplication

	failingHandler := func(context.Context, sdm.EventEnvelope) error { return wanted }
	_, err = sdm.HandleEvent(ctx, sdm.HandleEventRequest{Data: []byte(eventJSON)}, failingHandler)

	if !errors.Is(err, wanted) {
		t.Fatal("application error lost")
	}

	_, err = sdm.HandleEvent(ctx, sdm.HandleEventRequest{Data: []byte(eventJSON)}, nil)
	if err == nil {
		t.Fatal("nil handler accepted")
	}

	_, err = sdm.HandleEvent(ctx, sdm.HandleEventRequest{Data: []byte(`{}`)}, func(
		context.Context, sdm.EventEnvelope,
	) error {
		t.Fatal("malformed event dispatched")

		return nil
	})
	if err == nil {
		t.Fatal("malformed input accepted")
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()

	_, err = sdm.HandleEvent(canceled, sdm.HandleEventRequest{Data: []byte(eventJSON)}, func(
		context.Context, sdm.EventEnvelope,
	) error {
		t.Fatal("canceled event dispatched")

		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost")
	}
}

func TestReconcilePartialUpdatesAndOrdering(t *testing.T) {
	t.Parallel()

	device, err := sdm.DecodeDevice([]byte(`{
  "name": "enterprises/example/devices/device",
  "traits": {
    "sdm.devices.traits.ThermostatMode": {
      "mode": "HEAT",
      "availableModes": [
        "HEAT",
        "COOL"
      ]
    },
    "sdm.devices.traits.ThermostatTemperatureSetpoint": {
      "heatCelsius": 20,
      "coolCelsius": 22
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	event, err := sdm.DecodeEvent([]byte(`{
  "eventId": "first",
  "timestamp": "2026-10-04T00:00:00Z",
  "userId": "user",
  "resourceUpdate": {
    "name": "enterprises/example/devices/device",
    "traits": {
      "sdm.devices.traits.ThermostatMode": {
        "mode": "COOL"
      },
      "sdm.devices.traits.ThermostatTemperatureSetpoint": {
        "heatCelsius": 0
      }
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	var initial sdm.DeviceState

	initial.Device = device

	result, err := sdm.Reconcile(sdm.ReconcileRequest{State: initial, Event: event})
	if err != nil {
		t.Fatal(err)
	}

	if result.Disposition != sdm.ReconcileApplied {
		t.Fatal(result.Disposition)
	}

	mode := result.State.Device.Traits.SdmDevicesTraitsThermostatMode
	if *mode.Mode != sdm.ThermostatModeValueCOOL || len(*mode.AvailableModes) != 2 {
		t.Fatal("partial update discarded unchanged fields")
	}

	setpoint := result.State.Device.Traits.SdmDevicesTraitsThermostatTemperatureSetpoint
	if *setpoint.HeatCelsius != 0 || *setpoint.CoolCelsius != 22 {
		t.Fatal("partial update conflated absent and zero")
	}

	if *initial.Device.Traits.SdmDevicesTraitsThermostatMode.Mode != sdm.ThermostatModeValueHEAT {
		t.Fatal("input state mutated")
	}

	duplicate, err := sdm.Reconcile(sdm.ReconcileRequest{State: result.State, Event: event})
	if err != nil || duplicate.Disposition != sdm.ReconcileDuplicate {
		t.Fatal("duplicate applied", err)
	}

	event.EventId = "older"
	event.Timestamp = event.Timestamp.Add(-time.Second)

	stale, err := sdm.Reconcile(sdm.ReconcileRequest{State: result.State, Event: event})
	if err != nil || stale.Disposition != sdm.ReconcileStale {
		t.Fatal("stale applied", err)
	}

	event.EventId = "other-user"
	event.UserId = "other"

	unrelated, err := sdm.Reconcile(sdm.ReconcileRequest{State: result.State, Event: event})
	if err != nil || unrelated.Disposition != sdm.ReconcileUnrelated {
		t.Fatal("account crossed", err)
	}
}

func TestReconcileRelationsAndBoundedHistory(t *testing.T) {
	t.Parallel()

	var state sdm.DeviceState

	state.Device.Name = "enterprises/example/devices/device"
	state.RecentEventIds = make([]sdm.EnvelopeEventID, sdm.RecentEventLimit)

	var event sdm.EventEnvelope

	event.EventId = "relation"
	event.Timestamp = time.Date(2026, time.October, 4, 0, 0, 0, 0, time.UTC)
	event.UserId = "user"
	event.RelationUpdate = &sdm.ResourceRelation{
		Type:                 sdm.RelationTypeCREATED,
		Subject:              "enterprises/example/structures/home",
		Object:               state.Device.Name,
		AdditionalProperties: nil,
	}

	result, err := sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.State.RecentEventIds) != sdm.RecentEventLimit ||
		*(*result.State.Device.ParentRelations)[0].Parent != event.RelationUpdate.Subject {
		t.Fatal("history or relation wrong")
	}

	event.EventId = "delete"
	event.RelationUpdate.Type = sdm.RelationTypeDELETED

	deleted, err := sdm.Reconcile(sdm.ReconcileRequest{State: result.State, Event: event})
	if err != nil || !deleted.State.Deleted {
		t.Fatal("deletion lost", err)
	}

	event.EventId = "future"
	event.RelationUpdate.Type = "FUTURE"

	unsupported, err := sdm.Reconcile(sdm.ReconcileRequest{State: result.State, Event: event})
	if err != nil || unsupported.Disposition != sdm.ReconcileUnsupported {
		t.Fatal("unknown relation applied", err)
	}

	event.EventId = "unrelated"
	event.RelationUpdate.Object = "enterprises/example/devices/other"

	unrelated, err := sdm.Reconcile(sdm.ReconcileRequest{State: result.State, Event: event})
	if err != nil || unrelated.Disposition != sdm.ReconcileUnrelated {
		t.Fatal("other resource applied", err)
	}

	var emptyEvent sdm.EventEnvelope

	_, err = sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: emptyEvent})
	if err == nil {
		t.Fatal("missing event identity accepted")
	}
}

func TestErrorsPreserveCausesWithoutPrintingSecrets(t *testing.T) {
	t.Parallel()

	wanted := errPrivateToken

	err := &sdm.Error{Kind: sdm.ErrorTransport, Operation: "test", Cause: wanted, StatusCode: 503}

	if !errors.Is(err, wanted) || err.Error() != "sdm test: transport" {
		t.Fatal("error cause or safe diagnostic wrong")
	}
}

func TestHandlerClassifiesContextFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sdm.HandleEvent(ctx, sdm.HandleEventRequest{Data: []byte(eventJSON)},
		func(context.Context, sdm.EventEnvelope) error {
			t.Fatal("canceled event dispatched")

			return nil
		})

	var typed *sdm.Error
	if !errors.As(err, &typed) || typed.Kind != sdm.ErrorCanceled {
		t.Fatal("cancellation did not return a typed client error")
	}

	deadline, release := context.WithDeadline(context.Background(), time.Time{})
	defer release()

	_, err = sdm.HandleEvent(deadline, sdm.HandleEventRequest{Data: []byte(eventJSON)},
		func(context.Context, sdm.EventEnvelope) error {
			t.Fatal("expired event dispatched")

			return nil
		})
	if !errors.As(err, &typed) || typed.Kind != sdm.ErrorTimeout || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("deadline did not return a typed client error preserving the cause")
	}
}
