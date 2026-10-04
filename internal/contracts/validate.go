// Package contracts validates JSON against generated canonical API contracts.
package contracts

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/portpowered/go-google-nest-sdm/api"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaOrigin = "https://sdm.local/contracts/"

// Error identifies a contract failure and preserves the validation cause.
type Error struct {
	Document  string
	Component string
	Cause     error
}

// Error describes the failed contract.
func (err *Error) Error() string {
	return fmt.Sprintf("contract %s#%s: %v", err.Document, err.Component, err.Cause)
}

// Unwrap returns the schema compilation, JSON decoding or validation cause.
func (err *Error) Unwrap() error { return err.Cause }

type registry struct {
	mutex    sync.Mutex
	compiler *jsonschema.Compiler
	schemas  map[string]*jsonschema.Schema
}

// Compilation is serialized because the compiler owns mutable reference caches.
// Compiled schemas are immutable and validation runs without a lock.
//
//nolint:gochecknoglobals // A synchronized process-wide cache avoids recompiling immutable schemas for each event.
var runtimeRegistry = newRegistry()

var (
	errInvalidReference = errors.New("invalid contract reference")
	errOutsideInventory = errors.New("contract reference outside embedded inventory")
)

func newRegistry() *registry {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	compiler.AssertFormat()
	compiler.UseLoader(offlineLoader{})

	return &registry{mutex: sync.Mutex{}, compiler: compiler, schemas: make(map[string]*jsonschema.Schema)}
}

// Validate rejects malformed known fields while allowing future fields and enum
// values wherever the canonical schema allows them. Optional trait fields remain
// optional for both resource snapshots and partial event updates (SCHEMA-07).
func Validate(document, component string, data []byte) error {
	schema, err := runtimeRegistry.compile(document, component)
	if err == nil {
		var value any

		value, err = jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err == nil {
			err = schema.Validate(value)
		}
	}

	if err != nil {
		return &Error{Document: document, Component: component, Cause: err}
	}

	return nil
}

func (r *registry) compile(document, component string) (*jsonschema.Schema, error) {
	if strings.ContainsAny(document, "/\\#?") || strings.ContainsAny(component, "/~#?") || component == "" {
		return nil, errInvalidReference
	}

	location := schemaOrigin + document + "#/components/schemas/" + component

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if schema, exists := r.schemas[location]; exists {
		return schema, nil
	}

	schema, err := r.compiler.Compile(location)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	r.schemas[location] = schema

	return schema, nil
}

type offlineLoader struct{}

func (offlineLoader) Load(location string) (any, error) {
	parsed, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("parse contract URL: %w", err)
	}

	if parsed.Scheme != "https" || parsed.Host != "sdm.local" || parsed.RawQuery != "" ||
		!strings.HasPrefix(parsed.Path, "/contracts/") {
		return nil, errOutsideInventory
	}

	data, err := api.RuntimeSchemas.ReadFile(strings.TrimPrefix(parsed.Path, "/"))
	if err != nil {
		return nil, fmt.Errorf("read embedded contract: %w", err)
	}

	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode embedded contract: %w", err)
	}

	return value, nil
}
