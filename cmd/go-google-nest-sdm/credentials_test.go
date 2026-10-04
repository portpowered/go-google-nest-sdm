package main

import (
	"strings"
	"testing"
)

func TestCredentialsExplicitInput(t *testing.T) {
	t.Parallel()

	lookup := func(key string) string {
		if key == "SDM_ACCESS_TOKEN" {
			return "environment"
		}

		return ""
	}

	value, err := readCredentials("-", strings.NewReader(`{"access_token":"file","refresh_token":"refresh"}`), lookup)
	if err != nil || value.AccessToken != "file" || value.RefreshToken != "refresh" {
		t.Fatalf("credentials: %+v, %v", value, err)
	}
}

func TestCredentialsRejectMalformedWithoutLeaking(t *testing.T) {
	t.Parallel()

	for _, input := range []string{`{"secret":"private"}`, `{"access_token":"private"} {}`, `private`, `null`, `[]`} {
		_, err := readCredentials("-", strings.NewReader(input), func(string) string { return "" })
		if err == nil || strings.Contains(err.Error(), "private") {
			t.Fatalf("error exposes input or accepted invalid credentials: %v", err)
		}
	}
}
