// Package tack_test guards tack.schema.json against drift from
// internal/config's actual validation rules.
package tack_test

import (
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	sch, err := jsonschema.Compile("tack.schema.json")
	if err != nil {
		t.Fatalf("compile tack.schema.json: %v", err)
	}
	return sch
}

func decodeYAML(t *testing.T, content string) any {
	t.Helper()
	var doc any
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		t.Fatalf("unmarshal yaml: %v", err)
	}
	return doc
}

func TestSchema_GoldenConfigValidates(t *testing.T) {
	sch := compileSchema(t)

	data, err := os.ReadFile("openspec/initial-idea/tack.yaml")
	if err != nil {
		t.Fatalf("read golden config: %v", err)
	}
	doc := decodeYAML(t, string(data))

	if err := sch.Validate(doc); err != nil {
		t.Errorf("golden config failed schema validation: %v", err)
	}
}

func TestSchema_RejectsPackageLevelProvidersKey(t *testing.T) {
	sch := compileSchema(t)

	doc := decodeYAML(t, `
packages:
  src/iface:
    providers:
      - src/other
    Iface:
      name: App
`)

	if err := sch.Validate(doc); err == nil {
		t.Error("expected schema validation to fail for package-level providers key, got nil")
	}
}

func TestSchema_RejectsInterfaceLevelProvidersKey(t *testing.T) {
	sch := compileSchema(t)

	doc := decodeYAML(t, `
packages:
  src/iface:
    Iface:
      name: App
      providers:
        - src/other
`)

	if err := sch.Validate(doc); err == nil {
		t.Error("expected schema validation to fail for interface-level providers key, got nil")
	}
}

func TestSchema_RejectsUnknownTopLevelKey(t *testing.T) {
	sch := compileSchema(t)

	doc := decodeYAML(t, `
bogus: true
packages:
  src/iface:
    Iface:
      name: App
`)

	if err := sch.Validate(doc); err == nil {
		t.Error("expected schema validation to fail for unknown top-level key, got nil")
	}
}
