package load

import (
	"errors"
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/benipranata/tack/internal/config"
)

// candidate is a provider function and the type it returns.
type candidate struct {
	Provider
	result types.Type
}

// providerLayers lists provider candidates in lookup order: the target's provider, the global providers, the output package.
func providerLayers(cfg *config.Config, t config.Target, o config.Output, pkgs map[string]*packages.Package) [][]candidate {
	var layers [][]candidate
	if t.Provider != "" {
		layers = append(layers, providersIn(pkgs, t.Provider, true))
	}

	var global []candidate
	for _, dir := range cfg.Providers {
		global = append(global, providersIn(pkgs, dir, true)...)
	}
	return append(layers, global, providersIn(pkgs, o.Package, false))
}

// providersIn returns the provider functions of the package in dir; a missing package has none.
func providersIn(pkgs map[string]*packages.Package, dir string, exportedOnly bool) []candidate {
	pkg, ok := pkgs[dir]
	if !ok || pkg.Types == nil {
		return nil
	}

	info := newPackage(pkgs, dir, pkg.Module.Path)
	var cs []candidate
	for _, name := range pkg.Types.Scope().Names() {
		fn, ok := pkg.Types.Scope().Lookup(name).(*types.Func)
		if !ok || exportedOnly && !fn.Exported() {
			continue
		}
		if result, ok := providerResult(fn.Type().(*types.Signature)); ok {
			cs = append(cs, candidate{Provider{info, name}, result})
		}
	}
	return cs
}

// providerResult returns T for a func(context.Context) (T, func(), error).
func providerResult(sig *types.Signature) (types.Type, bool) {
	params, results := sig.Params(), sig.Results()
	if sig.TypeParams().Len() > 0 || params.Len() != 1 || results.Len() != 3 {
		return nil, false
	}
	cleanup, ok := results.At(1).Type().Underlying().(*types.Signature)
	if !ok || cleanup.Params().Len() != 0 || cleanup.Results().Len() != 0 {
		return nil, false
	}
	isError := types.Identical(results.At(2).Type(), types.Universe.Lookup("error").Type())
	return results.At(0).Type(), isContext(params.At(0).Type()) && isError
}

func isContext(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "context" && n.Obj().Name() == "Context"
}

// resolveMethods binds every wanted method to a provider and reports all failures together.
func resolveMethods(layers [][]candidate, wants []Method) ([]Method, error) {
	var errs []error
	methods := make([]Method, len(wants))
	for i, w := range wants {
		p, err := pickProvider(layers, w)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		methods[i] = Method{Name: w.Name, Type: w.Type, Provider: p}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return methods, nil
}

// pickProvider takes the first layer with a match; two matches in one layer are ambiguous.
func pickProvider(layers [][]candidate, want Method) (Provider, error) {
	typ := types.TypeString(want.Type, func(p *types.Package) string { return p.Name() })
	for _, layer := range layers {
		var found []Provider
		for _, c := range layer {
			if types.Identical(c.result, want.Type) {
				found = append(found, c.Provider)
			}
		}
		switch len(found) {
		case 0:
		case 1:
			return found[0], nil
		default:
			names := make([]string, len(found))
			for i, p := range found {
				names[i] = p.Package.Name + "." + p.Name
			}
			return Provider{}, fmt.Errorf("%s(): %s is provided by %s", want.Name, typ, strings.Join(names, ", "))
		}
	}
	return Provider{}, fmt.Errorf("%s(): no provider returns %s", want.Name, typ)
}
