// Command basic lists devices shared with a Device Access enterprise.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const requestTimeout = 30 * time.Second

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	client, err := httptransport.NewClient()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	result, err := client.ListDevices(ctx, sdm.ListDevicesRequest{
		Auth:   sdm.AuthContext{AccessToken: os.Getenv("NEST_ACCESS_TOKEN")},
		Parent: "enterprises/" + os.Getenv("NEST_ENTERPRISE_ID"),
		Filter: nil,
	})
	if err != nil {
		return fmt.Errorf("list devices: %w", err)
	}

	for _, device := range result.Devices {
		_, err = fmt.Fprintln(os.Stdout, device.Name)
		if err != nil {
			return fmt.Errorf("write device name: %w", err)
		}
	}

	return nil
}
