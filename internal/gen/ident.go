// Package gen emits the generated wiring file for a resolved interface.
package gen

import (
	"fmt"
	"go/types"
)

// Allocator hands out collision-free identifiers for one generated file,
// shared across local variables, cleanup variables, and import aliases so
// neither can ever shadow the other.
type Allocator struct {
	used map[string]bool
}

var keywords = []string{
	"break", "case", "chan", "const", "continue", "default", "defer", "else",
	"fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
	"map", "package", "range", "return", "select", "struct", "switch", "type", "var",
}

// NewAllocator returns an Allocator seeded with Go keywords, predeclared
// identifiers (via go/types.Universe), and the fixed names the generated
// code always uses.
func NewAllocator() *Allocator {
	a := &Allocator{used: map[string]bool{}}
	for _, kw := range keywords {
		a.used[kw] = true
	}
	for _, name := range types.Universe.Names() {
		a.used[name] = true
	}
	for _, seed := range []string{"ctx", "cleanup", "cleanups", "err", "i"} {
		a.used[seed] = true
	}
	return a
}

// Alloc returns a name based on base, suffixed with a numeric counter
// starting at 2 if base is already in use, and reserves the result.
func (a *Allocator) Alloc(base string) string {
	name := base
	for n := 2; a.used[name]; n++ {
		name = fmt.Sprintf("%s%d", base, n)
	}
	a.used[name] = true
	return name
}

// Reserve marks name as already in use without collision suffixing, for a
// name fixed elsewhere (e.g. an identifier already assigned by another
// Allocator) that this one must still avoid colliding with.
func (a *Allocator) Reserve(name string) {
	a.used[name] = true
}
