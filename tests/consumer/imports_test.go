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

	requireClient(t, client)
}

func requireClient(t *testing.T, client sdm.Client) {
	t.Helper()

	if client == nil {
		t.Fatal("constructor returned a nil public client")
	}
}
