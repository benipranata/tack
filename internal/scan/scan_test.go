package scan

import (
	"os"
	"path/filepath"
	"testing"
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

func TestAdd_QualifyingAndMismatchedProvide(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module scantest\n\ngo 1.26.5\n",
		"p/p.go": `package p

import "context"

type T struct{}

// Qualifies: exact signature.
func ProvideT(ctx context.Context) (T, func(), error) {
	return T{}, func() {}, nil
}

// Provide-prefixed but wrong signature (missing cleanup return): a near-miss,
// not indexed, and must not error on its own.
func ProvideBad(ctx context.Context) (T, error) {
	return T{}, nil
}

// Not Provide-prefixed and wrong signature: irrelevant, ignored entirely.
func Helper() int { return 0 }
`,
	})

	pkg, err := LoadDir(dir, "p")
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	idx := NewIndex()
	if err := idx.Add(pkg); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(idx.providers) != 1 {
		t.Fatalf("providers = %d, want 1 (got %+v)", len(idx.providers), idx.providers)
	}
	if idx.providers[0].Func.Name() != "ProvideT" {
		t.Errorf("provider func = %s, want ProvideT", idx.providers[0].Func.Name())
	}

	nm := idx.NearMisses(idx.providers[0].Type)
	if len(nm) != 1 || nm[0].Func.Name() != "ProvideBad" {
		t.Errorf("near misses = %+v, want exactly ProvideBad", nm)
	}
}

func TestAdd_AmbiguityError(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module scantest\n\ngo 1.26.5\n",
		"p/p.go": `package p

import "context"

type T struct{}

func ProvideOne(ctx context.Context) (T, func(), error) { return T{}, func() {}, nil }
func ProvideTwo(ctx context.Context) (T, func(), error) { return T{}, func() {}, nil }
`,
	})

	pkg, err := LoadDir(dir, "p")
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	idx := NewIndex()
	if err := idx.Add(pkg); err == nil {
		t.Fatal("Add: expected ambiguity error, got nil")
	}
}
