package main

import (
	"context"
	"strings"
	"testing"
)

func TestDiscoveryCommandsUsePairedRequests(t *testing.T) {
	t.Parallel()

	groups := []string{devicesGroup, devicesGroup, "structures", "structures", "rooms", "rooms"}
	operations := []string{listOperation, getOperation, listOperation, getOperation, listOperation, getOperation}
	resources := []string{
		syntheticEnterprise, syntheticDevice,
		syntheticEnterprise, "enterprises/synthetic-enterprise/structures/synthetic-structure",
		"enterprises/synthetic-enterprise/structures/synthetic-structure",
		"enterprises/synthetic-enterprise/structures/synthetic-structure/rooms/synthetic-room",
	}

	for index, group := range groups {
		app, _ := newTestApplication(t, loadPairs(t, "rest-resources", index), "")

		err := app.run(context.Background(), []string{group, operations[index], resourceFlag, resources[index]})
		if err != nil {
			t.Fatalf("%s %s: %v", group, operations[index], err)
		}
	}
}

func TestInvocationHelpAndErrors(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{}, {"help"}, {"--help"}, {devicesGroup, listOperation, "--help"}} {
		app, output := newTestApplication(t, nil, "")

		err := app.run(context.Background(), args)
		if err != nil {
			t.Fatal(err)
		}

		if output.Len() == 0 {
			t.Fatal("help was empty")
		}
	}

	for _, args := range [][]string{
		{devicesGroup}, {devicesGroup, listOperation, "--timeout", "0s"}, {devicesGroup, listOperation, "--count", "0"},
		{devicesGroup, listOperation, "--credentials", "-", paramsFlag, "-"}, {devicesGroup, listOperation, "unexpected"},
		{devicesGroup, listOperation, "--unknown"}, {"unknown", "command", resourceFlag, "enterprises/example"},
	} {
		app, output := newTestApplication(t, nil, "")

		err := app.run(context.Background(), args)
		if err == nil {
			t.Fatalf("accepted invalid invocation: %v", args)
		}

		if output.Len() != 0 {
			t.Fatalf("error printed unstructured output: %s", output)
		}
	}
}

func TestAuthExportIsExplicitAndReusable(t *testing.T) {
	t.Parallel()

	app, output := newTestApplication(t, nil, "")

	runErr := app.run(context.Background(), []string{authGroup, "export"})
	if runErr != nil {
		t.Fatal(runErr)
	}

	account, err := readCredentials("-", strings.NewReader(output.String()), func(string) string { return "" })
	if err != nil || account.AccessToken != syntheticAccessToken || account.ClientSecret != syntheticClientSecret {
		t.Fatalf("exported credentials: %+v, %v", account, err)
	}
}

func TestDecodeEventsValidatesKnownPayloads(t *testing.T) {
	t.Parallel()

	event := `{"eventId":"synthetic-event","timestamp":"2026-01-01T00:00:00Z","userId":"synthetic-user",` +
		`"resourceUpdate":{"name":"enterprises/example/devices/one",` +
		`"traits":{"sdm.devices.traits.ThermostatMode":{"mode":2}}}}`
	app, _ := newTestApplication(t, nil, event)

	err := app.run(context.Background(), []string{eventsGroup, "decode", paramsFlag, "-"})
	if err == nil {
		t.Fatal("malformed known trait decoded")
	}
}
