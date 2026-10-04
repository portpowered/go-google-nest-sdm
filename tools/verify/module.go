package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const (
	cliModule        = "cmd/go-google-nest-sdm"
	publicModule     = "github.com/portpowered/go-google-nest-sdm"
	moduleFilename   = "go.mod"
	checksumFilename = "go.sum"
	moduleFileMode   = 0o600
)

type moduleCheck struct {
	directory          string
	environment        []string
	temporaryDirectory string
	temporaryModfile   string
}

func selectModules(modules []string, selected string) ([]string, error) {
	if selected == "" {
		return modules, nil
	}

	for _, module := range modules {
		if filepath.ToSlash(module) == selected {
			return []string{module}, nil
		}
	}

	return nil, verificationError{operation: "unknown repository Go module: " + selected, cause: nil}
}

// prepareModule leaves the distributable CLI metadata untouched. Repository
// checks resolve the matching checkout exclusively through a temporary modfile.
func prepareModule(ctx context.Context, root, module string, environment []string) (moduleCheck, error) {
	check := moduleCheck{
		directory: filepath.Join(root, module), environment: environment,
		temporaryDirectory: "", temporaryModfile: "",
	}
	if filepath.ToSlash(module) != cliModule {
		return check, nil
	}

	// Workspace membership compares paths lexically. On macOS the system temp
	// directory may use /var while Getwd returns /private/var. Resolve aliases
	// before writing the workspace and its SDK replacement.
	canonicalRoot, err := canonicalDirectory(root)
	if err != nil {
		return check, err
	}

	check.directory, err = canonicalDirectory(check.directory)
	if err != nil {
		return check, err
	}

	directory, err := os.MkdirTemp("", "sdm-cli-verify-")
	if err != nil {
		return check, verificationError{operation: "create CLI verification modfile directory", cause: err}
	}

	check.temporaryDirectory = directory

	check.temporaryDirectory, err = canonicalDirectory(directory)
	if err != nil {
		check.temporaryDirectory = directory

		return check, errors.Join(err, check.close())
	}

	directory = check.temporaryDirectory
	check.temporaryModfile = filepath.Join(directory, moduleFilename)

	err = copyMetadata(check.directory, directory)
	if err != nil {
		return check, errors.Join(err, check.close())
	}

	flags, err := modfileFlags(environment, check.temporaryModfile)
	if err != nil {
		return check, errors.Join(err, check.close())
	}

	check.environment = append(slices.Clone(environment), "GOFLAGS="+flags, "GOWORK=off")

	err = command(ctx, check.directory, check.environment,
		"go", "mod", "edit", "-replace="+publicModule+"="+canonicalRoot)
	if err != nil {
		return check, errors.Join(err, check.close())
	}

	return check, nil
}

func copyMetadata(source, destination string) error {
	for _, name := range []string{moduleFilename, checksumFilename} {
		// #nosec G304 -- names are fixed module metadata files from the local CLI checkout.
		data, err := os.ReadFile(filepath.Join(source, name))
		if name == checksumFilename && os.IsNotExist(err) {
			continue
		}

		if err != nil {
			return verificationError{operation: "read CLI " + name, cause: err}
		}

		// #nosec G703 -- destination is this tool's owned temporary directory; names are fixed metadata files.
		err = os.WriteFile(filepath.Join(destination, name), data, moduleFileMode)
		if err != nil {
			return verificationError{operation: "copy CLI " + name, cause: err}
		}
	}

	return nil
}

func modfileFlags(environment []string, filename string) (string, error) {
	inherited := ""

	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(name, "GOFLAGS") {
			inherited = value
		}
	}

	flag := "-modfile=" + filename

	switch {
	case !strings.ContainsRune(flag, '\''):
		flag = "'" + flag + "'"
	case !strings.ContainsRune(flag, '"'):
		flag = "\"" + flag + "\""
	default:
		return "", verificationError{operation: "temporary modfile path contains both quote characters", cause: nil}
	}

	return strings.TrimSpace(inherited + " " + flag), nil
}

func runModule(
	ctx context.Context, root, module string, environment []string,
	program string, args []string, metadata bool,
) error {
	check, err := prepareModule(ctx, root, module, environment)
	if err != nil {
		return err
	}

	if program != "go" && check.temporaryModfile != "" {
		check.environment, err = check.workspaceEnvironment(ctx, root, environment)
		if err != nil {
			return errors.Join(err, check.close())
		}
	}

	err = command(ctx, check.directory, check.environment, program, args...)
	if err == nil && metadata && check.temporaryModfile != "" {
		err = check.tidyMatches(ctx)
	}

	return errors.Join(err, check.close())
}

// workspaceEnvironment avoids go/packages' module-disabled Go version probe,
// which rejects an inherited -modfile flag. The workspace is used solely by
// repository lint and leaves the published CLI's module metadata unchanged.
func (check moduleCheck) workspaceEnvironment(
	ctx context.Context, root string, environment []string,
) ([]string, error) {
	canonicalRoot, err := canonicalDirectory(root)
	if err != nil {
		return nil, err
	}

	workspaceEnvironment := slices.Clone(environment)
	workspaceEnvironment = append(workspaceEnvironment, "GOWORK=off")

	err = command(ctx, check.temporaryDirectory, workspaceEnvironment,
		"go", "work", "init", check.directory)
	if err != nil {
		return nil, err
	}

	workspaceEnvironment = append(workspaceEnvironment, "GOWORK="+filepath.Join(check.temporaryDirectory, "go.work"))

	err = command(ctx, check.temporaryDirectory, workspaceEnvironment,
		"go", "work", "edit", "-replace="+publicModule+"="+canonicalRoot)
	if err != nil {
		return nil, err
	}

	return workspaceEnvironment, nil
}

func canonicalDirectory(directory string) (string, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", verificationError{operation: "resolve verification directory", cause: err}
	}

	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", verificationError{operation: "resolve verification directory aliases", cause: err}
	}

	return canonical, nil
}

func (check moduleCheck) tidyMatches(ctx context.Context) error {
	version, err := check.pinnedSDKVersion(ctx)
	if err != nil {
		return err
	}

	err = command(ctx, check.directory, check.environment, "go", "mod", "edit", "-dropreplace="+publicModule)
	if err != nil {
		return err
	}

	for _, filename := range []string{moduleFilename, checksumFilename} {
		original, err := optionalMetadata(filepath.Join(check.directory, filename))
		if err != nil {
			return err
		}

		updated, err := optionalMetadata(filepath.Join(check.temporaryDirectory, filename))
		if err != nil {
			return err
		}

		original = bytes.ReplaceAll(original, []byte("\r\n"), []byte("\n"))
		if filename == checksumFilename {
			original, err = normalizeSDKChecksums(original, version)
			if err != nil {
				return err
			}

			updated, err = normalizeSDKChecksums(updated, version)
			if err != nil {
				return err
			}
		}

		if !bytes.Equal(original, updated) {
			message := "CLI " + filename + " is not tidy after local replacement is removed"

			return verificationError{operation: message, cause: nil}
		}
	}

	return nil
}

//nolint:tagliatelle // Preserve the Go tool's published go mod edit -json field names.
type goModuleRequirement struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
}

//nolint:tagliatelle // Preserve the Go tool's published go mod edit -json field names.
type goModuleMetadata struct {
	Require []goModuleRequirement `json:"Require"`
}

func (check moduleCheck) pinnedSDKVersion(ctx context.Context) (string, error) {
	// Go's parser reads the distributable metadata without resolving the unpublished SDK.
	// #nosec G204 -- fixed read-only Go command and repository module filename; no shell.
	cmd := exec.CommandContext(ctx, "go", "mod", "edit", "-json", filepath.Join(check.directory, moduleFilename))
	cmd.Dir = check.directory
	cmd.Env = append(slices.Clone(check.environment), "GOFLAGS=", "GOWORK=off", "PWD="+check.directory)

	output, err := cmd.Output()
	if err != nil {
		return "", verificationError{operation: "read pinned CLI SDK requirement", cause: err}
	}

	var metadata goModuleMetadata

	err = json.Unmarshal(output, &metadata)
	if err != nil {
		return "", verificationError{operation: "decode CLI module metadata", cause: err}
	}

	version := ""

	for _, requirement := range metadata.Require {
		if requirement.Path == publicModule {
			if version != "" || requirement.Version == "" {
				return "", verificationError{operation: "invalid pinned CLI SDK requirement", cause: nil}
			}

			version = requirement.Version
		}
	}

	if version == "" {
		return "", verificationError{operation: "missing pinned CLI SDK requirement", cause: nil}
	}

	return version, nil
}

// A local replacement removes its own module and go.mod checksums during tidy.
// Normalize only the two well-formed entries for the exact distributable pin.
// Actual checksum authenticity and complete published metadata remain enforced
// by the release workflow's replacement-free go mod tidy -diff.
func normalizeSDKChecksums(data []byte, version string) ([]byte, error) {
	seen := map[string]bool{}
	lines := bytes.Split(data, []byte("\n"))

	retained := make([][]byte, 0, len(lines))

	for _, line := range lines {
		fields := strings.Fields(string(line))
		if len(fields) == 0 || fields[0] != publicModule {
			retained = append(retained, line)

			continue
		}

		if len(fields) != 3 || strings.Join(fields, " ") != string(line) ||
			(fields[1] != version && fields[1] != version+"/go.mod") || seen[fields[1]] {
			return nil, verificationError{operation: "invalid or stale CLI SDK checksum", cause: nil}
		}

		checksum, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(fields[2], "h1:"))
		if err != nil || !strings.HasPrefix(fields[2], "h1:") || len(checksum) != sha256.Size {
			return nil, verificationError{operation: "malformed CLI SDK checksum", cause: err}
		}

		seen[fields[1]] = true
	}

	if len(seen) == 1 {
		return nil, verificationError{operation: "incomplete CLI SDK checksum pair", cause: nil}
	}

	return bytes.Join(retained, []byte("\n")), nil
}

func optionalMetadata(filename string) ([]byte, error) {
	// #nosec G304 -- caller selects fixed metadata filenames in the checkout or owned temporary directory.
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, verificationError{operation: "read module verification metadata", cause: err}
	}

	return data, nil
}

func (check moduleCheck) close() error {
	if check.temporaryDirectory == "" {
		return nil
	}

	err := os.RemoveAll(check.temporaryDirectory)
	if err != nil {
		return verificationError{operation: "remove owned temporary CLI modfile directory", cause: err}
	}

	return nil
}
