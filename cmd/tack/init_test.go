package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInit_ScaffoldsSchemaReference asserts the starter config tack init
// writes leads with a yaml-language-server $schema comment, so a freshly
// scaffolded tack.yaml is IDE-autocomplete-ready with no further setup.
func TestInit_ScaffoldsSchemaReference(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := runInit(nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "tack.yaml"))
	if err != nil {
		t.Fatalf("read scaffolded tack.yaml: %v", err)
	}

	firstLine, _, _ := strings.Cut(string(data), "\n")
	const want = "# yaml-language-server: $schema=https://raw.githubusercontent.com/benipranata/tack/main/tack.schema.json"
	if firstLine != want {
		t.Errorf("first line = %q, want %q", firstLine, want)
	}
}
