package load

import (
	"fmt"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"

	"github.com/benipranata/tack/internal/config"
)

func TestProviderLayers(t *testing.T) {
	const src = `
package %s

import "context"

func Provide%s(ctx context.Context) (*int, func(), error) { return nil, nil, nil }
func private%s(ctx context.Context) (*int, func(), error) { return nil, nil, nil }
`
	pkg := func(dir, name string) *packages.Package {
		return fakePackage(t, dir, fmt.Sprintf(src, dir, name, name))
	}

	pkgs := map[string]*packages.Package{
		"over":   pkg("over", "Over"),
		"global": pkg("global", "Global"),
		"other":  pkg("other", "Other"),
		"out":    pkg("out", "Out"),
	}

	names := func(layers [][]candidate) [][]string {
		var got [][]string
		for _, l := range layers {
			var ns []string
			for _, c := range l {
				ns = append(ns, c.Package.Dir+"."+c.Name)
			}
			got = append(got, ns)
		}
		return got
	}

	tests := []struct {
		name   string
		cfg    *config.Config
		output config.Output
		want   [][]string
	}{
		{
			name:   "output provider, global providers, then output package",
			cfg:    &config.Config{Providers: []string{"global", "other"}},
			output: config.Output{Package: "out", Provider: "over"},
			want: [][]string{
				{"over.ProvideOver"},
				{"global.ProvideGlobal", "other.ProvideOther"},
				{"out.ProvideOut", "out.privateOut"},
			},
		},
		{
			name:   "no output provider",
			cfg:    &config.Config{Providers: []string{"global"}},
			output: config.Output{Package: "out"},
			want: [][]string{
				{"global.ProvideGlobal"},
				{"out.ProvideOut", "out.privateOut"},
			},
		},
		{
			name:   "no providers and a missing output package",
			cfg:    &config.Config{},
			output: config.Output{Package: "new"},
			want:   [][]string{nil, nil},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, names(providerLayers(tc.cfg, tc.output, pkgs)))
		})
	}
}

func TestProvidersIn(t *testing.T) {
	pkgs := map[string]*packages.Package{
		"p": fakePackage(t, "p", `
package p

import "context"

type T struct{}

func ProvideT(ctx context.Context) (*T, func(), error) { return nil, nil, nil }
func provideU(ctx context.Context) (string, func(), error) { return "", nil, nil }
func NotAProvider() int { return 0 }

var V = 1

func (T) Method(ctx context.Context) (int, func(), error) { return 0, nil, nil }
`),
		"untyped": {Module: &packages.Module{Path: testModule}},
	}

	tests := []struct {
		name         string
		dir          string
		exportedOnly bool
		want         []string
	}{
		{name: "exported only", dir: "p", exportedOnly: true, want: []string{"ProvideT"}},
		{name: "unexported too", dir: "p", want: []string{"ProvideT", "provideU"}},
		{name: "missing package", dir: "nope", exportedOnly: true},
		{name: "package without types", dir: "untyped", exportedOnly: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, c := range providersIn(pkgs, tc.dir, tc.exportedOnly) {
				got = append(got, c.Name)
				require.Equal(t, Package{Dir: "p", Path: testModule + "/p", Name: "p"}, c.Package)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestProviderResult(t *testing.T) {
	const header = `
package x

import "context"

type Cleanup func()
type Ctx = context.Context
type Other interface{ Done() }
`
	tests := []struct {
		name string
		decl string
		want string
		ok   bool
	}{
		{name: "valid", decl: "func F(ctx context.Context) (*int, func(), error) { return nil, nil, nil }", want: "*int", ok: true},
		{name: "named cleanup type", decl: "func F(ctx context.Context) (int, Cleanup, error) { return 0, nil, nil }", want: "int", ok: true},
		{name: "aliased context", decl: "func F(ctx Ctx) (int, func(), error) { return 0, nil, nil }", want: "int", ok: true},
		{name: "wrong context type", decl: "func F(ctx Other) (int, func(), error) { return 0, nil, nil }"},
		{name: "not a named type", decl: "func F(ctx int) (int, func(), error) { return 0, nil, nil }"},
		{name: "no parameters", decl: "func F() (int, func(), error) { return 0, nil, nil }"},
		{name: "two parameters", decl: "func F(ctx context.Context, n int) (int, func(), error) { return 0, nil, nil }"},
		{name: "two results", decl: "func F(ctx context.Context) (int, error) { return 0, nil }"},
		{name: "cleanup is not a function", decl: "func F(ctx context.Context) (int, int, error) { return 0, 0, nil }"},
		{name: "cleanup takes arguments", decl: "func F(ctx context.Context) (int, func(int), error) { return 0, nil, nil }"},
		{name: "cleanup returns a value", decl: "func F(ctx context.Context) (int, func() error, error) { return 0, nil, nil }"},
		{name: "last result is not error", decl: "func F(ctx context.Context) (int, func(), string) { return 0, nil, \"\" }"},
		{name: "generic", decl: "func F[T any](ctx context.Context) (T, func(), error) { var z T; return z, nil, nil }"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkg := typecheck(t, header+tc.decl)
			sig := pkg.Scope().Lookup("F").Type().(*types.Signature)

			got, ok := providerResult(sig)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.want, typeString(got))
			}
		})
	}
}

func TestIsContext(t *testing.T) {
	pkg := typecheck(t, `
package x

import (
	"context"
	"time"
)

type Ctx = context.Context

var (
	Context context.Context
	Alias   Ctx
	Time    time.Time
	Count   int
	Err     error
)
`)

	tests := []struct {
		name string
		v    string
		want bool
	}{
		{name: "context.Context", v: "Context", want: true},
		{name: "alias of context.Context", v: "Alias", want: true},
		{name: "named type from another package", v: "Time"},
		{name: "unnamed type", v: "Count"},
		{name: "named type without a package", v: "Err"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isContext(pkg.Scope().Lookup(tc.v).Type()))
		})
	}
}

func TestResolveMethods(t *testing.T) {
	str, num := types.Typ[types.String], types.Typ[types.Int]
	a := Provider{Package: Package{Name: "a"}, Name: "ProvideStr"}
	b := Provider{Package: Package{Name: "b"}, Name: "ProvideNum"}
	layers := [][]candidate{{{a, str}, {b, num}}}

	tests := []struct {
		name    string
		wants   []Method
		want    []Method
		wantErr []string
	}{
		{
			name:  "every method is bound",
			wants: []Method{{Name: "Name", Type: str}, {Name: "Port", Type: num}},
			want: []Method{
				{Name: "Name", Type: str, Provider: a},
				{Name: "Port", Type: num, Provider: b},
			},
		},
		{
			name:  "no methods",
			wants: nil,
			want:  []Method{},
		},
		{
			name:    "every failure is reported",
			wants:   []Method{{Name: "Flag", Type: types.Typ[types.Bool]}, {Name: "Name", Type: str}, {Name: "Ratio", Type: types.Typ[types.Float64]}},
			wantErr: []string{"Flag(): no provider returns bool", "Ratio(): no provider returns float64"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveMethods(layers, tc.wants)
			if tc.wantErr != nil {
				require.Error(t, err)
				for _, w := range tc.wantErr {
					require.ErrorContains(t, err, w)
				}
				require.NotContains(t, err.Error(), "Name()")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestPickProvider(t *testing.T) {
	str := types.Typ[types.String]
	p := func(pkg, name string) Provider { return Provider{Package: Package{Name: pkg}, Name: name} }

	tests := []struct {
		name    string
		layers  [][]candidate
		typ     types.Type
		want    Provider
		wantErr string
	}{
		{
			name:   "single match",
			layers: [][]candidate{{{p("a", "One"), str}}},
			want:   p("a", "One"),
		},
		{
			name:   "first layer wins",
			layers: [][]candidate{{{p("a", "One"), str}}, {{p("b", "Two"), str}}},
			want:   p("a", "One"),
		},
		{
			name:   "no match falls through to the next layer",
			layers: [][]candidate{{{p("a", "One"), types.Typ[types.Int]}}, {{p("b", "Two"), str}}},
			want:   p("b", "Two"),
		},
		{
			name:    "two matches in one layer",
			layers:  [][]candidate{{{p("a", "One"), str}, {p("b", "Two"), str}}, {{p("c", "Three"), str}}},
			wantErr: "Method(): string is provided by a.One, b.Two",
		},
		{
			name:    "no layers match",
			layers:  [][]candidate{{{p("a", "One"), types.Typ[types.Int]}}, nil},
			wantErr: "Method(): no provider returns string",
		},
		{
			name:    "no layers",
			wantErr: "Method(): no provider returns string",
		},
		{
			name:    "named types are qualified by package name",
			typ:     typecheck(t, "package x\n\nimport \"log\"\n\nvar L *log.Logger\n").Scope().Lookup("L").Type(),
			wantErr: "Method(): no provider returns *log.Logger",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			typ := tc.typ
			if typ == nil {
				typ = str
			}
			got, err := pickProvider(tc.layers, Method{Name: "Method", Type: typ})
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
