package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryEnvironmentPreservesInheritedGitConfiguration(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	original := []string{
		"PATH=original", "GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=core.autocrlf", "GIT_CONFIG_VALUE_0=false",
	}

	got, err := repositoryEnvironment(original, root)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"PATH=original", "GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=core.autocrlf", "GIT_CONFIG_VALUE_0=false",
		"GIT_CONFIG_KEY_1=safe.directory", "GIT_CONFIG_VALUE_1=" + filepath.ToSlash(root),
	}
	for _, entry := range want {
		found := false

		for _, candidate := range got {
			if candidate == entry {
				found = true
			}
		}

		if !found {
			t.Errorf("missing environment entry %q", entry)
		}
	}

	if original[1] != "GIT_CONFIG_COUNT=1" {
		t.Fatal("inherited environment was mutated")
	}

	counts := 0

	for _, entry := range got {
		if strings.HasPrefix(entry, "GIT_CONFIG_COUNT=") {
			counts++
		}
	}

	if counts != 1 {
		t.Fatalf("Git configuration counts = %d", counts)
	}
}

func TestRepositoryEnvironmentRejectsInvalidInheritedCount(t *testing.T) {
	t.Parallel()

	for _, count := range []string{"invalid", "-1"} {
		_, err := repositoryEnvironment([]string{"GIT_CONFIG_COUNT=" + count}, t.TempDir())
		if err == nil {
			t.Fatalf("accepted Git configuration count %q", count)
		}
	}
}
