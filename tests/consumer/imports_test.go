package consumer_test

import (
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func TestPublicImportPaths(t *testing.T) {
	t.Parallel()

	client, err := httptransport.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	var publicClient sdm.Client = client
	if publicClient == nil {
		t.Fatal("constructor returned a nil public client")
	}
}
