package load

import (
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
