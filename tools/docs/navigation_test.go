package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNavigationRejectsDroppedAPIGroup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	directory := filepath.Join(root, "docs")

	err := os.Mkdir(directory, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(directory, "index.html")

	var page strings.Builder

	for _, group := range navigationGroups() {
		if group != "pubsub" {
			page.WriteString("/project/docs/openapi/" + group + "/Operation/")
		}
	}

	err = os.WriteFile(filename, []byte(page.String()), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = checkNavigation(root)
	if err == nil {
		t.Fatal("dropped Pub/Sub navigation passed")
	}

	page.WriteString("/project/docs/openapi/pubsub/Pull/")

	err = os.WriteFile(filename, []byte(page.String()), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = checkNavigation(root)
	if err != nil {
		t.Fatal(err)
	}
}
