package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCLIUsesCheckoutWithoutChangingPublishedMetadata(t *testing.T) {
	t.Parallel()

	root, original := setupLocalCLI(t)

	environment := append(os.Environ(), "GOFLAGS=-trimpath", "GOWORK=off", "GOTOOLCHAIN=local")

	check, err := prepareModule(t.Context(), root, cliModule, environment)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := check.close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	err = command(t.Context(), check.directory, check.environment, "go", "test", "./...")
	if err != nil {
		t.Fatal(err)
	}

	err = command(t.Context(), check.directory, check.environment, "go", "mod", "tidy")
	if err != nil {
		t.Fatal(err)
	}

	err = check.tidyMatches(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	unchanged, err := os.ReadFile(filepath.Join(check.directory, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(unchanged, original) {
		t.Fatal("published CLI metadata was modified")
	}

	if strings.Contains(string(unchanged), "replace") {
		t.Fatal("published CLI contains local replacement")
	}
}

func TestCLITidyGateDetectsDependencyDrift(t *testing.T) {
	t.Parallel()

	root, _ := setupLocalCLI(t)

	check, err := prepareModule(t.Context(), root, cliModule, append(os.Environ(), "GOWORK=off"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := check.close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	err = command(t.Context(), check.directory, check.environment,
		"go", "mod", "edit", "-require=example.invalid/drift@v1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	err = check.tidyMatches(t.Context())
	if err == nil {
		t.Fatal("dependency drift passed CLI metadata verification")
	}
}

func TestCLIWorkspaceSupportsModuleDisabledLinterProbe(t *testing.T) {
	t.Parallel()

	root, _ := setupLocalCLI(t)

	environment := append(os.Environ(), "GOFLAGS=-trimpath", "GOWORK=off")

	check, err := prepareModule(t.Context(), root, cliModule, environment)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := check.close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	workspace, err := check.workspaceEnvironment(t.Context(), root, environment)
	if err != nil {
		t.Fatal(err)
	}

	probe := slices.Clone(workspace)
	probe = append(probe, "GO111MODULE=off")

	err = command(t.Context(), check.directory, probe, "go", "list", "-f={{context.ReleaseTags}}", "unsafe")
	if err != nil {
		t.Fatal(err)
	}

	err = command(t.Context(), check.directory, workspace, "go", "test", "./...")
	if err != nil {
		t.Fatal(err)
	}
}

func TestSelectedModuleMustBelongToRepository(t *testing.T) {
	t.Parallel()

	modules := []string{".", filepath.FromSlash(cliModule)}
	for _, selected := range []string{"../outside", "cmd/missing", "/outside"} {
		_, err := selectModules(modules, selected)
		if err == nil {
			t.Fatalf("accepted module outside discovered set: %q", selected)
		}
	}

	got, err := selectModules(modules, cliModule)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0] != filepath.FromSlash(cliModule) {
		t.Fatalf("selected modules = %q", got)
	}
}

func setupLocalCLI(t *testing.T) (string, []byte) {
	t.Helper()

	root := filepath.Join(t.TempDir(), "checkout with spaces")
	directory := filepath.Join(root, filepath.FromSlash(cliModule))

	err := os.MkdirAll(directory, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	original := []byte("module example.com/cli\n\ngo 1.24.0\n\nrequire " + publicModule + " v0.1.0\n")
	testSource := "package cli\nimport (\"testing\"; sdm \"" + publicModule + "\")\n" +
		"func TestCheckout(t *testing.T) { if sdm.Version != \"checkout\" { t.Fatal(sdm.Version) } }\n"

	files := map[string][]byte{
		filepath.Join(root, "go.mod"):              []byte("module " + publicModule + "\n\ngo 1.24.0\n"),
		filepath.Join(root, "library.go"):          []byte("package sdm\n\nconst Version = \"checkout\"\n"),
		filepath.Join(directory, "go.mod"):         original,
		filepath.Join(directory, "client_test.go"): []byte(testSource),
	}
	for filename, data := range files {
		err = os.WriteFile(filename, data, 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	return root, original
}
