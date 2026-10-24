package render

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewNamer(t *testing.T) {
	n := newNamer(true, "a", "b")

	require.True(t, n.universe)
	require.Equal(t, map[string]bool{"a": true, "b": true}, n.used)
}

func TestNamer_take(t *testing.T) {
	tests := []struct {
		name     string
		universe bool
		reserved []string
		takes    []string
		want     []string
	}{
		{name: "free", takes: []string{"db"}, want: []string{"db"}},
		{name: "repeated", takes: []string{"db", "db", "db"}, want: []string{"db", "db2", "db3"}},
		{name: "reserved", reserved: []string{"db"}, takes: []string{"db"}, want: []string{"db2"}},
		{name: "suffix taken", reserved: []string{"db2"}, takes: []string{"db", "db"}, want: []string{"db", "db3"}},
		{name: "keyword", takes: []string{"type"}, want: []string{"type2"}},
		{name: "predeclared allowed", takes: []string{"len"}, want: []string{"len"}},
		{name: "predeclared avoided", universe: true, takes: []string{"len", "nil"}, want: []string{"len2", "nil2"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := newNamer(tc.universe, tc.reserved...)

			var got []string
			for _, s := range tc.takes {
				got = append(got, n.take(s))
			}

			require.Equal(t, tc.want, got)
		})
	}
}

func TestLowerName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "word", in: "Logger", want: "logger"},
		{name: "acronym", in: "DB", want: "db"},
		{name: "leading acronym", in: "HTTPClient", want: "httpClient"},
		{name: "acronym then word", in: "DBPool", want: "dbPool"},
		{name: "digit in acronym", in: "DB2Pool", want: "db2Pool"},
		{name: "single letter", in: "X", want: "x"},
		{name: "already lower", in: "logger", want: "logger"},
		{name: "empty", in: "", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, lowerName(tc.in))
		})
	}
}
