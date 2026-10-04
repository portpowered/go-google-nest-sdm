package replay_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
)

const (
	accessToken   = "synthetic-access-token"
	enterprise    = "enterprises/synthetic-enterprise"
	deviceName    = enterprise + "/devices/synthetic-device"
	structureName = enterprise + "/structures/synthetic-structure"
	roomName      = structureName + "/rooms/synthetic-room"
)

func replayClient(t *testing.T, path string) *transport.Client {
	t.Helper()

	httpClient := replayHTTPClient(loadTransport(t, "fixtures/synthetic/"+path+".json"))

	client, err := transport.New(transport.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func TestResourceReplay(t *testing.T) {
	t.Parallel()
	client := replayClient(t, "rest-resources")
	ctx := t.Context()

	devices, err := client.ListDevices(ctx, accessToken, enterprise)
	if err != nil || len(*devices.Devices) != 1 {
		t.Fatalf("list devices: %#v %v", devices, err)
	}

	device, err := client.GetDevice(ctx, accessToken, deviceName)
	if err != nil || device.Name != deviceName {
		t.Fatalf("get device: %#v %v", device, err)
	}

	structures, err := client.ListStructures(ctx, accessToken, enterprise)
	if err != nil || len(*structures.Structures) != 1 {
		t.Fatalf("list structures: %#v %v", structures, err)
	}

	structure, err := client.GetStructure(ctx, accessToken, structureName)
	if err != nil || structure.Name != structureName {
		t.Fatalf("get structure: %#v %v", structure, err)
	}

	rooms, err := client.ListRooms(ctx, accessToken, structureName)
	if err != nil || len(*rooms.Rooms) != 1 {
		t.Fatalf("list rooms: %#v %v", rooms, err)
	}

	room, err := client.GetRoom(ctx, accessToken, roomName)
	if err != nil || room.Name != roomName {
		t.Fatalf("get room: %#v %v", room, err)
	}
}

func TestEveryCommandReplay(t *testing.T) {
	t.Parallel()
	client := replayClient(t, "commands")

	data, err := os.ReadFile("fixtures/synthetic/commands.json")
	if err != nil {
		t.Fatal(err)
	}

	var transcript fixture

	err = json.Unmarshal(data, &transcript)
	if err != nil {
		t.Fatal(err)
	}

	if len(transcript.Exchanges) != 13 {
		t.Fatal("all thirteen commands require paired replay")
	}

	for _, exchange := range transcript.Exchanges {
		var request wire.ExecuteCommandRequest

		err = json.Unmarshal([]byte(exchange.Request.Body), &request)
		if err != nil {
			t.Fatal(err)
		}

		result, commandErr := client.ExecuteCommand(t.Context(), accessToken, deviceName, request)
		if commandErr != nil {
			t.Fatalf("%s: %v", request.Command, commandErr)
		}

		actual, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}

		var actualValue, expectedValue any

		err = json.Unmarshal(actual, &actualValue)
		if err != nil {
			t.Fatal(err)
		}

		err = json.Unmarshal([]byte(exchange.Response.Body), &expectedValue)
		if err != nil {
			t.Fatal(err)
		}
		// Empty success may be represented by an omitted or empty results object.
		if !reflect.DeepEqual(actualValue, expectedValue) {
			t.Fatalf("command %s result differs: got %s, want %s", request.Command, actual, exchange.Response.Body)
		}
	}
}

func TestProviderErrorReplay(t *testing.T) {
	t.Parallel()
	client := replayClient(t, "provider-error")

	_, err := client.GetDevice(t.Context(), accessToken, deviceName)
	if err == nil {
		t.Fatal("provider denial must be returned")
	}
}
