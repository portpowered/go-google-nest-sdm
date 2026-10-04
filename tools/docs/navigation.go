package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errMissingNavigation = errors.New("missing API navigation group")

func navigationGroups() []string {
	return []string{"devices", "structures", "rooms", "pubsub", "auth", "media"}
}

func checkNavigation(root string) error {
	// #nosec G304 -- this checker reads the fixed rendered API landing page in the selected output directory.
	page, err := os.ReadFile(filepath.Join(root, "docs", "index.html"))
	if err != nil {
		return fmt.Errorf("read rendered API navigation: %w", err)
	}

	// Closed sidebar folders retain their route tree in Next's rendered data even
	// when their individual links are not expanded in the initial HTML.
	for _, group := range navigationGroups() {
		prefix := "/docs/openapi/" + group + "/"
		if !strings.Contains(string(page), prefix) {
			return fmt.Errorf("%w: %s", errMissingNavigation, group)
		}
	}

	return nil
}
