package render

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewImports(t *testing.T) {
	n := newNamer(true)
	i := newImports(n)

	require.Same(t, n, i.names)
	require.Empty(t, i.byPath)
}

func TestImports_use(t *testing.T) {
	tests := []struct {
		name string
		uses [][2]string
		want []string
	}{
		{name: "single", uses: [][2]string{{"log/slog", "slog"}}, want: []string{"slog"}},
		{
			name: "same path twice",
			uses: [][2]string{{"log/slog", "slog"}, {"log/slog", "slog"}},
			want: []string{"slog", "slog"},
		},
		{
			name: "same name different path",
			uses: [][2]string{{"a/store", "store"}, {"b/store", "store"}, {"c/store", "store"}},
			want: []string{"store", "store2", "store3"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := newImports(newNamer(true))

			var got []string
			for _, u := range tc.uses {
				got = append(got, i.use(u[0], u[1]))
			}

			require.Equal(t, tc.want, got)
		})
	}
}

func TestImports_specs(t *testing.T) {
	tests := []struct {
		name string
		uses [][2]string
		want []importSpec
	}{
		{name: "none", want: []importSpec{}},
		{
			name: "std only",
			uses: [][2]string{{"log/slog", "slog"}, {"context", "context"}},
			want: []importSpec{{Path: "context"}, {Path: "log/slog"}},
		},
		{
			name: "std before others with a gap",
			uses: [][2]string{{"example.com/b", "b"}, {"fmt", "fmt"}, {"example.com/a", "a"}},
			want: []importSpec{{Path: "fmt"}, {Path: "example.com/a", Gap: true}, {Path: "example.com/b"}},
		},
		{
			name: "others only have no gap",
			uses: [][2]string{{"example.com/a", "a"}},
			want: []importSpec{{Path: "example.com/a"}},
		},
		{
			name: "alias only when renamed",
			uses: [][2]string{{"example.com/a/store", "store"}, {"example.com/b/store", "store"}, {"gopkg.in/yaml.v3", "yaml"}},
			want: []importSpec{
				{Path: "example.com/a/store", Gap: false},
				{Alias: "store2", Path: "example.com/b/store"},
				{Path: "gopkg.in/yaml.v3"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := newImports(newNamer(true))
			for _, u := range tc.uses {
				i.use(u[0], u[1])
			}

			require.Equal(t, tc.want, i.specs())
		})
	}
}

func TestIsStd(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "single element", in: "fmt", want: true},
		{name: "nested", in: "log/slog", want: true},
		{name: "domain", in: "example.com/app", want: false},
		{name: "domain with dot only in first element", in: "golang.org/x/tools", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isStd(tc.in))
		})
	}
}
