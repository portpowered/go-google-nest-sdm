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
	client, err := httptransport.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	result, err := client.ListDevices(ctx, sdm.ListDevicesRequest{
		Auth:   sdm.AuthContext{AccessToken: os.Getenv("NEST_ACCESS_TOKEN")},
		Parent: "enterprises/" + os.Getenv("NEST_ENTERPRISE_ID"),
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, device := range result.Devices {
		fmt.Println(device.Name)
	}
}
