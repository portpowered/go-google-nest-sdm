package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDriftGateRejectsUntrackedStagedAndModifiedOutputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	err := command(t.Context(), root, nil, "git", "init", "--quiet")
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(root, "api")

	err = os.Mkdir(directory, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(directory, "model.json")

	err = os.WriteFile(filename, []byte("{}\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	assertDrift(t, root, true)

	err = command(t.Context(), root, nil, "git", "add", "api/model.json")
	if err != nil {
		t.Fatal(err)
	}

	assertDrift(t, root, true)

	err = command(t.Context(), root, nil, "git",
		"-c", "user.name=Verification", "-c", "user.email=verification@example.invalid",
		"commit", "--quiet", "-m", "baseline")
	if err != nil {
		t.Fatal(err)
	}

	assertDrift(t, root, false)

	err = os.WriteFile(filename, []byte("{\"changed\":true}\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	assertDrift(t, root, true)

	err = os.Remove(filename)
	if err != nil {
		t.Fatal(err)
	}

	assertDrift(t, root, true)
}

func assertDrift(t *testing.T, root string, want bool) {
	t.Helper()

	err := cleanPaths(t.Context(), root, "generated output drift", "api")
	if (err != nil) != want {
		t.Fatalf("drift error = %v, want drift %v", err, want)
	}
}
