package main

import (
	"path/filepath"
	"strconv"
	"strings"
)

// repositoryEnvironment trusts only this checkout for child Git processes,
// including Git invoked indirectly by Go's VCS stamping. Existing Git settings
// retain their indices and values; no user or global configuration is changed.
func repositoryEnvironment(environment []string, root string) ([]string, error) {
	count := 0

	for _, entry := range environment {
		name, value, found := strings.Cut(entry, "=")
		if !found || !strings.EqualFold(name, "GIT_CONFIG_COUNT") || value == "" {
			continue
		}

		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, verificationError{operation: "parse inherited Git configuration count", cause: err}
		}

		if parsed < 0 {
			return nil, verificationError{operation: "negative inherited Git configuration count", cause: nil}
		}

		count = parsed
	}

	const addedEntries = 3

	result := make([]string, 0, len(environment)+addedEntries)

	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "GIT_CONFIG_COUNT") {
			result = append(result, entry)
		}
	}

	index := strconv.Itoa(count)
	result = append(result,
		"GIT_CONFIG_COUNT="+strconv.Itoa(count+1),
		"GIT_CONFIG_KEY_"+index+"=safe.directory",
		"GIT_CONFIG_VALUE_"+index+"="+filepath.ToSlash(root))

	return result, nil
}
