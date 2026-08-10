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
		Packages: map[string]map[string]config.InterfaceConfig{
			"target": {"IFace": {Name: "App", LocalScan: true}},
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

	outPath := filepath.Join(dir, "target", OutputFilename(ifaces[0].Config, ifaces[0].IfaceName))
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		t.Fatalf("write generated file: %v", err)
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if buildOut, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code does not compile: %v\n%s", err, buildOut)
	}
}
