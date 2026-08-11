package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "go.mod", "module example.com/foo\n\ngo 1.26.5\n")
	writeTemp(t, dir, "tack.yaml", `
providers:
  - src/provider-01
  - src/provider-02
targets:
  - package: src/iface
    interface: Iface
    output:
      - name: App
        file: app_iface_gen.go
`)

	cfg, err := Load("", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ModuleRoot != dir {
		t.Errorf("ModuleRoot = %q, want %q", cfg.ModuleRoot, dir)
	}
	if len(cfg.Providers) != 2 {
		t.Fatalf("Providers = %v, want 2 entries", cfg.Providers)
	}
	if len(cfg.Targets) != 1 {
		t.Fatalf("Targets = %v, want 1 entry", cfg.Targets)
	}
	tgt := cfg.Targets[0]
	if tgt.Package != "src/iface" || tgt.Interface != "Iface" {
		t.Fatalf("unexpected target: %+v", tgt)
	}
	if len(tgt.Output) != 1 {
		t.Fatalf("Output = %v, want 1 entry", tgt.Output)
	}
	oc := tgt.Output[0]
	if oc.Name != "App" || oc.File != "app_iface_gen.go" || !oc.LocalScan {
		t.Errorf("unexpected OutputConfig: %+v", oc)
	}
	if got := oc.EffectiveDir(tgt); got != "src/iface" {
		t.Errorf("EffectiveDir = %q, want %q", got, "src/iface")
	}
}

func TestLoad_MultipleOutputVariants(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "go.mod", "module example.com/foo\n\ngo 1.26.5\n")
	writeTemp(t, dir, "tack.yaml", `
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

	cfg, err := Load("", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatalf("Targets = %v, want 1 entry", cfg.Targets)
	}
	tgt := cfg.Targets[0]
	if len(tgt.Output) != 2 {
		t.Fatalf("Output = %v, want 2 entries", tgt.Output)
	}
	prod, staging := tgt.Output[0], tgt.Output[1]
	if prod.Name != "Prod" || prod.EffectiveDir(tgt) != "src/state/prod" || !prod.LocalScan {
		t.Errorf("unexpected Prod output: %+v", prod)
	}
	if staging.Name != "Staging" || staging.EffectiveDir(tgt) != "src/state/staging" || staging.LocalScan {
		t.Errorf("unexpected Staging output: %+v", staging)
	}
}

func TestLoad_RemovedTopLevelPackagesKeyRejected(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "go.mod", "module example.com/foo\n\ngo 1.26.5\n")
	writeTemp(t, dir, "tack.yaml", `
packages:
  src/iface:
    Iface:
      name: App
`)

	_, err := Load("", dir)
	if err == nil {
		t.Fatal("Load: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "packages") {
		t.Errorf("error %q does not name the offending key", err.Error())
	}
}

func TestLoad_TargetLevelProvidersKeyRejected(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "go.mod", "module example.com/foo\n\ngo 1.26.5\n")
	writeTemp(t, dir, "tack.yaml", `
targets:
  - package: src/iface
    interface: Iface
    providers:
      - src/other
    output:
      - name: App
`)

	_, err := Load("", dir)
	if err == nil {
		t.Fatal("Load: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "targets[0].providers") {
		t.Errorf("error %q does not name the offending key and location", err.Error())
	}
}

func TestLoad_OutputLevelProvidersKeyRejected(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "go.mod", "module example.com/foo\n\ngo 1.26.5\n")
	writeTemp(t, dir, "tack.yaml", `
targets:
  - package: src/iface
    interface: Iface
    output:
      - name: App
        providers:
          - src/other
`)

	_, err := Load("", dir)
	if err == nil {
		t.Fatal("Load: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "targets[0].output[0].providers") {
		t.Errorf("error %q does not name the offending key and location", err.Error())
	}
}

func TestLoad_UnknownTopLevelKeyRejected(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "go.mod", "module example.com/foo\n\ngo 1.26.5\n")
	writeTemp(t, dir, "tack.yaml", `
bogus: true
targets:
  - package: src/iface
    interface: Iface
    output:
      - name: App
`)

	_, err := Load("", dir)
	if err == nil {
		t.Fatal("Load: expected error, got nil")
	}
	if !strings.Contains(err.Error(), `"bogus"`) {
		t.Errorf("error %q does not name the offending key", err.Error())
	}
}

func TestLoad_OutputCollisionRejected(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "go.mod", "module example.com/foo\n\ngo 1.26.5\n")
	writeTemp(t, dir, "tack.yaml", `
targets:
  - package: src/iface
    interface: Iface
    output:
      - name: App
      - name: App
`)

	_, err := Load("", dir)
	if err == nil {
		t.Fatal("Load: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "src/iface/app_iface_gen.go") {
		t.Errorf("error %q does not name the colliding output path", err.Error())
	}
}

func TestLoad_OutputCollisionAcrossTargetsRejected(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "go.mod", "module example.com/foo\n\ngo 1.26.5\n")
	writeTemp(t, dir, "tack.yaml", `
targets:
  - package: src/iface
    interface: Iface
    output:
      - name: App
        package: src/shared
        file: gen.go
  - package: src/other
    interface: Other
    output:
      - name: App
        package: src/shared
        file: gen.go
`)

	_, err := Load("", dir)
	if err == nil {
		t.Fatal("Load: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "src/shared/gen.go") {
		t.Errorf("error %q does not name the colliding output path", err.Error())
	}
}

func TestDiscover_WalksUp(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "tack.yaml", "targets: []\n")
	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	found, err := Discover(nested)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	want := filepath.Join(dir, "tack.yaml")
	if found != want {
		t.Errorf("Discover = %q, want %q", found, want)
	}
}

func TestDiscover_NotFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := Discover(dir); err == nil {
		t.Fatal("Discover: expected error, got nil")
	}
}

func TestLoad_ExplicitConfigSkipsDiscovery(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "go.mod", "module example.com/foo\n\ngo 1.26.5\n")
	other := writeTemp(t, dir, "other.yaml", `
targets:
  - package: src/iface
    interface: Iface
    output:
      - name: App
`)

	cfg, err := Load(other, "/nonexistent/does/not/matter")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Path != other {
		t.Errorf("Path = %q, want %q", cfg.Path, other)
	}
}
