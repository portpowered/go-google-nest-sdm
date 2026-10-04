package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderedLinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	page := filepath.Join(root, "index.html")
	body := `<style data-href="not-a-link"></style><a href="/project/guide">Guide</a>` +
		`<a href="https://example.com">External</a>`

	err := os.WriteFile(page, []byte(body), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = checkSite(root, "/project")
	if err == nil {
		t.Fatal("missing linked page passed verification")
	}

	err = os.WriteFile(filepath.Join(root, "guide.html"), []byte(`<a href="/project/">Home</a>`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = checkSite(root, "/project")
	if err != nil {
		t.Fatal(err)
	}
}
