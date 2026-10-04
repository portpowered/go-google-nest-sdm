package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// credentials belong to one CLI invocation; the SDK never stores them.
//
//nolint:tagliatelle // Persisted OAuth credential files intentionally use the documented snake_case keys.
type credentials struct {
	AccessToken       string `json:"access_token,omitempty"`
	ClientID          string `json:"client_id,omitempty"`
	ClientSecret      string `json:"client_secret,omitempty"`
	RefreshToken      string `json:"refresh_token,omitempty"`
	AuthorizationCode string `json:"authorization_code,omitempty"`
	RedirectURI       string `json:"redirect_uri,omitempty"`
	CodeVerifier      string `json:"code_verifier,omitempty"`
}

func readCredentials(path string, input io.Reader, lookup func(string) string) (credentials, error) {
	value := credentials{
		AccessToken:       lookup("SDM_ACCESS_TOKEN"),
		ClientID:          lookup("SDM_CLIENT_ID"),
		ClientSecret:      lookup("SDM_CLIENT_SECRET"),
		RefreshToken:      lookup("SDM_REFRESH_TOKEN"),
		AuthorizationCode: lookup("SDM_AUTHORIZATION_CODE"),
		RedirectURI:       lookup("SDM_REDIRECT_URI"),
		CodeVerifier:      lookup("SDM_CODE_VERIFIER"),
	}
	if path == "" {
		return value, nil
	}

	if path != "-" {
		data, err := os.ReadFile(path) // #nosec G304 -- This is the explicitly requested CLI credential input file.
		if err != nil {
			return credentials{}, fmt.Errorf("open credentials: %w", err)
		}

		input = bytes.NewReader(data)
	}

	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()

	object := &value

	err := decoder.Decode(&object)

	if err != nil || object == nil {
		return credentials{}, errCredentialsObject
	}

	var extra any

	err = decoder.Decode(&extra)
	if !errors.Is(err, io.EOF) {
		return credentials{}, errCredentialsTrailingData
	}

	return value, nil
}
