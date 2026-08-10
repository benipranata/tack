// Package resolve ties internal/config and internal/scan together: for each
// configured interface, it resolves every accessor method's dependency type
// to exactly one provider, validating interface shape and type nilability
// along the way.
package resolve

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
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

// Interface is a fully resolved, validated configured interface, ready for
// code emission.
type Interface struct {
	PkgDir    string // module-relative directory, e.g. "src/iface"
	Pkg       *packages.Package
	IfaceName string
	Config    config.InterfaceConfig
	Methods   []Method
}

// ResolveAll loads and resolves every interface configured in cfg: the
// global provider scope is built once from cfg.Providers, and a local scope
// is built once per target package directory, then shared across every
// interface configured in that directory.
func ResolveAll(cfg *config.Config) ([]*Interface, error) {
	pkgDirs := sortedKeys(cfg.Packages)

	// Load every directory involved (global providers + every target
	// package) in one shared session, so types.Identical can match a
	// dependency type shared across scopes.
	allDirs := make([]string, 0, len(cfg.Providers)+len(pkgDirs))
	allDirs = append(allDirs, cfg.Providers...)
	allDirs = append(allDirs, pkgDirs...)
	loaded, err := scan.LoadDirs(cfg.ModuleRoot, allDirs)
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

	var results []*Interface
	for _, pkgDir := range pkgDirs {
		targetPkg := loaded[pkgDir]
		local := scan.NewIndex()
		if err := local.Add(targetPkg); err != nil {
			return nil, err
		}

		for _, ifaceName := range sortedKeys(cfg.Packages[pkgDir]) {
			icfg := cfg.Packages[pkgDir][ifaceName]
			iface, err := resolveInterface(targetPkg, pkgDir, ifaceName, icfg, local, global)
			if err != nil {
				return nil, err
			}
			results = append(results, iface)
		}
	}
	return results, nil
}

func resolveInterface(pkg *packages.Package, pkgDir, ifaceName string, icfg config.InterfaceConfig, local, global *scan.Index) (*Interface, error) {
	methodOrder, ifaceType, err := findInterface(pkg, ifaceName)
	if err != nil {
		return nil, err
	}

	explicit := map[string]*types.Func{}
	for i := 0; i < ifaceType.NumExplicitMethods(); i++ {
		f := ifaceType.ExplicitMethod(i)
		explicit[f.Name()] = f
	}

	methods := make([]Method, 0, len(methodOrder))
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

		provider, err := resolveType(pkgDir, ifaceName, name, resultType, icfg, local, global)
		if err != nil {
			return nil, err
		}
		methods = append(methods, Method{Name: name, Type: resultType, Provider: provider})
	}

	return &Interface{PkgDir: pkgDir, Pkg: pkg, IfaceName: ifaceName, Config: icfg, Methods: methods}, nil
}

func resolveType(pkgDir, ifaceName, methodName string, t types.Type, icfg config.InterfaceConfig, local, global *scan.Index) (scan.Provider, error) {
	if icfg.LocalScan {
		if p, ok := local.Lookup(t); ok {
			return p, nil
		}
	}
	if p, ok := global.Lookup(t); ok {
		return p, nil
	}

	var nearMisses []scan.NearMiss
	if icfg.LocalScan {
		nearMisses = append(nearMisses, local.NearMisses(t)...)
	}
	nearMisses = append(nearMisses, global.NearMisses(t)...)

	msg := fmt.Sprintf("tack: interface %s.%s: method %s: no provider found for %s", pkgDir, ifaceName, methodName, types.TypeString(t, nil))
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

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
