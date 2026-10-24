package render

import (
	"encoding/json"
	"testing"

	"github.com/benipranata/goldie/pkg/goldie"
	"github.com/benipranata/tack/internal/load"
	"github.com/stretchr/testify/require"
)

//go:generate go test -v -update -clean -run '^TestNewConstructor$'
func TestNewConstructor(t *testing.T) {
	for _, tc := range outputCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.MarshalIndent(newConstructor(tc.out), "", "  ")
			require.NoError(t, err)

			goldie.New(t).Equal(t.Name(), got)
		})
	}
}

func TestFieldNames(t *testing.T) {
	tests := []struct {
		name    string
		methods []string
		want    []string
	}{
		{name: "none", want: []string{}},
		{name: "plain", methods: []string{"DB", "Logger"}, want: []string{"db", "logger"}},
		{name: "same lower name", methods: []string{"DB", "Db"}, want: []string{"db", "db2"}},
		{name: "keyword", methods: []string{"Type", "Func"}, want: []string{"type2", "func2"}},
		{name: "predeclared is fine", methods: []string{"Len"}, want: []string{"len"}},
		{name: "unexported method", methods: []string{"logger"}, want: []string{"logger2"}},
		{name: "unexported method next to an exported one", methods: []string{"Logger", "logger"}, want: []string{"logger2", "logger3"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			methods := []load.Method{}
			for _, m := range tc.methods {
				methods = append(methods, load.Method{Name: m})
			}

			require.Equal(t, tc.want, fieldNames(methods))
		})
	}
}
