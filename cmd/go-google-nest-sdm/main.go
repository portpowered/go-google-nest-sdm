// Command go-google-nest-sdm exercises the public Smart Device Management SDK.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const defaultTimeout = 30 * time.Second

const commandParts = 2

type invocation struct {
	credentialsPath string
	paramsPath      string
	export          bool
	timeout         time.Duration
	resource        string
	count           int
}

type application struct {
	client sdm.Client
	in     io.Reader
	out    io.Writer
	lookup func(string) string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	client, err := httptransport.NewClient()
	if err == nil {
		err = (application{client: client, in: os.Stdin, out: os.Stdout, lookup: os.Getenv}).run(ctx, os.Args[1:])
	}

	stop()

	if err != nil {
		// Provider errors retain their class without echoing credentials or server bodies.
		writeErr := json.NewEncoder(os.Stderr).Encode(describeError(err))
		if writeErr != nil {
			os.Exit(1)
		}

		os.Exit(1)
	}
}

type errorOutput struct {
	Error      string        `json:"error"`
	Kind       sdm.ErrorKind `json:"kind,omitempty"`
	StatusCode int           `json:"statusCode,omitempty"`
}

func describeError(err error) errorOutput {
	output := errorOutput{Error: err.Error(), Kind: "", StatusCode: 0}

	var provider *sdm.Error

	if errors.As(err, &provider) {
		output.Kind = provider.Kind
		output.StatusCode = provider.StatusCode
	}

	return output
}

func (app application) run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		_, err := io.WriteString(app.out, helpText)

		return wrapError(err)
	}

	if len(args) < commandParts {
		return errCommandRequired
	}

	options, err := parseInvocation(args[commandParts:], app.out)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}

	if err != nil {
		return wrapError(err)
	}

	account, err := readCredentials(options.credentialsPath, app.in, app.lookup)
	if err != nil {
		return wrapError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, options.timeout)
	defer cancel()

	result, err := app.execute(ctx, args[0], args[1], options, account)
	if err != nil {
		return wrapError(err)
	}

	if _, streamed := result.(streamedOutput); streamed {
		return nil
	}

	return wrapError(json.NewEncoder(app.out).Encode(result))
}

func parseInvocation(args []string, output io.Writer) (invocation, error) {
	var options invocation

	flags := flag.NewFlagSet("operation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	flags.StringVar(&options.credentialsPath, "credentials", "",
		"credential JSON file, or - for stdin; defaults to SDM_* environment")
	flags.StringVar(&options.paramsPath, "params", "", "typed command parameters JSON file, or - for stdin")
	flags.StringVar(&options.resource, "resource", "", "full resource name or parent enterprise/structure name")
	flags.BoolVar(&options.export, "export", false,
		"explicitly include credentials and media access secrets in JSON output")
	flags.DurationVar(&options.timeout, "timeout", defaultTimeout, "operation deadline")
	flags.IntVar(&options.count, "count", 1, "number of events to process before closing the pull session")

	err := flags.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			flags.SetOutput(output)
			flags.PrintDefaults()
		}

		return options, wrapError(err)
	}

	if flags.NArg() != 0 {
		return options, errUnexpectedArgument
	}

	if options.timeout <= 0 {
		return options, errInvalidTimeout
	}

	if options.count < 1 {
		return options, errInvalidCount
	}

	if options.credentialsPath == "-" && options.paramsPath == "-" {
		return options, errSharedStdin
	}

	return options, nil
}

//nolint:ireturn // The concrete generated command model is selected by the caller's type parameter.
func readParams[T any](path string, input io.Reader) (T, error) {
	var value T
	if path == "" {
		return value, errParamsRequired
	}

	if path != "-" {
		data, err := os.ReadFile(path) // #nosec G304 -- This is the explicitly requested CLI parameter input file.
		if err != nil {
			return value, fmt.Errorf("open parameters: %w", err)
		}

		input = bytes.NewReader(data)
	}

	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(&value)
	if err != nil {
		return value, errParamsType
	}

	var extra any

	err = decoder.Decode(&extra)
	if !errors.Is(err, io.EOF) {
		return value, errParamsTrailingData
	}

	return value, nil
}

const helpText = `go-google-nest-sdm GROUP OPERATION [flags]

Authentication: auth exchange, refresh, export
Discovery: devices list, get; structures list, get; rooms list, get
Controls: fan timer; thermostat mode, eco, heat, cool, range
Media: camera image, rtsp-start, rtsp-extend, rtsp-stop,
       webrtc-start, webrtc-extend, webrtc-stop
Events: events pull, decode

Use --resource enterprises/PROJECT/devices/DEVICE (or the list parent).
Controls require --params FILE containing that SDK command's JSON parameters.
Credentials: SDM_ACCESS_TOKEN, SDM_CLIENT_ID, SDM_CLIENT_SECRET,
SDM_REFRESH_TOKEN, SDM_AUTHORIZATION_CODE, SDM_REDIRECT_URI; alternatively
--credentials FILE containing snake_case keys, or --credentials - for stdin.
Secret values are never command arguments. --export explicitly includes
credentials or media access secrets; ordinary results redact those values.
--timeout 30s sets the deadline. Ctrl+C cancels and closes owned sessions.
Use GROUP OPERATION --help to display common flags.
`
