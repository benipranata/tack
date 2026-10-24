package load

import (
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInterfaceMethods(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		iface   string
		want    map[string]string
		wantErr []string
	}{
		{
			name: "methods are listed with their result type",
			src: `
package x

import "log"

type Service interface {
	Redis() *int
	Logger() *log.Logger
}
`,
			iface: "Service",
			want:  map[string]string{"Redis": "*int", "Logger": "*log.Logger"},
		},
		{
			name: "embedded interface methods are included",
			src: `
package x

type Base interface{ Name() string }

type Service interface {
	Base
	Port() int
}
`,
			iface: "Service",
			want:  map[string]string{"Name": "string", "Port": "int"},
		},
		{
			name:    "type not found",
			src:     "package x",
			iface:   "Service",
			wantErr: []string{`type "Service" not found in package example.com/app/x`},
		},
		{
			name: "not a type",
			src: `
package x

func Service() {}
`,
			iface:   "Service",
			wantErr: []string{`type "Service" not found`},
		},
		{
			name: "not an interface",
			src: `
package x

type Service struct{}
`,
			iface:   "Service",
			wantErr: []string{`"Service" is not an interface`},
		},
		{
			name: "alias of an instantiated generic interface",
			src: `
package x

type Repo[T any] interface{ Get() T }

type IntRepo = Repo[int]
`,
			iface: "IntRepo",
			want:  map[string]string{"Get": "int"},
		},
		{
			name: "generic interface",
			src: `
package x

type Repo[T any] interface{ Get() T }
`,
			iface:   "Repo",
			wantErr: []string{`"Repo" is generic, which is not supported; name an instantiation such as type Int = Repo[int]`},
		},
		{
			name: "method with arguments",
			src: `
package x

type Service interface{ Redis(addr string) *int }
`,
			iface:   "Service",
			wantErr: []string{"method Redis must take no arguments and return one value"},
		},
		{
			name: "every bad method is reported",
			src: `
package x

type Service interface {
	Run()
	Pair() (int, error)
}
`,
			iface: "Service",
			wantErr: []string{
				"method Run must take no arguments and return one value",
				"method Pair must take no arguments and return one value",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := interfaceMethods(typecheck(t, tc.src), tc.iface)
			if tc.wantErr != nil {
				require.Error(t, err)
				for _, w := range tc.wantErr {
					require.ErrorContains(t, err, w)
				}
				return
			}
			require.NoError(t, err)

			gotTypes := map[string]string{}
			for _, m := range got {
				gotTypes[m.Name] = typeString(m.Type)
				require.Equal(t, Provider{}, m.Provider)
			}
			require.Equal(t, tc.want, gotTypes)
		})
	}
}

func TestInterfaceMethods_local(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want map[string]string
	}{
		{
			name: "exported methods can be implemented anywhere",
			src:  "package x\n\ntype Service interface{ Port() int }\n",
			want: map[string]string{"Port": ""},
		},
		{
			name: "unexported methods belong to their package",
			src:  "package x\n\ntype Service interface{ port() int }\n",
			want: map[string]string{"port": testModule + "/x"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := interfaceMethods(typecheck(t, tc.src), "Service")
			require.NoError(t, err)

			local := map[string]string{}
			for _, m := range got {
				local[m.Name] = m.local
			}
			require.Equal(t, tc.want, local)
		})
	}
}

func TestCheckImplementable(t *testing.T) {
	methods := []Method{
		{Name: "Port"},
		{Name: "port", local: "example.com/app/x"},
		{Name: "host", local: "example.com/app/y"},
	}
	tests := []struct {
		name    string
		pkg     string
		wantErr []string
	}{
		{
			name: "the declaring package of one method",
			pkg:  "example.com/app/x",
			wantErr: []string{
				"host(): unexported, so it can only be implemented in package example.com/app/y",
			},
		},
		{
			name: "another package",
			pkg:  "example.com/app/z",
			wantErr: []string{
				"port(): unexported, so it can only be implemented in package example.com/app/x",
				"host(): unexported, so it can only be implemented in package example.com/app/y",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, err := range checkImplementable(methods, tc.pkg) {
				got = append(got, err.Error())
			}

			require.Equal(t, tc.wantErr, got)
		})
	}

	t.Run("nothing to report", func(t *testing.T) {
		require.Empty(t, checkImplementable(methods[:1], "example.com/app/z"))
	})
}

func TestIsGeneric(t *testing.T) {
	const src = `
package x

type Plain interface{ Get() int }

type Repo[T any] interface{ Get() T }

type IntRepo = Repo[int]

type AnyRepo[T any] = Repo[T]
`
	pkg := typecheck(t, src)
	tests := []struct {
		name string
		typ  string
		want bool
	}{
		{name: "plain interface", typ: "Plain", want: false},
		{name: "generic interface", typ: "Repo", want: true},
		{name: "alias of an instantiation", typ: "IntRepo", want: false},
		{name: "generic alias", typ: "AnyRepo", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isGeneric(pkg.Scope().Lookup(tc.typ).Type()))
		})
	}

	t.Run("not a named type", func(t *testing.T) {
		require.False(t, isGeneric(types.Typ[types.Int]))
	})
}
