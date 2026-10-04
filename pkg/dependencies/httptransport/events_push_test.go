package httptransport_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

//nolint:ireturn // Consumers access push decoding through the public event interface.
func newPushClient(t *testing.T) sdm.EventClient {
	t.Helper()

	client, err := httptransport.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	events, ok := client.(sdm.EventClient)
	if !ok {
		t.Fatal("client lacks events interface")
	}

	return events
}

func TestPushDecodingWrappedAndUnwrapped(t *testing.T) {
	t.Parallel()
	client := newPushClient(t)

	data := base64.StdEncoding.EncodeToString([]byte(testEvent))
	wrapped := `{"subscription":"projects/project/subscriptions/events","message":{"data":"` + data + `"}}`
	cases := []sdm.DecodePushEventRequest{
		{Body: []byte(testEvent), Unwrapped: true},
		{Body: []byte(wrapped), Unwrapped: false},
	}

	for _, request := range cases {
		result, err := client.DecodePushEvent(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}

		if result.Event.EventId != "event" {
			t.Fatal("wrong event")
		}

		if result.Event.ResourceUpdate == nil || result.Event.ResourceUpdate.Name != "enterprises/project/devices/device" {
			t.Fatal("resource update was lost")
		}
	}
}

func TestPushRejectsMalformedWrappedData(t *testing.T) {
	t.Parallel()

	client := newPushClient(t)
	for _, body := range []string{
		`null`, `[]`, `{}`, `{"subscription":"projects/project/subscriptions/events","message":null}`,
		`{"subscription":"projects/project/subscriptions/events","message":{"data":"!"}}`,
		`{"subscription":"projects/project/subscriptions/events","message":{"data":"e30="}}`,
	} {
		_, err := client.DecodePushEvent(context.Background(),
			sdm.DecodePushEventRequest{Body: []byte(body), Unwrapped: false})

		var failure *sdm.Error

		if !errors.As(err, &failure) || failure.Kind != sdm.ErrorInvalidResponse {
			t.Errorf("body %s: %v", body, err)
		}
	}
}
