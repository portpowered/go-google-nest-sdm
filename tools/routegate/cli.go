package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const cliRoot = "cmd/go-google-nest-sdm"

const listenHelper = "listenLoopback"
const receiveHelper = "receiveAuthorization"

const loginHelper = "loginAccount"

const browserHelper = "openBrowser"
const launcherParameter = "launch"
const serveMethod = "Serve"
const processConstructor = "CommandContext"
const netImport = "net"
const urlImport = "net/url"
const roundTripMethod = "RoundTrip"

// These incoming loopback and OS browser seams are not provider HTTP clients.
// Their exact implementations are inventoried, while every CLI source file is
// still inspected for additional outbound network or process primitives.
const cliConstructors = `package main
func defaultLoginDependencies() loginDependencies {
 return loginDependencies{listen: listenLoopback, openBrowser: openBrowser}
}
func listenLoopback(ctx context.Context, address string) (net.Listener, error) {
 config := new(net.ListenConfig)
 listener, err := config.Listen(ctx, "tcp", address)
 return listener, wrapError(err)
}
func openBrowser(ctx context.Context, address string) error {
 var command *exec.Cmd
 switch runtime.GOOS {
 case "windows": command = exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", address)
 case "darwin": command = exec.CommandContext(ctx, "open", address)
 default: command = exec.CommandContext(ctx, "xdg-open", address)
 }
 return wrapError(command.Run())
}
func loginRedirect(value string) (*url.URL, error) {
 redirect, err := url.Parse(value)
 if err != nil || redirect.Scheme != "http" || redirect.User != nil || redirect.RawQuery != "" ||
 redirect.Fragment != "" || redirect.Port() == "" || redirect.Port() == "0" || redirect.Path == "" {
 return nil, errLoginRedirect
 }
 host := redirect.Hostname()
 if host != "localhost" && host != "127.0.0.1" && host != "::1" { return nil, errLoginRedirect }
 return redirect, nil
}`

func auditCLI(root string) error {
	expected, err := parser.ParseFile(token.NewFileSet(), "cli-inventory.go", cliConstructors, 0)
	if err != nil {
		return fmt.Errorf("parse CLI inventory: %w", err)
	}

	seen := map[string]bool{}

	err = filepath.WalkDir(filepath.Join(root, "cmd"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parse CLI source: %w", parseErr)
		}

		return auditCLIFile(file, filepath.ToSlash(path), expected, seen)
	})
	if err != nil {
		return fmt.Errorf("audit CLI network edges: %w", err)
	}

	for _, name := range []string{"defaultLoginDependencies", listenHelper, browserHelper, "loginRedirect",
		loginHelper, receiveHelper} {
		if !seen[name] {
			return fmt.Errorf("%w: CLI authorization boundary %s absent", errRouteInvalid, name)
		}
	}

	return nil
}

func auditCLIFile(file *ast.File, path string, expected *ast.File, seen map[string]bool) error {
	imports := map[string]string{}

	for _, imported := range file.Imports {
		owner, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return fmt.Errorf("decode CLI import: %w", err)
		}

		alias := filepath.Base(owner)
		if imported.Name != nil {
			alias = imported.Name.Name
		}

		imports[alias] = owner

		if owner == "crypto/tls" || strings.Contains(owner, "websocket") || strings.Contains(owner, "cloud.google.com") {
			return fmt.Errorf("%w: unregistered CLI network import", errRouteInvalid)
		}
	}

	err := verifyCLIImports(path, imports)
	if err != nil {
		return err
	}

	for _, declaration := range file.Decls {
		function, recognized := declaration.(*ast.FuncDecl)
		if !recognized {
			err := auditCLINode(declaration, nil, imports)
			if err != nil {
				return err
			}

			continue
		}

		err = verifyCLIConstructors(function, path, expected, seen)
		if err != nil {
			return err
		}

		if function.Name.Name == loginHelper || function.Name.Name == receiveHelper {
			if !strings.HasSuffix(path, cliRoot+"/login.go") {
				return fmt.Errorf("%w: CLI authorization boundary moved", errRouteInvalid)
			}

			err := verifyCLIFlow(function)
			if err != nil {
				return err
			}

			seen[function.Name.Name] = true
		}

		err := auditCLINode(function, function, imports)
		if err != nil {
			return err
		}
	}

	return nil
}

func verifyCLIImports(path string, imports map[string]string) error {
	if !strings.HasSuffix(path, cliRoot+"/login.go") {
		return nil
	}

	for alias, owner := range map[string]string{netImport: netImport, "http": "net/http", "exec": "os/exec",
		"url": urlImport, "runtime": "runtime", "sdm": module + "/pkg/sdm"} {
		if imports[alias] != owner {
			return fmt.Errorf("%w: CLI boundary import %s changed", errRouteInvalid, alias)
		}
	}

	return nil
}

func verifyCLIConstructors(function *ast.FuncDecl, path string, expected *ast.File, seen map[string]bool) error {
	for _, constructor := range expected.Decls {
		owned, recognized := constructor.(*ast.FuncDecl)
		if !recognized {
			return fmt.Errorf("%w: invalid CLI constructor inventory", errRouteInvalid)
		}

		if function.Name.Name != owned.Name.Name {
			continue
		}

		if !strings.HasSuffix(path, cliRoot+"/login.go") || !sameSyntax(function, owned) {
			return fmt.Errorf("%w: CLI %s differs from inventoried boundary", errRouteInvalid, function.Name.Name)
		}

		seen[function.Name.Name] = true
	}

	return nil
}

func sameSyntax(actual, expected ast.Node) bool {
	var left, right bytes.Buffer

	leftErr := format.Node(&left, token.NewFileSet(), actual)
	rightErr := format.Node(&right, token.NewFileSet(), expected)

	return leftErr == nil && rightErr == nil && bytes.Equal(left.Bytes(), right.Bytes())
}

func auditCLINode(node ast.Node, function *ast.FuncDecl, imports map[string]string) error {
	var failure error

	ast.Inspect(node, func(current ast.Node) bool {
		if failure != nil {
			return false
		}

		selector, recognized := current.(*ast.SelectorExpr)
		if !recognized {
			return true
		}

		name := ""
		if function != nil {
			name = function.Name.Name
		}

		owner, identifier := selector.X.(*ast.Ident)
		if identifier && owner.Obj == nil {
			allowed := cliImportedSelector(imports[owner.Name], selector.Sel.Name, name)
			if !allowed {
				failure = fmt.Errorf("%w: unregistered CLI imported network/process primitive %s",
					errRouteInvalid, selector.Sel.Name)
			}
		}

		if slices.Contains([]string{"Do", roundTripMethod, "Dial", "DialContext", "Listen", "ListenAndServe",
			serveMethod, "Command", processConstructor}, selector.Sel.Name) {
			allowed := name == listenHelper && selector.Sel.Name == "Listen" ||
				name == browserHelper && selector.Sel.Name == processConstructor ||
				name == receiveHelper && selector.Sel.Name == serveMethod && identifier && owner.Name == "server"
			if !allowed {
				failure = fmt.Errorf("%w: unregistered CLI network/process call %s", errRouteInvalid, selector.Sel.Name)
			}
		}

		return true
	})

	return failure
}

func cliImportedSelector(owner, selector, function string) bool {
	switch owner {
	case netImport:
		return selector == "Listener" || selector == "ListenConfig" && function == listenHelper ||
			selector == "JoinHostPort" && function == loginHelper
	case "net/http":
		return slices.Contains([]string{"ResponseWriter", "Request", "Server", "MethodGet", "Error",
			"StatusNotFound", "StatusConflict", "StatusBadRequest", "ErrServerClosed"}, selector)
	case "os/exec":
		return function == browserHelper && (selector == "Cmd" || selector == processConstructor)
	default:
		return true
	}
}
