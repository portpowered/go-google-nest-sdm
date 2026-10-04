// Command verify runs repository checks consistently on supported operating systems.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
)

const (
	lintVersion = "v2.14.0"
	allPackages = "./..."
)

type verificationError struct {
	operation string
	cause     error
}

func (err verificationError) Error() string {
	if err.cause == nil {
		return err.operation
	}

	return err.operation + ": " + err.cause.Error()
}
func (err verificationError) Unwrap() error { return err.cause }

func main() {
	mode := flag.String("mode", "build", "build, test, vet, lint, fmt, or modules")

	flag.Parse()

	err := run(context.Background(), *mode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, mode string) error {
	root, err := os.Getwd()
	if err != nil {
		return verificationError{operation: "find repository", cause: err}
	}

	modules, err := findModules(root)
	if err != nil {
		return err
	}

	if mode == "lint" {
		return lintModules(ctx, root, modules)
	}

	if mode == "generated" {
		return cleanPaths(ctx, root, "generated output drift", "api", "internal", "pkg")
	}

	if mode == "format-check" {
		return checkFormatting(ctx, root)
	}

	environment, err := repositoryEnvironment(os.Environ(), root)
	if err != nil {
		return err
	}

	for _, module := range modules {
		args := moduleArguments(mode)
		if len(args) == 0 {
			return verificationError{operation: "unknown verification mode: " + mode, cause: nil}
		}

		err = runModule(ctx, root, module, environment, "go", args, mode == "modules")
		if err != nil {
			return err
		}

		if mode == "modules" {
			err = cleanMetadata(ctx, root, module)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func findModules(root string) ([]string, error) {
	modules := make([]string, 0)

	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "vendor", "site", "bin":
				return filepath.SkipDir
			}

			return nil
		}

		if entry.Name() != "go.mod" {
			return nil
		}

		relative, err := filepath.Rel(root, filepath.Dir(filename))
		if err != nil {
			return verificationError{operation: "resolve module path", cause: err}
		}

		modules = append(modules, relative)

		return nil
	})
	if err != nil {
		return nil, verificationError{operation: "discover Go modules", cause: err}
	}

	sort.Strings(modules)

	return modules, nil
}

func moduleArguments(mode string) []string {
	switch mode {
	case "build":
		return []string{"build", allPackages}
	case "test":
		return []string{"test", "-race", "-timeout=180s", allPackages}
	case "vet":
		return []string{"vet", allPackages}
	case "fmt":
		return []string{"fmt", allPackages}
	case "modules":
		return []string{"mod", "tidy"}
	default:
		return nil
	}
}

func lintModules(ctx context.Context, root string, modules []string) error {
	environment, err := repositoryEnvironment(os.Environ(), root)
	if err != nil {
		return err
	}

	bin := filepath.Join(root, "tools", "bin", lintVersion)

	linter := filepath.Join(bin, "golangci-lint")
	if runtime.GOOS == "windows" {
		linter += ".exe"
	}

	_, err = os.Stat(linter)
	if err != nil {
		if !os.IsNotExist(err) {
			return verificationError{operation: "inspect pinned linter", cause: err}
		}

		env := slices.Clone(environment)
		env = append(env, "GOBIN="+bin, "GOTOOLCHAIN=auto", "GOWORK=off")
		tool := "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + lintVersion

		err = command(ctx, root, env, "go", "install", tool)
		if err != nil {
			return err
		}
	}

	for _, module := range modules {
		configuration := filepath.Join(root, ".golangci.yml")

		args := []string{"run", "--config", configuration, allPackages}
		err := runModule(ctx, root, module, environment, linter, args, false)
		if err != nil {
			return err
		}
	}

	return nil
}

func cleanMetadata(ctx context.Context, root, module string) error {
	moduleFile := filepath.ToSlash(filepath.Join(module, "go.mod"))
	checksumFile := filepath.ToSlash(filepath.Join(module, "go.sum"))

	return cleanPaths(ctx, root, "module metadata drift", moduleFile, checksumFile)
}

func cleanPaths(ctx context.Context, root, operation string, paths ...string) error {
	gitArguments := []string{
		"-c", "safe.directory=" + filepath.ToSlash(root), "status", "--porcelain", "--untracked-files=all", "--",
	}
	args := make([]string, 0, len(gitArguments)+len(paths))
	args = append(args, gitArguments...)
	args = append(args, paths...)
	// #nosec G204 -- fixed git command, local repository paths, no shell interpolation.
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root

	output, err := cmd.CombinedOutput()
	if err != nil {
		return verificationError{operation: "inspect module metadata", cause: err}
	}

	if strings.TrimSpace(string(output)) != "" {
		return verificationError{operation: operation + ": " + string(output), cause: nil}
	}

	return nil
}

func checkFormatting(ctx context.Context, root string) error {
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "vendor", "site", "bin":
				return filepath.SkipDir
			}

			return nil
		}

		if filepath.Ext(filename) != ".go" {
			return nil
		}

		// #nosec G204 -- fixed formatter with local source filename as an argument.
		cmd := exec.CommandContext(ctx, "gofmt", "-l", filename)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return verificationError{operation: "check Go formatting", cause: err}
		}

		if strings.TrimSpace(string(output)) != "" {
			return verificationError{operation: "unformatted Go file: " + filename, cause: nil}
		}

		return nil
	})
	if err != nil {
		return verificationError{operation: "check repository formatting", cause: err}
	}

	return nil
}

func command(ctx context.Context, directory string, environment []string, program string, args ...string) error {
	// #nosec G204 -- fixed repository check executables and argument lists; no shell is invoked.
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = directory
	cmd.Env = environment

	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	err := cmd.Run()
	if err != nil {
		return verificationError{operation: program + " " + strings.Join(args, " "), cause: err}
	}

	return nil
}
