package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const checksumTestVersion = "v0.1.0"
const checksumDriftMode = "other-drift"

// Synthetic hashes test format validation; published checksum authenticity is
// established separately by replacement-free release verification.
func sdkChecksumPair(version string) []byte {
	checksum := "h1:" + base64.StdEncoding.EncodeToString(make([]byte, 32))

	return []byte(publicModule + " " + version + " " + checksum + "\n" +
		publicModule + " " + version + "/go.mod " + checksum + "\n")
}

func TestSDKChecksumNormalizationIsLimitedToPinnedPair(t *testing.T) {
	t.Parallel()

	valid := sdkChecksumPair(checksumTestVersion)

	other := []byte("example.com/other v1.0.0 h1:other-is-retained-exactly\n")

	for _, original := range [][]byte{nil, other, valid, append(bytes.Clone(valid), other...)} {
		normalized, err := normalizeSDKChecksums(original, checksumTestVersion)
		if err != nil {
			t.Fatal(err)
		}

		expected := []byte(nil)
		if bytes.Contains(original, other) {
			expected = other
		}

		if !bytes.Equal(normalized, expected) {
			t.Fatalf("normalization changed other metadata: %q", normalized)
		}
	}

	invalid := [][]byte{
		sdkChecksumPair("v0.0.9"),
		[]byte(publicModule + "\n"),
		bytes.ReplaceAll(valid, []byte("h1:"), []byte("h2:")),
		bytes.ReplaceAll(valid, []byte("h1:"), []byte("h1:!")),
		bytes.ReplaceAll(valid, []byte(base64.StdEncoding.EncodeToString(make([]byte, 32))),
			[]byte(base64.StdEncoding.EncodeToString([]byte("short")))),
		bytes.ReplaceAll(valid, []byte(" "), []byte("  ")),
		append(bytes.Clone(valid), valid...),
		[]byte(strings.Split(string(valid), "\n")[0] + "\n"),
	}
	for _, original := range invalid {
		_, err := normalizeSDKChecksums(original, checksumTestVersion)
		if err == nil {
			t.Fatalf("invalid SDK checksum passed normalization: %q", original)
		}
	}
}

func TestCLITidyOverlayAcceptsPublishedSDKChecksumsButRejectsOtherDrift(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"unpublished", "published-pair", checksumDriftMode} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			root, _ := setupLocalCLI(t)
			if mode != "unpublished" {
				err := os.WriteFile(filepath.Join(root, cliModule, checksumFilename),
					sdkChecksumPair(checksumTestVersion), moduleFileMode)
				if err != nil {
					t.Fatal(err)
				}
			}

			check, err := prepareModule(t.Context(), root, cliModule, append(os.Environ(), "GOWORK=off"))
			if err != nil {
				t.Fatal(err)
			}

			t.Cleanup(func() {
				closeErr := check.close()
				if closeErr != nil {
					t.Error(closeErr)
				}
			})

			err = command(t.Context(), check.directory, check.environment, "go", "mod", "tidy")
			if err != nil {
				t.Fatal(err)
			}

			if mode == checksumDriftMode {
				err = os.WriteFile(filepath.Join(check.temporaryDirectory, checksumFilename),
					[]byte("example.com/other v1.0.0 h1:unexpected\n"), moduleFileMode)
				if err != nil {
					t.Fatal(err)
				}
			}

			err = check.tidyMatches(t.Context())
			if (err != nil) != (mode == checksumDriftMode) {
				t.Fatalf("mode %s: %v", mode, err)
			}
		})
	}
}
