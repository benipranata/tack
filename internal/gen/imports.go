package gen

import (
	"go/types"
	"path"
	"sort"
)

// importEntry is one resolved import: Alias is the explicit local
// identifier to write when the plain import (bound to the path's last
// component) wouldn't already resolve to Ident.
type importEntry struct {
	Path  string
	Ident string
	Alias string // "" when no explicit alias is needed
}

// importSet accumulates the imports a generated file needs, assigning each
// referenced package a collision-free local identifier via alloc, and
// omitting the file's own target package (which needs no import).
type importSet struct {
	targetPkgPath string
	alloc         *Allocator
	byPath        map[string]*importEntry
}

func newImportSet(targetPkgPath string, alloc *Allocator) *importSet {
	return &importSet{targetPkgPath: targetPkgPath, alloc: alloc, byPath: map[string]*importEntry{}}
}

// add registers the package at pkgPath (whose declared name is pkgName) as
// needed, returning the local identifier to use to refer to it, or "" if
// pkgPath is the target package itself.
func (s *importSet) add(pkgPath, pkgName string) string {
	if pkgPath == s.targetPkgPath {
		return ""
	}
	if e, ok := s.byPath[pkgPath]; ok {
		return e.Ident
	}
	ident := s.alloc.Alloc(pkgName)
	entry := &importEntry{Path: pkgPath, Ident: ident}
	if ident != path.Base(pkgPath) {
		entry.Alias = ident
	}
	s.byPath[pkgPath] = entry
	return ident
}

// entries returns every registered import, sorted by path.
func (s *importSet) entries() []importEntry {
	out := make([]importEntry, 0, len(s.byPath))
	for _, e := range s.byPath {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// qualifier returns a types.Qualifier that renders types using this set's
// identifiers, registering any package it's asked about along the way.
func (s *importSet) qualifier() types.Qualifier {
	return func(pkg *types.Package) string {
		return s.add(pkg.Path(), pkg.Name())
	}
}
