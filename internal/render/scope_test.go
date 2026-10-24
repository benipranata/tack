package render

import (
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/benipranata/tack/internal/load"
)

func outputIn(dir, iface string) load.Output {
	return load.Output{
		Name:      "New" + iface,
		Package:   pkgAt(dir),
		Target:    pkgAt("internal/app"),
		Interface: iface,
	}
}

func TestNewScope(t *testing.T) {
	tests := []struct {
		name string
		out  load.Output
		want string
	}{
		{name: "exported interface", out: outputIn("internal/app", "Service"), want: "service"},
		{name: "acronym", out: outputIn("internal/app", "HTTPServer"), want: "httpServer"},
		{name: "unexported interface", out: outputIn("internal/app", "redis"), want: "redisImpl"},
		{name: "reserved name", out: outputIn("internal/app", "T"), want: "t2"},
		{name: "same name as the constructor", out: func() load.Output {
			o := outputIn("internal/app", "Service")
			o.Name = "service"
			return o
		}(), want: "service2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newScope(tc.out)

			require.Equal(t, tc.want, s.structName)
			require.Equal(t, tc.out, s.out)
		})
	}
}

func TestNewScope_interfaceName(t *testing.T) {
	tests := []struct {
		name string
		out  load.Output
		want string
	}{
		{name: "same package reserves it", out: outputIn("internal/app", "Service"), want: "Service2"},
		{name: "other package does not", out: outputIn("internal/app/prod", "Service"), want: "Service"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, newScope(tc.out).names.take("Service"))
		})
	}
}

func TestScope_pkg(t *testing.T) {
	tests := []struct {
		name string
		path string
		pkg  string
		want string
	}{
		{name: "output package", path: testModule + "/internal/app", pkg: "app", want: ""},
		{name: "other package", path: "log/slog", pkg: "slog", want: "slog"},
		{name: "name taken by the struct", path: testModule + "/internal/service", pkg: "service", want: "service2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newScope(outputIn("internal/app", "Service"))

			require.Equal(t, tc.want, s.pkg(tc.path, tc.pkg))
		})
	}
}

func TestScope_typeString(t *testing.T) {
	local := named(testModule+"/internal/app", "app", "Config")
	tests := []struct {
		name string
		typ  types.Type
		want string
	}{
		{name: "basic", typ: types.Typ[types.Int], want: "int"},
		{name: "imported pointer", typ: sqlDB(), want: "*sql.DB"},
		{name: "output package is unqualified", typ: types.NewPointer(local), want: "*Config"},
		{name: "slice of imported", typ: types.NewSlice(slogLogger()), want: "[]*slog.Logger"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newScope(outputIn("internal/app", "Service"))

			require.Equal(t, tc.want, s.typeString(tc.typ))
		})
	}
}

func TestScope_qualify(t *testing.T) {
	tests := []struct {
		name string
		pkg  load.Package
		want string
	}{
		{name: "output package", pkg: pkgAt("internal/app"), want: "ProvideDB"},
		{name: "other package", pkg: pkgAt("internal/providers"), want: "providers.ProvideDB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newScope(outputIn("internal/app", "Service"))

			require.Equal(t, tc.want, s.qualify(tc.pkg, "ProvideDB"))
		})
	}
}
