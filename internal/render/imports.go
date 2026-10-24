package render

import (
	"slices"
	"strings"
)

// importSpec is one import line; Gap marks the first non-standard-library import, which follows a blank line.
type importSpec struct {
	Alias string
	Path  string
	Gap   bool
}

type importName struct {
	pkg   string
	alias string
}

// imports collects the packages a generated file refers to and the name each is known by.
type imports struct {
	names  *namer
	byPath map[string]importName
}

func newImports(names *namer) *imports {
	return &imports{names: names, byPath: map[string]importName{}}
}

// use records the package and returns the name to refer to it by.
func (i *imports) use(path, name string) string {
	if n, ok := i.byPath[path]; ok {
		return n.alias
	}
	alias := i.names.take(name)
	i.byPath[path] = importName{pkg: name, alias: alias}
	return alias
}

// specs lists the imports with the standard library first, each group sorted by path.
func (i *imports) specs() []importSpec {
	paths := make([]string, 0, len(i.byPath))
	for p := range i.byPath {
		paths = append(paths, p)
	}
	slices.SortFunc(paths, func(a, b string) int {
		if sa, sb := isStd(a), isStd(b); sa != sb {
			if sa {
				return -1
			}
			return 1
		}
		return strings.Compare(a, b)
	})

	specs := make([]importSpec, len(paths))
	for n, p := range paths {
		specs[n] = importSpec{Path: p, Gap: n > 0 && isStd(paths[n-1]) && !isStd(p)}
		if name := i.byPath[p]; name.alias != name.pkg {
			specs[n].Alias = name.alias
		}
	}
	return specs
}

func isStd(p string) bool {
	first, _, _ := strings.Cut(p, "/")
	return !strings.Contains(first, ".")
}
