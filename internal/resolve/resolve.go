// Package resolve ties internal/config and internal/scan together: for each
// configured output variant, it resolves every accessor method's dependency
// type to exactly one provider, validating interface shape and type
// nilability along the way.
package resolve

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/benipranata/tack/internal/config"
	"github.com/benipranata/tack/internal/scan"
	"golang.org/x/tools/go/packages"
)

// Method is one resolved accessor method on a configured interface.
type Method struct {
	Name     string
	Type     types.Type
	Provider scan.Provider
}

// Interface is one fully resolved, validated output variant of a configured
// interface, ready for code emission.
type Interface struct {
	PkgDir    string // target's module-relative package directory, e.g. "src/state"
	Pkg       *packages.Package
	IfaceName string
	Output    config.OutputConfig

	// EffectiveDir is Output.EffectiveDir(target): the module-relative
	// directory this variant writes into and scans as its local scope.
	EffectiveDir string
	// OutputPkg is the loaded package at EffectiveDir, or nil when that
	// directory has no .go files yet (a freshly scaffolded or otherwise
	// empty output package) — in which case this variant's local scope is
	// empty and its package name is derived from EffectiveDir's basename.
	OutputPkg *packages.Package

	Methods []Method
}

type methodShape struct {
	Name string
	Type types.Type
}

// ResolveAll loads and resolves every output variant configured in cfg: the
// global provider scope is built once from cfg.Providers, each target's
// interface is parsed and validated once, and each output variant gets its
// own local scope built from its effective directory (cached by directory,
// so variants sharing one aren't rescanned).
func ResolveAll(cfg *config.Config) ([]*Interface, error) {
	loadDirs, err := collectLoadDirs(cfg)
	if err != nil {
		return nil, err
	}
	loaded, err := scan.LoadDirs(cfg.ModuleRoot, loadDirs)
	if err != nil {
		return nil, err
	}

	globalPkgs := make([]*packages.Package, 0, len(cfg.Providers))
	for _, dir := range cfg.Providers {
		globalPkgs = append(globalPkgs, loaded[dir])
	}
	global := scan.NewIndex()
	if err := global.Add(globalPkgs...); err != nil {
		return nil, err
	}

	localIndexes := map[string]*scan.Index{}

	var results []*Interface
	for _, t := range cfg.Targets {
		targetPkg, ok := loaded[t.Package]
		if !ok {
			return nil, fmt.Errorf("tack: target %s: package could not be loaded", t.Package)
		}

		shapes, err := parseAndValidateInterface(targetPkg, t.Package, t.Interface)
		if err != nil {
			return nil, err
		}

		for _, o := range t.Output {
			dir := o.EffectiveDir(t)
			local, ok := localIndexes[dir]
			if !ok {
				local = scan.NewIndex()
				if pkg, loadedOK := loaded[dir]; loadedOK {
					if err := local.Add(pkg); err != nil {
						return nil, err
					}
				}
				localIndexes[dir] = local
			}

			iface, err := resolveVariant(targetPkg, t, o, dir, loaded[dir], shapes, local, global)
			if err != nil {
				return nil, err
			}
			results = append(results, iface)
		}
	}
	return results, nil
}

// collectLoadDirs returns the deduplicated set of module-relative
// directories that need packages.Load: every global provider directory,
// every target's own package directory, and every output variant's
// effective directory that actually has .go files (a directory with none —
// not yet scaffolded, or scaffolded but still empty — contributes nothing
// and is never loaded).
func collectLoadDirs(cfg *config.Config) ([]string, error) {
	seen := map[string]bool{}
	var dirs []string
	add := func(d string) {
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}

	for _, p := range cfg.Providers {
		add(p)
	}
	for _, t := range cfg.Targets {
		add(t.Package)
	}
	for _, t := range cfg.Targets {
		for _, o := range t.Output {
			dir := o.EffectiveDir(t)
			if seen[dir] {
				continue
			}
			has, err := dirHasGoFiles(cfg.ModuleRoot, dir)
			if err != nil {
				return nil, err
			}
			if has {
				add(dir)
			}
		}
	}
	return dirs, nil
}

func dirHasGoFiles(moduleRoot, dir string) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(moduleRoot, dir))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("tack: read directory %s: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true, nil
		}
	}
	return false, nil
}

// parseAndValidateInterface locates ifaceName in pkg and validates every
// accessor method's shape (zero parameters, exactly one nilable result),
// returning the methods in source declaration order. This is shared across
// every output variant of a target, since method shape doesn't depend on
// where a variant writes its output.
func parseAndValidateInterface(pkg *packages.Package, pkgDir, ifaceName string) ([]methodShape, error) {
	methodOrder, ifaceType, err := findInterface(pkg, ifaceName)
	if err != nil {
		return nil, err
	}

	explicit := map[string]*types.Func{}
	for i := 0; i < ifaceType.NumExplicitMethods(); i++ {
		f := ifaceType.ExplicitMethod(i)
		explicit[f.Name()] = f
	}

	shapes := make([]methodShape, 0, len(methodOrder))
	for _, name := range methodOrder {
		fn, ok := explicit[name]
		if !ok {
			return nil, fmt.Errorf("tack: interface %s.%s: could not resolve method %s", pkgDir, ifaceName, name)
		}
		sig := fn.Type().(*types.Signature)
		if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
			return nil, fmt.Errorf("tack: interface %s.%s: method %s must take zero parameters and return exactly one result", pkgDir, ifaceName, name)
		}
		resultType := sig.Results().At(0).Type()
		if !isNilable(resultType) {
			return nil, fmt.Errorf("tack: interface %s.%s: method %s returns non-nilable type %s; the generated test helper can only zero-check pointer, interface, map, slice, channel, or func types", pkgDir, ifaceName, name, types.TypeString(resultType, nil))
		}
		shapes = append(shapes, methodShape{Name: name, Type: resultType})
	}
	return shapes, nil
}

func resolveVariant(targetPkg *packages.Package, t config.TargetConfig, o config.OutputConfig, effectiveDir string, outputPkg *packages.Package, shapes []methodShape, local, global *scan.Index) (*Interface, error) {
	methods := make([]Method, 0, len(shapes))
	for _, s := range shapes {
		provider, err := resolveType(t.Package, t.Interface, o.Name, s.Name, s.Type, o.LocalScan, local, global)
		if err != nil {
			return nil, err
		}
		methods = append(methods, Method{Name: s.Name, Type: s.Type, Provider: provider})
	}

	return &Interface{
		PkgDir:       t.Package,
		Pkg:          targetPkg,
		IfaceName:    t.Interface,
		Output:       o,
		EffectiveDir: effectiveDir,
		OutputPkg:    outputPkg,
		Methods:      methods,
	}, nil
}

func resolveType(pkgDir, ifaceName, variantName, methodName string, t types.Type, localScan bool, local, global *scan.Index) (scan.Provider, error) {
	if localScan {
		if p, ok := local.Lookup(t); ok {
			return p, nil
		}
	}
	if p, ok := global.Lookup(t); ok {
		return p, nil
	}

	var nearMisses []scan.NearMiss
	if localScan {
		nearMisses = append(nearMisses, local.NearMisses(t)...)
	}
	nearMisses = append(nearMisses, global.NearMisses(t)...)

	msg := fmt.Sprintf("tack: interface %s.%s output %q: method %s: no provider found for %s", pkgDir, ifaceName, variantName, methodName, types.TypeString(t, nil))
	if len(nearMisses) > 0 {
		names := make([]string, len(nearMisses))
		for i, nm := range nearMisses {
			names[i] = fmt.Sprintf("%s.%s", nm.Pkg.PkgPath, nm.Func.Name())
		}
		msg += fmt.Sprintf(" (near-miss candidates with a non-qualifying signature: %s)", strings.Join(names, ", "))
	}
	return scan.Provider{}, fmt.Errorf("%s", msg)
}

// findInterface locates ifaceName's type declaration in pkg, returning its
// methods in source declaration order alongside the type-checked
// *types.Interface used to resolve each method's real signature.
func findInterface(pkg *packages.Package, ifaceName string) ([]string, *types.Interface, error) {
	obj := pkg.Types.Scope().Lookup(ifaceName)
	if obj == nil {
		return nil, nil, fmt.Errorf("tack: package %s: interface %s not found", pkg.PkgPath, ifaceName)
	}
	tn, ok := obj.(*types.TypeName)
	if !ok {
		return nil, nil, fmt.Errorf("tack: package %s: %s is not a type", pkg.PkgPath, ifaceName)
	}
	ifaceType, ok := tn.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, nil, fmt.Errorf("tack: package %s: %s is not an interface", pkg.PkgPath, ifaceName)
	}

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name.Name != ifaceName {
					continue
				}
				astIface, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok {
					return nil, nil, fmt.Errorf("tack: package %s: %s is not declared as an interface literal", pkg.PkgPath, ifaceName)
				}
				var order []string
				for _, field := range astIface.Methods.List {
					if len(field.Names) == 0 {
						return nil, nil, fmt.Errorf("tack: package %s: interface %s: embedded interface members are not supported", pkg.PkgPath, ifaceName)
					}
					for _, n := range field.Names {
						order = append(order, n.Name)
					}
				}
				return order, ifaceType, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("tack: package %s: could not locate source declaration for interface %s", pkg.PkgPath, ifaceName)
}

func isNilable(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Interface, *types.Map, *types.Slice, *types.Chan, *types.Signature:
		return true
	default:
		return false
	}
}
