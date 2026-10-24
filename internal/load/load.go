// Package load type-checks the packages named in a config and binds each interface method to a provider.
package load

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/benipranata/tack/internal/config"
)

// NeedDeps type-checks from source, so a package that does not compile, such as one with a stale generated file, still loads.
const mode = packages.NeedName | packages.NeedFiles | packages.NeedTypes | packages.NeedSyntax | packages.NeedImports | packages.NeedDeps | packages.NeedModule

// Load resolves every output of cfg; root is the module root that config package paths are relative to.
func Load(root string, cfg *config.Config) ([]Output, error) {
	pkgs, err := loadPackages(root, packageDirs(cfg))
	if err != nil {
		return nil, err
	}
	if err := checkPackages(cfg, pkgs); err != nil {
		return nil, err
	}

	var outs []Output
	var errs []error
	for i, t := range cfg.Targets {
		at := fmt.Sprintf("targets[%d]", i)
		target := pkgs[t.Package]
		wants, err := interfaceMethods(target.Types, t.Interface)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.interface: %w", at, err))
			continue
		}

		for j, o := range t.Outputs {
			methods, err := resolveMethods(providerLayers(cfg, o, pkgs), wants)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s.output[%d]: %w", at, j, err))
				continue
			}
			outs = append(outs, Output{
				Name:      o.Name,
				Filename:  o.Filename,
				Package:   newPackage(pkgs, o.Package, target.Module.Path),
				Target:    newPackage(pkgs, t.Package, target.Module.Path),
				Interface: t.Interface,
				Methods:   methods,
			})
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return outs, nil
}

// packageDirs returns every package directory the config mentions, sorted and without duplicates.
func packageDirs(cfg *config.Config) []string {
	dirs := slices.Clone(cfg.Providers)
	for _, t := range cfg.Targets {
		dirs = append(dirs, t.Package)
		for _, o := range t.Outputs {
			dirs = append(dirs, o.Package)
			if o.Provider != "" {
				dirs = append(dirs, o.Provider)
			}
		}
	}
	slices.Sort(dirs)
	return slices.Compact(dirs)
}

// loadPackages loads the directories that exist, keyed by their path relative to the module root.
func loadPackages(root string, dirs []string) (map[string]*packages.Package, error) {
	var patterns []string
	for _, d := range dirs {
		if info, err := os.Stat(filepath.Join(root, d)); err == nil && info.IsDir() {
			patterns = append(patterns, "./"+d)
		}
	}

	byDir := map[string]*packages.Package{}
	if len(patterns) == 0 {
		return byDir, nil
	}
	pkgs, err := packages.Load(&packages.Config{Mode: mode, Dir: root}, patterns...)
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		if p.Module != nil {
			byDir[moduleDir(p)] = p
		}
	}
	return byDir, nil
}

func moduleDir(p *packages.Package) string {
	dir := strings.TrimPrefix(strings.TrimPrefix(p.PkgPath, p.Module.Path), "/")
	if dir == "" {
		return "."
	}
	return dir
}

// checkPackages requires every target and provider package to exist and to parse.
func checkPackages(cfg *config.Config, pkgs map[string]*packages.Package) error {
	var errs []error
	for i, dir := range cfg.Providers {
		err := requirePackage(pkgs, dir, fmt.Sprintf("providers[%d]", i))
		if err != nil {
			errs = append(errs, err)
		}
	}
	for i, t := range cfg.Targets {
		at := fmt.Sprintf("targets[%d]", i)
		if err := requirePackage(pkgs, t.Package, at+".package"); err != nil {
			errs = append(errs, err)
		}
		for j, o := range t.Outputs {
			if o.Provider == "" {
				continue
			}
			if err := requirePackage(pkgs, o.Provider, fmt.Sprintf("%s.output[%d].provider", at, j)); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// requirePackage ignores type errors: a stale generated file must not stop tack from regenerating it.
func requirePackage(pkgs map[string]*packages.Package, dir, at string) error {
	p, ok := pkgs[dir]
	if !ok {
		return fmt.Errorf("%s: package %q not found", at, dir)
	}
	var errs []error
	for _, e := range p.Errors {
		if e.Kind != packages.TypeError {
			errs = append(errs, e)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%s: %w", at, err)
	}
	return nil
}

// newPackage describes dir, which may not exist yet when it is an output package.
func newPackage(pkgs map[string]*packages.Package, dir, module string) Package {
	if p, ok := pkgs[dir]; ok && p.Name != "" {
		return Package{Dir: dir, Path: p.PkgPath, Name: p.Name}
	}
	full := path.Join(module, dir)
	return Package{Dir: dir, Path: full, Name: path.Base(full)}
}
