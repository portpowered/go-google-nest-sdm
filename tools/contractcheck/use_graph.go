package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
)

const inventoryModule = "github.com/portpowered/go-google-nest-sdm"

func localInventoryPackage(path string) bool {
	return strings.HasPrefix(path, inventoryModule+"/pkg/") ||
		strings.HasPrefix(path, inventoryModule+"/internal/") || path == inventoryModule+"/api"
}

func inventoryIndex(entries []inventoryEntry) map[string]int {
	index := map[string]int{}
	for position, entry := range entries {
		index[entry.Package+"."+entry.Declaration] = position
	}

	return index
}

func objectInventoryKey(object types.Object) string {
	if object == nil || object.Pkg() == nil || object.Parent() != object.Pkg().Scope() {
		return ""
	}

	return object.Pkg().Path() + "." + object.Name()
}

func inventoryObjectKeys(object types.Object) []string {
	keys := []string{}

	switch value := object.(type) {
	case *types.TypeName:
		keys = append(keys, objectInventoryKey(value))
	case *types.Const:
		keys = append(keys, objectInventoryKey(value))
	case *types.Var:
		keys = append(keys, inventoryTypeKeys(value.Type())...)
	}

	return keys
}

func inventoryTypeKeys(value types.Type) []string {
	switch kind := value.(type) {
	case *types.Named:
		return []string{objectInventoryKey(kind.Obj())}
	case *types.Alias:
		return []string{objectInventoryKey(kind.Obj())}
	case *types.Pointer:
		return inventoryTypeKeys(kind.Elem())
	case *types.Slice:
		return inventoryTypeKeys(kind.Elem())
	case *types.Array:
		return inventoryTypeKeys(kind.Elem())
	case *types.Map:
		return append(inventoryTypeKeys(kind.Key()), inventoryTypeKeys(kind.Elem())...)
	default:
		return nil
	}
}

func recordResolvedUses(root string, resolved inventoryResolution, entries []inventoryEntry,
	index map[string]int, edges map[int]map[int]bool) error {
	for _, file := range resolved.Sources {
		filename := resolved.FileSet.Position(file.Pos()).Filename

		relative, err := filepath.Rel(root, filename)
		if err != nil {
			return fmt.Errorf("resolve inventory source location: %w", err)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if !ok {
				return true
			}

			object, used := resolved.Info.Uses[identifier]
			if !used {
				return true
			} // Definition identifiers never count as uses.

			location := resolved.FileSet.Position(identifier.Pos())

			use := fmt.Sprintf("%s:%d", filepath.ToSlash(relative), location.Line)

			use += enclosingFunction(file, identifier)

			for _, key := range inventoryObjectKeys(object) {
				position, exists := index[key]
				if !exists {
					continue
				}

				entries[position].Uses = append(entries[position].Uses, use)
				if !strings.HasSuffix(relative, ".gen.go") {
					entries[position].ProductionUses = append(entries[position].ProductionUses, use)
				}
			}

			return true
		})
		recordTypeEdges(file, resolved.Info, index, edges)
	}

	return nil
}

func enclosingFunction(file *ast.File, identifier *ast.Ident) string {
	for _, declaration := range file.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if isFunction && identifier.Pos() >= function.Pos() && identifier.Pos() <= function.End() {
			return " @ " + function.Name.Name
		}
	}

	return ""
}

func recordTypeEdges(file *ast.File, info *types.Info, index map[string]int, edges map[int]map[int]bool) {
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range group.Specs {
			model, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			owner, exists := index[objectInventoryKey(info.Defs[model.Name])]
			if !exists {
				continue
			}

			if edges[owner] == nil {
				edges[owner] = map[int]bool{}
			}

			ast.Inspect(model.Type, func(node ast.Node) bool {
				identifier, ok := node.(*ast.Ident)
				if !ok {
					return true
				}

				for _, key := range inventoryObjectKeys(info.Uses[identifier]) {
					if child, exists := index[key]; exists && child != owner {
						edges[owner][child] = true
					}
				}

				return true
			})
		}
	}
}

func traceWireRoots(entries []inventoryEntry, edges map[int]map[int]bool) {
	for position := range entries {
		entry := &entries[position]
		entry.Uses = sortedUnique(entry.Uses)

		entry.ProductionUses = sortedUnique(entry.ProductionUses)

		if len(entry.ProductionUses) == 0 {
			continue
		}

		visited := map[int]bool{}
		traceRoot(position, position, entries, edges, visited)
	}

	for position := range entries {
		entry := &entries[position]

		entry.WireRoots = sortedUnique(entry.WireRoots)

		switch {
		case entry.Declaration == "ChannelSDMEvents" || entry.Declaration == "ChannelSDMEventsName":
			entry.Disposition = "provider topic address; no direct topic opening; " +
				"delivery uses subscription REST pull or HTTP push"
		case len(entry.ProductionUses) > 0:
			entry.Disposition = "direct production reference"
		case len(entry.WireRoots) > 0:
			entry.Disposition = "nested model reachable from production reference"
		default:
			entry.Disposition = "schema-generated export with no repository production reference"
		}
	}
}

func traceRoot(root, current int, entries []inventoryEntry, edges map[int]map[int]bool, visited map[int]bool) {
	if visited[current] {
		return
	}

	visited[current] = true

	for _, use := range entries[root].ProductionUses {
		location := entries[root].Package + "." + entries[root].Declaration + " @ " + use
		entries[current].WireRoots = append(entries[current].WireRoots, location)
	}

	for child := range edges[current] {
		traceRoot(root, child, entries, edges, visited)
	}
}

func sortedUnique(values []string) []string {
	slices.Sort(values)

	return slices.Compact(values)
}
