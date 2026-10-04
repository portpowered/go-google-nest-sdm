package main

import (
	"encoding/json"
	"io"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

// Validate the original JSON before typed decoding so absent required fields
// cannot silently become legitimate zero temperatures or empty command objects.
//
//nolint:ireturn // Return the generated model chosen by the caller while sharing raw command validation.
func readCommandParams[T any](path string, input io.Reader, command sdm.CommandName) (T, error) {
	var value T

	data, err := readParams[json.RawMessage](path, input)
	if err != nil {
		return value, wrapError(err)
	}

	err = sdm.ValidateCommandParams(command, data)
	if err != nil {
		return value, wrapError(err)
	}

	err = json.Unmarshal(data, &value)

	return value, wrapError(err)
}
