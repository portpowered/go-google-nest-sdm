package main

import (
	"path/filepath"
	"testing"
)

func TestRuntimeSourceCommentUsesPortablePaths(t *testing.T) {
	t.Parallel()

	const expected = "Generated from api/external/pubsub.openapi.yaml; DO NOT EDIT."

	paths := []string{
		"api/external/pubsub.openapi.yaml",
		"api\\external\\pubsub.openapi.yaml",
		filepath.Join("api", "external", "pubsub.openapi.yaml"),
	}

	for _, path := range paths {
		if result := runtimeSourceComment(path); result != expected {
			t.Fatalf("source %q: got %q, want %q", path, result, expected)
		}
	}
}
