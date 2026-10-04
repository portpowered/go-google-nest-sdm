package main

import (
	"path/filepath"
	"strings"
)

func (input source) references(root, output string, value any) (any, error) {
	switch value := value.(type) {
	case []any:
		for index, child := range value {
			rewritten, err := input.references(root, output, child)
			if err != nil {
				return nil, err
			}

			value[index] = rewritten
		}
	case map[string]any:
		for key, child := range value {
			if reference, ok := child.(string); key == "$ref" && ok {
				rewritten, err := input.reference(root, output, reference)
				if err != nil {
					return nil, err
				}

				value[key] = rewritten

				continue
			}

			rewritten, err := input.references(root, output, child)
			if err != nil {
				return nil, err
			}

			value[key] = rewritten
		}
	}

	return value, nil
}

func (input source) reference(root, output, reference string) (string, error) {
	if after, ok := strings.CutPrefix(reference, "#/components/"); ok {
		const minimumComponentParts = 2

		parts := strings.Split(after, "/")
		if len(parts) < minimumComponentParts {
			return "", bundleError{operation: "invalid local component reference " + reference, cause: nil}
		}

		parts[1] = input.prefix + "__" + parts[1]

		return "#/components/" + strings.Join(parts, "/"), nil
	}

	if strings.HasPrefix(reference, "#") || strings.Contains(reference, "://") {
		return reference, nil
	}

	filename, fragment, hasFragment := strings.Cut(reference, "#")
	resolved := filepath.Join(root, filepath.Dir(input.filename), filepath.FromSlash(filename))

	relative, err := filepath.Rel(filepath.Join(root, filepath.Dir(output)), resolved)
	if err != nil {
		return "", bundleError{operation: "rebase documentation reference", cause: err}
	}

	result := filepath.ToSlash(relative)
	if hasFragment {
		result += "#" + fragment
	}

	return result, nil
}
