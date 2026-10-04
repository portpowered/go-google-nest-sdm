package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBaselinePackageDistinguishesNewAPIFromExistingAPI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	exists, err := baselineHasPackage(root, "pkg/sdm")
	if err != nil || exists {
		t.Fatalf("new package baseline = %v, %v", exists, err)
	}

	directory := filepath.Join(root, "pkg", "sdm")

	err = os.MkdirAll(directory, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(directory, "example_test.go"), []byte("package sdm\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	exists, err = baselineHasPackage(root, "pkg/sdm")
	if err != nil || exists {
		t.Fatalf("test-only package baseline = %v, %v", exists, err)
	}

	err = os.WriteFile(filepath.Join(directory, "client.go"), []byte("package sdm\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	exists, err = baselineHasPackage(root, "pkg/sdm")
	if err != nil || !exists {
		t.Fatalf("existing package baseline = %v, %v", exists, err)
	}
}
