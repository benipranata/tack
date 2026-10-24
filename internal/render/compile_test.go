package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"

	"github.com/benipranata/tack/internal/config"
	"github.com/benipranata/tack/internal/load"
)

// TestRender_compiles proves the generated files build against real providers and interfaces.
func TestRender_compiles(t *testing.T) {
	tests := []struct {
		name   string
		config string
		files  map[string]string
	}{
		{
			name: "quickstart with a test double",
			config: `
prefix: New
test: true
providers:
  - internal/providers
targets:
  - package: internal/app
    interface: Service
`,
			files: map[string]string{
				"internal/app/service.go": `package app

import (
	"database/sql"
	"log/slog"
)

type Service interface {
	DB() *sql.DB
	Logger() *slog.Logger
	Len() int
}
`,
				"internal/providers/providers.go": `package providers

import (
	"context"
	"database/sql"
	"log/slog"
)

func ProvideDB(ctx context.Context) (*sql.DB, func(), error) { return nil, nil, nil }

func ProvideLogger(ctx context.Context) (*slog.Logger, func(), error) { return nil, nil, nil }

func ProvideLen(ctx context.Context) (int, func(), error) { return 0, func() {}, nil }
`,
			},
		},
		{
			name: "another package, shared provider and colliding package names",
			config: `
prefix: New
test: true
providers:
  - internal/providers
targets:
  - package: internal/state
    interface: State
    output:
      - name: NewProdState
        package: internal/state/prod
        provider: internal/other/providers
`,
			files: map[string]string{
				"internal/state/state.go": `package state

import "example.com/app/internal/providers"

type State interface {
	Primary() *providers.Store
	Replica() *providers.Store
	Type() string
	Close() func()
}
`,
				"internal/providers/providers.go": `package providers

import "context"

type Store struct{}

func ProvideStore(ctx context.Context) (*Store, func(), error) { return nil, nil, nil }

func ProvideType(ctx context.Context) (string, func(), error) { return "", nil, nil }
`,
				"internal/other/providers/providers.go": `package providers

import "context"

func ProvideClose(ctx context.Context) (func(), func(), error) { return nil, nil, nil }
`,
			},
		},
		{
			name: "two outputs of one interface in one package",
			config: `
prefix: New
test: true
providers:
  - internal/providers
targets:
  - package: internal/app
    interface: Service
    output:
      - name: NewProd
        filename: prod_gen.go
      - name: NewStaging
        filename: staging_gen.go
`,
			files: map[string]string{
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
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.files["go.mod"] = "module " + testModule + "\n\ngo 1.21\n"
			for name, content := range tc.files {
				write(t, filepath.Join(root, name), content)
			}

			cfg, err := config.Parse([]byte(tc.config))
			require.NoError(t, err)
			outs, err := load.Load(root, cfg)
			require.NoError(t, err)
			for _, out := range outs {
				files, err := Render(out)
				require.NoError(t, err)
				for _, f := range files {
					if !f.Remove {
						write(t, filepath.Join(root, f.Path), string(f.Content))
					}
				}
			}

			pkgs, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax, Dir: root}, "./...")
			require.NoError(t, err)
			require.NotEmpty(t, pkgs)
			for _, p := range pkgs {
				require.Empty(t, p.Errors, p.PkgPath)
			}
		})
	}
}

func write(t *testing.T, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(name), 0o750))
	require.NoError(t, os.WriteFile(name, []byte(content), 0o600))
}
