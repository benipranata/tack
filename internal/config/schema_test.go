package config

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// schemaObject is the part of a JSON Schema object this test compares.
type schemaObject struct {
	Properties           map[string]json.RawMessage `json:"properties"`
	Required             []string                   `json:"required"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
}

func yamlKeys(t reflect.Type) []string {
	var keys []string
	for f := range t.Fields() {
		keys = append(keys, strings.Split(f.Tag.Get("yaml"), ",")[0])
	}
	slices.Sort(keys)
	return keys
}

// TestSchema keeps tack.schema.json in step with the config structs, which the parser enforces.
func TestSchema(t *testing.T) {
	raw, err := os.ReadFile("../../tack.schema.json")
	require.NoError(t, err)
	var schema struct {
		schemaObject
		Definitions map[string]schemaObject `json:"definitions"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))

	tests := []struct {
		name         string
		object       schemaObject
		typ          reflect.Type
		wantRequired []string
	}{
		{name: "config", object: schema.schemaObject, typ: reflect.TypeFor[Config](), wantRequired: []string{"targets"}},
		{name: "target", object: schema.Definitions["target"], typ: reflect.TypeFor[Target](), wantRequired: []string{"interface", "package"}},
		{name: "output", object: schema.Definitions["output"], typ: reflect.TypeFor[Output]()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for k := range tc.object.Properties {
				got = append(got, k)
			}
			slices.Sort(got)
			slices.Sort(tc.object.Required)

			require.Equal(t, yamlKeys(tc.typ), got, "properties must match the yaml keys")
			require.Equal(t, tc.wantRequired, tc.object.Required)
			require.NotNil(t, tc.object.AdditionalProperties)
			require.False(t, *tc.object.AdditionalProperties, "the parser rejects unknown keys")
		})
	}
}

// TestSchema_examples keeps the examples in the descriptions valid, so the schema never teaches a config that fails.
func TestSchema_examples(t *testing.T) {
	raw, err := os.ReadFile("../../tack.schema.json")
	require.NoError(t, err)
	var schema struct {
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))

	_, example, found := strings.Cut(schema.Description, "Example: ")
	require.True(t, found, "the root description needs an example")

	cfg, err := Parse([]byte(example))

	require.NoError(t, err)
	require.Equal(t, "NewService", cfg.Targets[0].Outputs[0].Name)
}
