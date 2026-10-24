package render

import (
	"go/types"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/benipranata/tack/internal/config"
	"github.com/benipranata/tack/internal/load"
)

// scope is the file-level naming state shared by an output's constructor and test double,
// so both agree on the struct name and on how packages are referred to.
type scope struct {
	out        load.Output
	names      *namer
	imports    *imports
	structName string
}

// newScope must allocate the same names in the same order for a given output every time.
func newScope(out load.Output) *scope {
	names := newNamer(true, "ctx", "err", "cleanup", "cleanups", "i", "s", "t", "fields",
		out.Name, config.DoubleName(out.Name), config.DoubleFields(out.Name))
	if out.Target.Path == out.Package.Path {
		names.take(out.Interface)
	}

	base := lowerName(structBase(out.Name))
	if base == out.Interface {
		base += "Impl"
	}
	return &scope{out: out, names: names, imports: newImports(names), structName: names.take(base)}
}

// structBase names the struct after the constructor, so outputs of one interface in one package never share it:
// NewService gives Service, and a name without a New prefix is used as it is.
func structBase(name string) string {
	for _, p := range []string{"New", "new"} {
		rest, ok := strings.CutPrefix(name, p)
		if r, _ := utf8.DecodeRuneInString(rest); ok && unicode.IsUpper(r) {
			return rest
		}
	}
	return name
}

// pkg returns the name to refer to a package by, or "" for the output package itself.
func (s *scope) pkg(path, name string) string {
	if path == s.out.Package.Path {
		return ""
	}
	return s.imports.use(path, name)
}

// typeString writes t as it is spelled inside the output package.
func (s *scope) typeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string { return s.pkg(p.Path(), p.Name()) })
}

// qualify prefixes name with its package, or returns it bare when it lives in the output package.
func (s *scope) qualify(p load.Package, name string) string {
	if alias := s.pkg(p.Path, p.Name); alias != "" {
		return alias + "." + name
	}
	return name
}
