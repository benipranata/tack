package scan

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Provider is a function that qualifies as a provider for Type.
type Provider struct {
	Func *types.Func
	Pkg  *packages.Package
	Type types.Type
}

// NearMiss is a Provide*-prefixed function that did not qualify as a
// provider, kept so unsatisfied-dependency errors can point at it.
type NearMiss struct {
	Func *types.Func
	Pkg  *packages.Package
	// Type is the function's first result type, the best-effort guess at
	// what it was meant to provide. Never nil (functions with no results
	// aren't recorded).
	Type types.Type
}

// Index is a single provider scope: every qualifying provider found while
// scanning a set of packages, keyed by exact return type, plus every
// Provide*-prefixed near-miss for diagnostics.
type Index struct {
	providers  []Provider
	nearMisses []NearMiss
}

// NewIndex returns an empty Index.
func NewIndex() *Index {
	return &Index{}
}

// Add scans every package-scope, exported function in pkgs and adds
// qualifying providers to the index. It returns an error naming both
// candidates the moment two functions in this call (combined with anything
// already added) qualify for the same type.
func (idx *Index) Add(pkgs ...*packages.Package) error {
	for _, pkg := range pkgs {
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			fn, ok := scope.Lookup(name).(*types.Func)
			if !ok || !fn.Exported() {
				continue
			}
			sig, ok := fn.Type().(*types.Signature)
			if !ok || sig.Recv() != nil {
				continue
			}
			if t, ok := qualifies(sig); ok {
				if err := idx.addProvider(Provider{Func: fn, Pkg: pkg, Type: t}); err != nil {
					return err
				}
				continue
			}
			if strings.HasPrefix(fn.Name(), "Provide") && sig.Results().Len() > 0 {
				idx.nearMisses = append(idx.nearMisses, NearMiss{Func: fn, Pkg: pkg, Type: sig.Results().At(0).Type()})
			}
		}
	}
	return nil
}

func (idx *Index) addProvider(p Provider) error {
	if existing, ok := idx.Lookup(p.Type); ok {
		return fmt.Errorf("tack: ambiguous provider for %s: both %s.%s and %s.%s qualify",
			types.TypeString(p.Type, nil),
			existing.Pkg.PkgPath, existing.Func.Name(),
			p.Pkg.PkgPath, p.Func.Name())
	}
	idx.providers = append(idx.providers, p)
	return nil
}

// Lookup returns the provider for t, if any, using exact type identity.
func (idx *Index) Lookup(t types.Type) (Provider, bool) {
	for _, p := range idx.providers {
		if types.Identical(p.Type, t) {
			return p, true
		}
	}
	return Provider{}, false
}

// NearMisses returns every recorded near-miss whose best-guess type is
// identical to t.
func (idx *Index) NearMisses(t types.Type) []NearMiss {
	var out []NearMiss
	for _, nm := range idx.nearMisses {
		if types.Identical(nm.Type, t) {
			out = append(out, nm)
		}
	}
	return out
}
