package load

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"

	"github.com/benipranata/tack/internal/config"
)

const serviceSrc = `
package svc

import "log"

type Service interface{ Logger() *log.Logger }
`

func providerFile(pkg string) string {
	const src = `
package %s

import (
	"context"
	"log"
)

func ProvideLogger(ctx context.Context) (*log.Logger, func(), error) { return nil, nil, nil }
`
	return fmt.Sprintf(src, pkg)
}

func pkgOf(dir, name string) Package {
	return Package{Dir: dir, Path: testModule + "/" + dir, Name: name}
}

func TestLoad(t *testing.T) {
	svc := pkgOf("svc", "svc")
	out := config.Output{Name: "NewService", Package: "svc", Filename: "service_gen.go"}
	target := func(outs ...config.Output) config.Target {
		return config.Target{Package: "svc", Interface: "Service", Outputs: outs}
	}
	withTest := func(o Output) Output {
		o.Test = true
		return o
	}
	named := func(o Output, name, filename string) Output {
		o.Name, o.Filename = name, filename
		return o
	}
	output := func(p Package, providerDir, providerPkg string) Output {
		return Output{
			Name:      "NewService",
			Filename:  "service_gen.go",
			Package:   p,
			Target:    svc,
			Interface: "Service",
			Methods: []Method{{
				Name:     "Logger",
				Provider: Provider{Package: pkgOf(providerDir, providerPkg), Name: "ProvideLogger"},
			}},
		}
	}

	tests := []struct {
		name    string
		files   map[string]string
		noMod   bool
		cfg     config.Config
		want    []Output
		wantErr []string
	}{
		{
			name:  "global provider",
			files: map[string]string{"svc/svc.go": serviceSrc, "global/g.go": providerFile("global")},
			cfg:   config.Config{Providers: []string{"global"}, Targets: []config.Target{target(out)}},
			want:  []Output{output(svc, "global", "global")},
		},
		{
			name: "output provider wins over global",
			files: map[string]string{
				"svc/svc.go":    serviceSrc,
				"global/g.go":   providerFile("global"),
				"special/sp.go": providerFile("special"),
			},
			cfg: config.Config{Providers: []string{"global"}, Targets: []config.Target{target(
				config.Output{Name: "NewService", Package: "svc", Filename: "service_gen.go", Provider: "special"},
			)}},
			want: []Output{output(svc, "special", "special")},
		},
		{
			name:  "test double setting is carried to the output",
			files: map[string]string{"svc/svc.go": serviceSrc, "global/g.go": providerFile("global")},
			cfg: config.Config{Providers: []string{"global"}, Targets: []config.Target{target(
				config.Output{Name: "NewService", Package: "svc", Filename: "service_gen.go", Test: ptr(true)},
				config.Output{Name: "NewOther", Package: "svc", Filename: "other_gen.go", Test: ptr(false)},
			)}},
			want: []Output{
				withTest(output(svc, "global", "global")),
				named(output(svc, "global", "global"), "NewOther", "other_gen.go"),
			},
		},
		{
			name: "each output of one target uses its own provider",
			files: map[string]string{
				"svc/svc.go":         serviceSrc,
				"global/g.go":        providerFile("global"),
				"prod/prod.go":       providerFile("prod"),
				"staging/staging.go": providerFile("staging"),
			},
			cfg: config.Config{Providers: []string{"global"}, Targets: []config.Target{target(
				config.Output{Name: "NewProd", Package: "svc", Filename: "prod_gen.go", Provider: "prod"},
				config.Output{Name: "NewStaging", Package: "svc", Filename: "staging_gen.go", Provider: "staging"},
				config.Output{Name: "NewDefault", Package: "svc", Filename: "default_gen.go"},
			)}},
			want: []Output{
				named(output(svc, "prod", "prod"), "NewProd", "prod_gen.go"),
				named(output(svc, "staging", "staging"), "NewStaging", "staging_gen.go"),
				named(output(svc, "global", "global"), "NewDefault", "default_gen.go"),
			},
		},
		{
			name: "output package is the last resort",
			files: map[string]string{
				"svc/svc.go":       serviceSrc,
				"svc/prod/prod.go": providerFile("prod"),
			},
			cfg: config.Config{Targets: []config.Target{
				target(config.Output{Name: "NewService", Package: "svc/prod", Filename: "service_gen.go"}),
			}},
			want: []Output{output(pkgOf("svc/prod", "prod"), "svc/prod", "prod")},
		},
		{
			name:  "output package that does not exist yet",
			files: map[string]string{"svc/svc.go": serviceSrc, "global/g.go": providerFile("global")},
			cfg: config.Config{Providers: []string{"global"}, Targets: []config.Target{
				target(config.Output{Name: "NewService", Package: "svc/prod", Filename: "service_gen.go"}),
			}},
			want: []Output{output(pkgOf("svc/prod", "prod"), "global", "global")},
		},
		{
			name:  "a target with two outputs",
			files: map[string]string{"svc/svc.go": serviceSrc, "global/g.go": providerFile("global")},
			cfg: config.Config{Providers: []string{"global"}, Targets: []config.Target{
				target(out, config.Output{Name: "NewOther", Package: "svc", Filename: "other_gen.go"}),
			}},
			want: []Output{
				output(svc, "global", "global"),
				{
					Name:      "NewOther",
					Filename:  "other_gen.go",
					Package:   svc,
					Target:    svc,
					Interface: "Service",
					Methods:   output(svc, "global", "global").Methods,
				},
			},
		},
		{
			name: "type errors in a stale generated file are ignored",
			files: map[string]string{
				"svc/svc.go":         serviceSrc,
				"svc/service_gen.go": "package svc\n\nvar _ = missing\n",
				"global/g.go":        providerFile("global"),
			},
			cfg:  config.Config{Providers: []string{"global"}, Targets: []config.Target{target(out)}},
			want: []Output{output(svc, "global", "global")},
		},
		{
			name: "ambiguous providers",
			files: map[string]string{
				"svc/svc.go": serviceSrc,
				"a/a.go":     providerFile("a"),
				"b/b.go":     providerFile("b"),
			},
			cfg:     config.Config{Providers: []string{"a", "b"}, Targets: []config.Target{target(out)}},
			wantErr: []string{"targets[0].output[0]: Logger(): *log.Logger is provided by a.ProvideLogger, b.ProvideLogger"},
		},
		{
			name:    "missing provider",
			files:   map[string]string{"svc/svc.go": serviceSrc},
			cfg:     config.Config{Targets: []config.Target{target(out)}},
			wantErr: []string{"targets[0].output[0]: Logger(): no provider returns *log.Logger"},
		},
		{
			name:    "interface not found",
			files:   map[string]string{"svc/svc.go": "package svc\n"},
			cfg:     config.Config{Targets: []config.Target{target(out)}},
			wantErr: []string{`targets[0].interface: type "Service" not found`},
		},
		{
			name:  "every failing target is reported",
			files: map[string]string{"svc/svc.go": serviceSrc},
			cfg: config.Config{Targets: []config.Target{
				target(out),
				{Package: "svc", Interface: "Nope", Outputs: []config.Output{out}},
			}},
			wantErr: []string{
				"targets[0].output[0]: Logger(): no provider returns *log.Logger",
				`targets[1].interface: type "Nope" not found`,
			},
		},
		{
			name:    "target package missing",
			files:   map[string]string{},
			cfg:     config.Config{Targets: []config.Target{target(out)}},
			wantErr: []string{`targets[0].package: package "svc" not found`},
		},
		{
			name:    "provider package missing",
			files:   map[string]string{"svc/svc.go": serviceSrc},
			cfg:     config.Config{Providers: []string{"nope"}, Targets: []config.Target{target(out)}},
			wantErr: []string{`providers[0]: package "nope" not found`},
		},
		{
			name:  "output provider package missing",
			files: map[string]string{"svc/svc.go": serviceSrc},
			cfg: config.Config{Targets: []config.Target{target(
				config.Output{Name: "NewService", Package: "svc", Filename: "service_gen.go", Provider: "nope"},
			)}},
			wantErr: []string{`targets[0].output[0].provider: package "nope" not found`},
		},
		{
			name:    "provider package does not parse",
			files:   map[string]string{"svc/svc.go": serviceSrc, "bad/bad.go": "package bad\n\nfunc {"},
			cfg:     config.Config{Providers: []string{"bad"}, Targets: []config.Target{target(out)}},
			wantErr: []string{"providers[0]:"},
		},
		{
			name:    "no go.mod",
			files:   map[string]string{"svc/svc.go": serviceSrc},
			noMod:   true,
			cfg:     config.Config{Targets: []config.Target{target(out)}},
			wantErr: []string{"go.mod file not found"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := writeModule(t, tc.files)
			if tc.noMod {
				require.NoError(t, os.Remove(filepath.Join(root, "go.mod")))
			}

			got, err := Load(root, &tc.cfg)
			if tc.wantErr != nil {
				require.Error(t, err)
				for _, w := range tc.wantErr {
					require.ErrorContains(t, err, w)
				}
				return
			}
			require.NoError(t, err)
			for i := range got {
				for j := range got[i].Methods {
					require.Equal(t, "*log.Logger", typeString(got[i].Methods[j].Type))
					got[i].Methods[j].Type = nil
				}
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestPackageDirs(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want []string
	}{
		{name: "empty", cfg: config.Config{}, want: nil},
		{
			name: "everything is collected, sorted and deduplicated",
			cfg: config.Config{
				Providers: []string{"infra", "shared"},
				Targets: []config.Target{
					{Package: "cache", Outputs: []config.Output{{Package: "cache", Provider: "shared"}, {Package: "wire"}}},
					{Package: "db", Outputs: []config.Output{{Package: "wire"}}},
				},
			},
			want: []string{"cache", "db", "infra", "shared", "wire"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, packageDirs(&tc.cfg))
		})
	}
}

func TestLoadPackages(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		noMod   bool
		dirs    []string
		want    []string
		wantErr string
	}{
		{
			name:  "existing directories are keyed by module-relative path",
			files: map[string]string{"a/a.go": "package a\n", "b/c/c.go": "package c\n", "root.go": "package app\n"},
			dirs:  []string{".", "a", "b/c"},
			want:  []string{".", "a", "b/c"},
		},
		{
			name:  "missing directories are skipped",
			files: map[string]string{"a/a.go": "package a\n"},
			dirs:  []string{"a", "missing"},
			want:  []string{"a"},
		},
		{
			name:  "a file is not a directory",
			files: map[string]string{"a/a.go": "package a\n", "file": ""},
			dirs:  []string{"file"},
			want:  []string{},
		},
		{
			name:  "nothing to load",
			files: map[string]string{},
			want:  []string{},
		},
		{
			name:    "outside a module",
			files:   map[string]string{"a/a.go": "package a\n"},
			noMod:   true,
			dirs:    []string{"a"},
			wantErr: "go.mod file not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := writeModule(t, tc.files)
			if tc.noMod {
				require.NoError(t, os.Remove(filepath.Join(root, "go.mod")))
			}

			got, err := loadPackages(root, tc.dirs)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			keys := []string{}
			for k := range got {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			require.Equal(t, tc.want, keys)
		})
	}
}

func TestModuleDir(t *testing.T) {
	tests := []struct {
		name    string
		pkgPath string
		want    string
	}{
		{name: "module root", pkgPath: testModule, want: "."},
		{name: "nested package", pkgPath: testModule + "/internal/x", want: "internal/x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &packages.Package{PkgPath: tc.pkgPath, Module: &packages.Module{Path: testModule}}
			require.Equal(t, tc.want, moduleDir(p))
		})
	}
}

func TestCheckPackages(t *testing.T) {
	ok := &packages.Package{}
	broken := &packages.Package{Errors: []packages.Error{{Msg: "boom", Kind: packages.ParseError}}}

	tests := []struct {
		name    string
		cfg     config.Config
		pkgs    map[string]*packages.Package
		wantErr []string
	}{
		{
			name: "all present",
			cfg: config.Config{
				Providers: []string{"p"},
				Targets:   []config.Target{{Package: "t", Outputs: []config.Output{{Provider: "op"}}}},
			},
			pkgs: map[string]*packages.Package{"p": ok, "t": ok, "op": ok},
		},
		{
			name: "output without a provider override",
			cfg:  config.Config{Targets: []config.Target{{Package: "t"}}},
			pkgs: map[string]*packages.Package{"t": ok},
		},
		{
			name: "every problem is reported with its field path",
			cfg: config.Config{
				Providers: []string{"p"},
				Targets: []config.Target{
					{Package: "t", Outputs: []config.Output{{}, {Provider: "op"}}},
					{Package: "u"},
				},
			},
			pkgs: map[string]*packages.Package{"t": broken},
			wantErr: []string{
				`providers[0]: package "p" not found`,
				"targets[0].package: ",
				`targets[0].output[1].provider: package "op" not found`,
				`targets[1].package: package "u" not found`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPackages(&tc.cfg, tc.pkgs)
			if tc.wantErr != nil {
				require.Error(t, err)
				for _, w := range tc.wantErr {
					require.ErrorContains(t, err, w)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRequirePackage(t *testing.T) {
	pkgs := map[string]*packages.Package{
		"clean": {},
		"types": {Errors: []packages.Error{{Msg: "undefined: x", Kind: packages.TypeError}}},
		"parse": {Errors: []packages.Error{{Msg: "unexpected {", Kind: packages.ParseError}, {Msg: "no files", Kind: packages.ListError}}},
	}

	tests := []struct {
		name    string
		dir     string
		wantErr string
	}{
		{name: "no errors", dir: "clean"},
		{name: "type errors are ignored", dir: "types"},
		{name: "missing", dir: "nope", wantErr: `at: package "nope" not found`},
		{name: "parse and list errors", dir: "parse", wantErr: "at: -: unexpected {\n-: no files"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requirePackage(pkgs, tc.dir, "at")
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewPackage(t *testing.T) {
	pkgs := map[string]*packages.Package{
		"a":       {Name: "alpha", PkgPath: testModule + "/a"},
		"unnamed": {PkgPath: testModule + "/unnamed"},
	}

	tests := []struct {
		name string
		dir  string
		want Package
	}{
		{name: "loaded package", dir: "a", want: Package{Dir: "a", Path: testModule + "/a", Name: "alpha"}},
		{name: "loaded package without a name", dir: "unnamed", want: Package{Dir: "unnamed", Path: testModule + "/unnamed", Name: "unnamed"}},
		{name: "directory that does not exist yet", dir: "x/new", want: Package{Dir: "x/new", Path: testModule + "/x/new", Name: "new"}},
		{name: "module root", dir: ".", want: Package{Dir: ".", Path: testModule, Name: "app"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, newPackage(pkgs, tc.dir, testModule))
		})
	}
}
