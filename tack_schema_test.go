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

func TestSchema_RejectsRemovedTopLevelPackagesKey(t *testing.T) {
	sch := compileSchema(t)

	doc := decodeYAML(t, `
packages:
  src/iface:
    Iface:
      name: App
`)

	if err := sch.Validate(doc); err == nil {
		t.Error("expected schema validation to fail for the removed top-level packages key, got nil")
	}
}

func TestSchema_RejectsTargetLevelProvidersKey(t *testing.T) {
	sch := compileSchema(t)

	doc := decodeYAML(t, `
targets:
  - package: src/iface
    interface: Iface
    providers:
      - src/other
    output:
      - name: App
`)

	if err := sch.Validate(doc); err == nil {
		t.Error("expected schema validation to fail for target-level providers key, got nil")
	}
}

func TestSchema_RejectsOutputLevelProvidersKey(t *testing.T) {
	sch := compileSchema(t)

	doc := decodeYAML(t, `
targets:
  - package: src/iface
    interface: Iface
    output:
      - name: App
        providers:
          - src/other
`)

	if err := sch.Validate(doc); err == nil {
		t.Error("expected schema validation to fail for output-level providers key, got nil")
	}
}

func TestSchema_RejectsUnknownTopLevelKey(t *testing.T) {
	sch := compileSchema(t)

	doc := decodeYAML(t, `
bogus: true
targets:
  - package: src/iface
    interface: Iface
    output:
      - name: App
`)

	if err := sch.Validate(doc); err == nil {
		t.Error("expected schema validation to fail for unknown top-level key, got nil")
	}
}

func TestSchema_AcceptsMultipleOutputVariants(t *testing.T) {
	sch := compileSchema(t)

	doc := decodeYAML(t, `
targets:
  - package: src/state
    interface: State
    output:
      - name: Prod
        package: src/state/prod
      - name: Staging
        package: src/state/staging
        localScan: false
`)

	if err := sch.Validate(doc); err != nil {
		t.Errorf("expected multiple output variants to validate, got: %v", err)
	}
}
