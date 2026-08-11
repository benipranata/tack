package gen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benipranata/tack/internal/config"
	"github.com/benipranata/tack/internal/resolve"
)

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestGenerate_CollidingMethodNameIsSuffixed covers task 5.6: a method
// named Cleanup derives a local variable name that collides with the
// generator's own reserved "cleanup" aggregate-cleanup closure, so the
// allocator must suffix it rather than reuse it, and the result must still
// compile.
func TestGenerate_CollidingMethodNameIsSuffixed(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module gentest\n\ngo 1.26.5\n",
		"x/x.go": "package x\n\ntype X struct{}\n",
		"target/target.go": `package target

import (
	"context"

	"gentest/x"
)

type IFace interface {
	Cleanup() *x.X
}

func ProvideX(ctx context.Context) (*x.X, func(), error) {
	return &x.X{}, func() {}, nil
}
`,
	})

	cfg := &config.Config{
		ModuleRoot: dir,
		Targets: []config.TargetConfig{
			{Package: "target", Interface: "IFace", Output: []config.OutputConfig{
				{Name: "App", LocalScan: true},
			}},
		},
	}

	ifaces, err := resolve.ResolveAll(cfg)
	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("got %d interfaces, want 1", len(ifaces))
	}

	out, err := Generate(ifaces[0])
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	src := string(out)
	if !strings.Contains(src, "cleanup2") {
		t.Errorf("generated source does not suffix the colliding local var; got:\n%s", src)
	}
	if strings.Contains(src, "cleanup, cleanupCleanup, err") {
		t.Errorf("generated source reused the reserved \"cleanup\" name instead of suffixing it; got:\n%s", src)
	}

	outPath := filepath.Join(dir, ifaces[0].EffectiveDir, OutputFilename(ifaces[0].Output, ifaces[0].IfaceName))
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		t.Fatalf("write generated file: %v", err)
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if buildOut, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code does not compile: %v\n%s", err, buildOut)
	}
}

// TestGenerate_CrossPackageOutputIsQualified covers a differentiated output
// package: the output variant writes into a directory other than the one
// the interface is declared in, so the generated file must import the
// interface's own package and refer to it qualified everywhere it would
// otherwise be bare, and its package clause must match the output
// directory's own package name (not the interface's).
func TestGenerate_CrossPackageOutputIsQualified(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod":         "module crosspkg\n\ngo 1.26.5\n",
		"state/state.go": "package state\n\ntype Store struct{}\n\ntype State interface {\n\tVal() *Store\n}\n",
		"state/prod/prod.go": `package prod

import (
	"context"

	"crosspkg/state"
)

func ProvideStore(ctx context.Context) (*state.Store, func(), error) {
	return &state.Store{}, func() {}, nil
}
`,
	})

	cfg := &config.Config{
		ModuleRoot: dir,
		Targets: []config.TargetConfig{
			{Package: "state", Interface: "State", Output: []config.OutputConfig{
				{Name: "Prod", Package: "state/prod", LocalScan: true},
			}},
		},
	}

	ifaces, err := resolve.ResolveAll(cfg)
	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("got %d interfaces, want 1", len(ifaces))
	}

	out, err := Generate(ifaces[0])
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	src := string(out)
	if !strings.Contains(src, "package prod") {
		t.Errorf("generated file's package clause is not \"prod\"; got:\n%s", src)
	}
	if !strings.Contains(src, `"crosspkg/state"`) {
		t.Errorf("generated file does not import the interface's own package; got:\n%s", src)
	}
	if !strings.Contains(src, "func NewProdState(ctx context.Context) (state.State, func(), error)") {
		t.Errorf("constructor return type is not qualified state.State; got:\n%s", src)
	}
	if !strings.Contains(src, "var _ state.State = (*prodState)(nil)") {
		t.Errorf("interface-satisfaction assertion is not qualified state.State; got:\n%s", src)
	}

	outPath := filepath.Join(dir, ifaces[0].EffectiveDir, OutputFilename(ifaces[0].Output, ifaces[0].IfaceName))
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		t.Fatalf("write generated file: %v", err)
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if buildOut, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code does not compile: %v\n%s", err, buildOut)
	}
}
