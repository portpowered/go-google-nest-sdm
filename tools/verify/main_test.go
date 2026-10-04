package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFindModulesIncludesNestedModulesAndSkipsBuildArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	directories := []string{
		".", "cmd/client", "tools/generator", "tools/bin/temporary",
		"node_modules/package", "vendor/package", ".git/cache", "site/cache",
	}

	for _, directory := range directories {
		filename := filepath.Join(root, directory, "go.mod")

		err := os.MkdirAll(filepath.Dir(filename), 0o700)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(filename, []byte("module example.com/test\n"), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	got, err := findModules(root)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{".", filepath.Join("cmd", "client"), filepath.Join("tools", "generator")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("modules = %q, want %q", got, want)
	}
}

func TestModuleArgumentsFailClosed(t *testing.T) {
	t.Parallel()

	if got := moduleArguments("unrecognized"); got != nil {
		t.Fatalf("unknown mode accepted: %q", got)
	}

	want := []string{"test", "-race", "-timeout=180s", "./..."}
	if got := moduleArguments("test"); !reflect.DeepEqual(got, want) {
		t.Fatalf("test args = %q", got)
	}
}
