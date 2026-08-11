package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// copyDir recursively copies src into dst, creating dst if needed.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatalf("copyDir(%s, %s): %v", src, dst, err)
	}
}

// goldenFiles are every generated file checked in under openspec/initial-idea,
// relative to the fixture root: the single-variant Iface/App case, plus the
// differentiated State/Prod/Staging case (cross-package qualified emission,
// Prod's local-provider override, and Staging's scaffold-on-demand +
// global-fallback path, since src/state/staging has no hand-written source
// of its own in the checked-in fixture).
var goldenFiles = []string{
	filepath.Join("src", "iface", "app_iface_gen.go"),
	filepath.Join("src", "state", "prod", "prod_state_gen.go"),
	filepath.Join("src", "state", "staging", "staging_state_gen.go"),
}

// TestGoldenFixture copies the openspec/initial-idea reference fixture to a
// temp directory, runs the generator in-process (a direct call, not
// exec.Command) against its tack.yaml, asserts every regenerated file in
// goldenFiles matches the checked-in file byte-for-byte, and then
// go build ./...'s the temp copy to confirm the regenerated output compiles.
func TestGoldenFixture(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fixtureSrc := filepath.Join(wd, "..", "..", "openspec", "initial-idea")

	tempDir := t.TempDir()
	copyDir(t, fixtureSrc, tempDir)

	configPath := filepath.Join(tempDir, "tack.yaml")
	if err := runGenerate(configPath); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}

	for _, rel := range goldenFiles {
		got, err := os.ReadFile(filepath.Join(tempDir, rel))
		if err != nil {
			t.Fatalf("read regenerated %s: %v", rel, err)
		}
		want, err := os.ReadFile(filepath.Join(fixtureSrc, rel))
		if err != nil {
			t.Fatalf("read checked-in golden %s: %v", rel, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("regenerated %s does not match the checked-in golden file byte-for-byte.\n--- got ---\n%s\n--- want ---\n%s", rel, got, want)
		}
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tempDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... on the regenerated temp copy failed: %v\n%s", err, out)
	}
}
