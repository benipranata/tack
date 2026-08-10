package resolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benipranata/tack/internal/config"
)

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func providerFor(t *testing.T, iface *Interface, method string) string {
	t.Helper()
	for _, m := range iface.Methods {
		if m.Name == method {
			return m.Provider.Pkg.PkgPath
		}
	}
	t.Fatalf("no method %s on resolved interface", method)
	return ""
}

// TestResolveAll_LocalShadowsGlobalPerType mirrors case-01's *c.C vs. *d.D
// split: the target package's own directory provides *c.C, the global scope
// provides both *c.C and *d.D, and only *d.D should come from global.
func TestResolveAll_LocalShadowsGlobalPerType(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module restest\n\ngo 1.26.5\n",
		"c/c.go": "package c\n\ntype C struct{}\n",
		"d/d.go": "package d\n\ntype D struct{}\n",
		"target/target.go": `package target

import (
	"context"

	"restest/c"
	"restest/d"
)

type IFace interface {
	Foo() *c.C
	Bar() *d.D
}

func ProvideC(ctx context.Context) (*c.C, func(), error) {
	return &c.C{}, func() {}, nil
}
`,
		"prov/prov.go": `package prov

import (
	"context"

	"restest/c"
	"restest/d"
)

func ProvideC(ctx context.Context) (*c.C, func(), error) {
	return &c.C{}, func() {}, nil
}

func ProvideD(ctx context.Context) (*d.D, func(), error) {
	return &d.D{}, func() {}, nil
}
`,
	})

	cfg := &config.Config{
		ModuleRoot: dir,
		Providers:  []string{"prov"},
		Packages: map[string]map[string]config.InterfaceConfig{
			"target": {"IFace": {Name: "App", LocalScan: true}},
		},
	}

	results, err := ResolveAll(cfg)
	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d resolved interfaces, want 1", len(results))
	}
	iface := results[0]

	if got := providerFor(t, iface, "Foo"); !strings.HasSuffix(got, "/target") {
		t.Errorf("Foo provider pkg = %s, want local (target) package", got)
	}
	if got := providerFor(t, iface, "Bar"); !strings.HasSuffix(got, "/prov") {
		t.Errorf("Bar provider pkg = %s, want global (prov) package", got)
	}
}

// TestResolveAll_LocalScanFalseDoesNotAffectSibling configures two
// interfaces from the same directory, one with localScan: false, and checks
// only that one ignores the directory's local provider.
func TestResolveAll_LocalScanFalseDoesNotAffectSibling(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module restest\n\ngo 1.26.5\n",
		"x/x.go": "package x\n\ntype X struct{}\n",
		"target/target.go": `package target

import (
	"context"

	"restest/x"
)

type A interface {
	Val() *x.X
}

type B interface {
	Val() *x.X
}

func ProvideX(ctx context.Context) (*x.X, func(), error) {
	return &x.X{}, func() {}, nil
}
`,
		"prov/prov.go": `package prov

import (
	"context"

	"restest/x"
)

func ProvideX(ctx context.Context) (*x.X, func(), error) {
	return &x.X{}, func() {}, nil
}
`,
	})

	cfg := &config.Config{
		ModuleRoot: dir,
		Providers:  []string{"prov"},
		Packages: map[string]map[string]config.InterfaceConfig{
			"target": {
				"A": {Name: "AppA", LocalScan: false},
				"B": {Name: "AppB", LocalScan: true},
			},
		},
	}

	results, err := ResolveAll(cfg)
	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	byName := map[string]*Interface{}
	for _, r := range results {
		byName[r.IfaceName] = r
	}

	if got := providerFor(t, byName["A"], "Val"); !strings.HasSuffix(got, "/prov") {
		t.Errorf("A.Val provider pkg = %s, want global (prov) package (localScan: false)", got)
	}
	if got := providerFor(t, byName["B"], "Val"); !strings.HasSuffix(got, "/target") {
		t.Errorf("B.Val provider pkg = %s, want local (target) package", got)
	}
}

// TestResolveAll_SelfListedDirectoryNoSpuriousAmbiguity configures a
// directory as both an interface's own package and a member of the global
// providers list; resolution must succeed with local-beats-global
// precedence, not error out as ambiguous.
func TestResolveAll_SelfListedDirectoryNoSpuriousAmbiguity(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module restest\n\ngo 1.26.5\n",
		"y/y.go": "package y\n\ntype Y struct{}\n",
		"target/target.go": `package target

import (
	"context"

	"restest/y"
)

type IFace interface {
	Val() *y.Y
}

func ProvideY(ctx context.Context) (*y.Y, func(), error) {
	return &y.Y{}, func() {}, nil
}
`,
	})

	cfg := &config.Config{
		ModuleRoot: dir,
		Providers:  []string{"target"},
		Packages: map[string]map[string]config.InterfaceConfig{
			"target": {"IFace": {Name: "App", LocalScan: true}},
		},
	}

	results, err := ResolveAll(cfg)
	if err != nil {
		t.Fatalf("ResolveAll: %v (self-listing must not cause spurious ambiguity)", err)
	}
	if got := providerFor(t, results[0], "Val"); !strings.HasSuffix(got, "/target") {
		t.Errorf("Val provider pkg = %s, want target package", got)
	}
}
