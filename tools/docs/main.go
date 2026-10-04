// Command docs checks internal links throughout a rendered documentation site.
package main

import (
	"errors"
	"flag"
	"fmt"
	"html"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var hrefPattern = regexp.MustCompile(`\shref="([^"]+)"`)

var errMissingLink = errors.New("missing internal link")

func main() {
	root := flag.String("site", "site", "rendered site directory")
	base := flag.String("base", "/go-google-nest-sdm", "published project base path")

	flag.Parse()

	err := checkSite(*root, *base)
	if err == nil {
		err = checkNavigation(*root)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func checkSite(root, base string) error {
	var failures []error

	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || filepath.Ext(filename) != ".html" {
			return nil
		}

		//nolint:gosec // G304: filenames come from walking the explicitly selected rendered site.
		body, readErr := os.ReadFile(filename)
		if readErr != nil {
			return fmt.Errorf("read page: %w", readErr)
		}

		for _, match := range hrefPattern.FindAllSubmatch(body, -1) {
			linkErr := checkLink(root, base, filename, html.UnescapeString(string(match[1])))
			if linkErr != nil {
				failures = append(failures, linkErr)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk site: %w", err)
	}

	return errors.Join(failures...)
}

func checkLink(root, base, source, href string) error {
	parsed, err := url.Parse(href)
	if err != nil {
		return fmt.Errorf("%s: invalid link: %w", source, err)
	}

	if parsed.Scheme != "" || parsed.Host != "" || parsed.Path == "" {
		return nil
	}

	var target string

	if strings.HasPrefix(parsed.Path, "/") {
		path := strings.TrimPrefix(parsed.Path, base)
		target = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(path, "/")))
	} else {
		target = filepath.Join(filepath.Dir(source), filepath.FromSlash(parsed.Path))
	}

	for _, candidate := range []string{target, target + ".html", filepath.Join(target, "index.html")} {
		//nolint:gosec // G703: this read-only checker inspects paths from generated site links.
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			return nil
		}
	}

	return fmt.Errorf("%s: %w %s", source, errMissingLink, href)
}
