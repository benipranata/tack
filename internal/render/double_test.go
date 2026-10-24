package render

import (
	"encoding/json"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:generate go test -v -update -clean -run '^TestNewDouble$'
func TestNewDouble(t *testing.T) {
	for _, tc := range outputCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.MarshalIndent(newDouble(tc.out), "", "  ")
			require.NoError(t, err)

			goldenEqual(t, got)
		})
	}
}

func TestNillable(t *testing.T) {
	tests := []struct {
		name string
		typ  types.Type
		want bool
	}{
		{name: "pointer", typ: sqlDB(), want: true},
		{name: "interface", typ: types.Universe.Lookup("error").Type(), want: true},
		{name: "func", typ: types.NewSignature(nil, nil, nil, false), want: true},
		{name: "map", typ: types.NewMap(types.Typ[types.String], types.Typ[types.Int]), want: true},
		{name: "slice", typ: types.NewSlice(types.Typ[types.Int]), want: true},
		{name: "chan", typ: types.NewChan(types.SendRecv, types.Typ[types.Int]), want: true},
		{name: "int", typ: types.Typ[types.Int], want: false},
		{name: "struct", typ: named("example.com/x", "x", "T"), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, nillable(tc.typ))
		})
	}
}
