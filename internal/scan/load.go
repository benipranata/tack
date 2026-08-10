// Package scan discovers Provide* functions in Go packages via go/packages
// and go/types, and indexes the ones that qualify as providers by their
// return type.
package scan

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// LoadDirs loads and type-checks the Go packages at the given
// module-relative directories, resolved against moduleRoot, in a single
// shared session. Loading everything in one call is required for type
// identity: two separate packages.Load calls type-check shared dependencies
// independently, so types.Identical would never match a type imported by
// both, even though it's the same source. Duplicate directories are loaded
// once.
func LoadDirs(moduleRoot string, relDirs []string) (map[string]*packages.Package, error) {
	unique := make([]string, 0, len(relDirs))
	seen := map[string]bool{}
	for _, d := range relDirs {
		if seen[d] {
			continue
		}
		seen[d] = true
		unique = append(unique, d)
	}
	if len(unique) == 0 {
		return map[string]*packages.Package{}, nil
	}

	patterns := make([]string, len(unique))
	for i, d := range unique {
		patterns[i] = "./" + d
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax |
			packages.NeedTypesSizes,
		Dir: moduleRoot,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("tack: load packages: %w", err)
	}

	result := make(map[string]*packages.Package, len(unique))
	for _, relDir := range unique {
		want := filepath.Clean(filepath.Join(moduleRoot, relDir))
		var match *packages.Package
		for _, pkg := range pkgs {
			if len(pkg.GoFiles) == 0 {
				continue
			}
			if filepath.Clean(filepath.Dir(pkg.GoFiles[0])) == want {
				match = pkg
				break
			}
		}
		if match == nil {
			return nil, fmt.Errorf("tack: load %s: package not found", relDir)
		}
		if len(match.Errors) > 0 {
			msgs := make([]string, len(match.Errors))
			for i, e := range match.Errors {
				msgs[i] = e.Error()
			}
			return nil, fmt.Errorf("tack: load %s: %s", relDir, strings.Join(msgs, "; "))
		}
		result[relDir] = match
	}
	return result, nil
}

// LoadDir is a single-directory convenience wrapper around LoadDirs.
func LoadDir(moduleRoot, relDir string) (*packages.Package, error) {
	pkgs, err := LoadDirs(moduleRoot, []string{relDir})
	if err != nil {
		return nil, err
	}
	return pkgs[relDir], nil
}
