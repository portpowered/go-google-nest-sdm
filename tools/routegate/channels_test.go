package main

import "testing"

func TestChannelSchemaProvenance(t *testing.T) {
	t.Parallel()

	const (
		channelName       = "sdmEvents"
		channelSymbol     = "ChannelSDMEvents"
		channelNameSymbol = "ChannelSDMEventsName"
		novelChannel      = "novel"
	)

	const address = "projects/{project}/topics/{topic}"

	cases := map[string]map[string]string{
		"valid":              {channelSymbol: address, channelNameSymbol: channelName},
		"drifted address":    {channelSymbol: address + "/novel", channelNameSymbol: channelName},
		"missing channel":    {channelNameSymbol: channelName},
		"wrong channel name": {channelSymbol: address, channelNameSymbol: novelChannel},
		"unknown generated channel": {channelSymbol: address, channelNameSymbol: channelName,
			"ChannelNovel": novelChannel},
	}
	channels := map[string]schemaChannel{channelName: {GoName: "SDMEvents", Address: address}}

	for name, constants := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			seen := map[string]bool{}

			err := auditSchemaChannels(channels, constants, seen)
			if err == nil {
				err = auditChannelInventory(constants, seen)
			}

			if (err == nil) != (name == "valid") {
				t.Fatalf("unexpected channel provenance: %v", err)
			}
		})
	}
}

func TestUnknownSchemaChannel(t *testing.T) {
	t.Parallel()

	channels := map[string]schemaChannel{"novel": {GoName: "Novel", Address: "projects/{project}/topics/{topic}"}}

	err := auditSchemaChannels(channels, map[string]string{}, map[string]bool{})
	if err == nil {
		t.Fatal("schema channel absent from generated inventory accepted")
	}
}
