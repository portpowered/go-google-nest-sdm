package main

import (
	"bytes"
	"context"
	"errors"
	"os"
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

	directory, err := os.MkdirTemp("", "sdm-cli-verify-")
	if err != nil {
		return check, verificationError{operation: "create CLI verification modfile directory", cause: err}
	}

	check.temporaryDirectory = directory
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

	err = command(ctx, check.directory, check.environment, "go", "mod", "edit", "-replace="+publicModule+"="+root)
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
	workspaceEnvironment := slices.Clone(environment)
	workspaceEnvironment = append(workspaceEnvironment, "GOWORK=off")

	err := command(ctx, check.temporaryDirectory, workspaceEnvironment,
		"go", "work", "init", check.directory)
	if err != nil {
		return nil, err
	}

	workspaceEnvironment = append(workspaceEnvironment, "GOWORK="+filepath.Join(check.temporaryDirectory, "go.work"))

	err = command(ctx, check.temporaryDirectory, workspaceEnvironment,
		"go", "work", "edit", "-replace="+publicModule+"="+root)
	if err != nil {
		return nil, err
	}

	return workspaceEnvironment, nil
}

func (check moduleCheck) tidyMatches(ctx context.Context) error {
	err := command(ctx, check.directory, check.environment, "go", "mod", "edit", "-dropreplace="+publicModule)
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

		if !bytes.Equal(bytes.ReplaceAll(original, []byte("\r\n"), []byte("\n")), updated) {
			message := "CLI " + filename + " is not tidy after local replacement is removed"

			return verificationError{operation: message, cause: nil}
		}
	}

	return nil
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
