package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type asyncAPIChannel struct {
	Address    string         `yaml:"address"`
	Extensions map[string]any `yaml:",inline"`
}

type asyncAPIChannels struct {
	Channels map[string]asyncAPIChannel `yaml:"channels"`
}

// channelConstants inventories the provider's message source. The SDK receives
// events through subscriptions and HTTP push; it does not open the topic itself.
func channelConstants() error {
	data, err := os.ReadFile("api/asyncapi.yaml")
	if err != nil {
		return fmt.Errorf("read channel schema: %w", err)
	}

	var document asyncAPIChannels

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return fmt.Errorf("parse channel schema: %w", err)
	}

	values := make(map[string]string, len(document.Channels))

	for name, channel := range document.Channels {
		goName, _ := channel.Extensions["x-go-name"].(string)
		if goName == "" {
			goName = strings.ToUpper(name[:1]) + name[1:]
		}

		values["Channel"+goName] = channel.Address
		values["Channel"+goName+"Name"] = name
	}

	return writeConstants("internal/protocol/channels.gen.go", "protocol", values)
}
