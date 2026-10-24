package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeModule(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.21\n",
		"tack.yaml": `prefix: New
providers:
  - internal/providers
targets:
  - package: internal/app
    interface: Service
`,
		"internal/app/service.go": `package app

import "log/slog"

type Service interface {
	Logger() *slog.Logger
}
`,
		"internal/providers/providers.go": `package providers

import (
	"context"
	"log/slog"
)

func ProvideLogger(ctx context.Context) (*slog.Logger, func(), error) { return nil, nil, nil }
`,
	}
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	return root
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       func(root string) []string
		wantCode   int
		wantStderr string
		wantFile   bool
	}{
		{
			name:     "generates with -config",
			args:     func(root string) []string { return []string{"-config", filepath.Join(root, "tack.yaml")} },
			wantCode: 0,
			wantFile: true,
		},
		{
			name:     "generates from the working directory",
			args:     func(string) []string { return nil },
			wantCode: 0,
			wantFile: true,
		},
		{
			name:       "reports a failure",
			args:       func(root string) []string { return []string{"-config", filepath.Join(root, "nope.yaml")} },
			wantCode:   1,
			wantStderr: "nope.yaml",
		},
		{
			name:       "unknown flag",
			args:       func(string) []string { return []string{"-nope"} },
			wantCode:   2,
			wantStderr: "flag provided but not defined: -nope",
		},
		{
			name:       "unexpected argument",
			args:       func(string) []string { return []string{"extra"} },
			wantCode:   2,
			wantStderr: "unexpected arguments: [extra]",
		},
		{
			name:       "help",
			args:       func(string) []string { return []string{"-h"} },
			wantCode:   0,
			wantStderr: "-config string",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := writeModule(t)
			t.Chdir(root)
			var stderr bytes.Buffer

			code := run(tc.args(root), &stderr)

			require.Equal(t, tc.wantCode, code, stderr.String())
			require.Contains(t, stderr.String(), tc.wantStderr)
			if tc.wantFile {
				require.FileExists(t, filepath.Join(root, "internal/app/service_gen.go"))
			}
		})
	}
}
