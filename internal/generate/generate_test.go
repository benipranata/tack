package generate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benipranata/goldie/pkg/goldie"
	"github.com/stretchr/testify/require"

	"github.com/benipranata/tack/internal/render"
)

const module = "example.com/app"

// testdata is absolute so that a test which changes the working directory still finds it.
var testdata = func() string {
	dir, err := filepath.Abs("testdata")
	if err != nil {
		panic(err)
	}
	return dir
}()

// goldenEqual compares got with the golden file of the running subtest. Each top-level test keeps its files in its
// own folder, because goldie's -clean removes every golden file of its fixture folder: with one shared folder,
// the go:generate directive of a single test would delete the files of all the others.
func goldenEqual(t *testing.T, got []byte) {
	t.Helper()
	top, name, found := strings.Cut(t.Name(), "/")
	if !found {
		name = top
	}
	goldie.New(t, goldie.WithFixtureDir(filepath.Join(testdata, top))).Equal(name, got)
}

const serviceSource = `package app

import (
	"database/sql"
	"log/slog"
)

type Service interface {
	DB() *sql.DB
	Logger() *slog.Logger
}
`

const providersSource = `package providers

import (
	"context"
	"database/sql"
	"log/slog"
)

func ProvideDB(ctx context.Context) (*sql.DB, func(), error) { return nil, nil, nil }

func ProvideLogger(ctx context.Context) (*slog.Logger, func(), error) { return nil, nil, nil }
`

const quickstartConfig = `
prefix: New
test: true
providers:
  - internal/providers
targets:
  - package: internal/app
    interface: Service
`

// moduleFiles returns a module with the quickstart sources and the given tack.yaml.
func moduleFiles(config string) map[string]string {
	return map[string]string{
		"go.mod":                          "module " + module + "\n\ngo 1.21\n",
		"tack.yaml":                       config,
		"internal/app/service.go":         serviceSource,
		"internal/providers/providers.go": providersSource,
	}
}

// dumpGenerated lists every file tack generated under root.
func dumpGenerated(t *testing.T, root string) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err == nil && render.IsGenerated(b) {
			rel, _ := filepath.Rel(root, p)
			fmt.Fprintf(&buf, "=== %s\n%s", filepath.ToSlash(rel), b)
		}
		return err
	}))
	return buf.Bytes()
}

//go:generate go test -v -update -clean -run '^TestRun$'
func TestRun(t *testing.T) {
	tests := []struct {
		name string
		// run generates from the module in root.
		run func(t *testing.T, root string) error
	}{
		{
			name: "explicit config",
			run: func(t *testing.T, root string) error {
				return Run(filepath.Join(root, "tack.yaml"))
			},
		},
		{
			name: "discovered from a subdirectory",
			run: func(t *testing.T, root string) error {
				t.Chdir(filepath.Join(root, "internal", "app"))
				return Run("")
			},
		},
		{
			name: "repeated run",
			run: func(t *testing.T, root string) error {
				require.NoError(t, Run(filepath.Join(root, "tack.yaml")))
				return Run(filepath.Join(root, "tack.yaml"))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, moduleFiles(quickstartConfig))

			require.NoError(t, tc.run(t, root))

			goldenEqual(t, dumpGenerated(t, root))
		})
	}
}

func TestRun_errors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		// dir is the working directory, relative to the module; it defaults to the module root.
		dir     string
		config  string
		wantErr string
	}{
		{
			name:    "no config",
			files:   map[string]string{"go.mod": "module " + module + "\n"},
			wantErr: "tack.yaml not found",
		},
		{
			name:    "explicit config is missing",
			files:   map[string]string{"go.mod": "module " + module + "\n"},
			config:  "missing.yaml",
			wantErr: "missing.yaml",
		},
		{
			name:    "invalid config",
			files:   moduleFiles("unknown: true\n"),
			wantErr: "field unknown not found",
		},
		{
			name:    "config below the module root is discovered",
			files:   map[string]string{"go.mod": "module " + module + "\n", "sub/tack.yaml": quickstartConfig},
			dir:     "sub",
			wantErr: "the config must be in the module root, next to go.mod",
		},
		{
			name:    "explicit config below the module root",
			files:   map[string]string{"go.mod": "module " + module + "\n", "sub/tack.yaml": quickstartConfig},
			config:  "sub/tack.yaml",
			wantErr: "the config must be in the module root, next to go.mod",
		},
		{
			name: "unbound method",
			files: func() map[string]string {
				f := moduleFiles(quickstartConfig)
				f["internal/providers/providers.go"] = "package providers\n"
				return f
			}(),
			wantErr: "DB(): no provider returns *sql.DB",
		},
		{
			name: "hand written file in the way",
			files: func() map[string]string {
				f := moduleFiles(quickstartConfig)
				f["internal/app/service_gen.go"] = "package app\n"
				return f
			}(),
			wantErr: "refusing to overwrite a file not generated by tack",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, tc.files)
			t.Chdir(filepath.Join(root, tc.dir))
			if tc.config != "" {
				tc.config = filepath.Join(root, tc.config)
			}

			require.ErrorContains(t, Run(tc.config), tc.wantErr)
		})
	}
}

func TestRun_removesDouble(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, moduleFiles(quickstartConfig))
	config := filepath.Join(root, "tack.yaml")
	require.NoError(t, Run(config))
	require.FileExists(t, filepath.Join(root, "internal/app/service_test_gen.go"))

	writeFiles(t, root, map[string]string{"tack.yaml": "prefix: New\nproviders: [internal/providers]\ntargets:\n  - package: internal/app\n    interface: Service\n"})
	require.NoError(t, Run(config))

	require.NoFileExists(t, filepath.Join(root, "internal/app/service_test_gen.go"))
	require.FileExists(t, filepath.Join(root, "internal/app/service_gen.go"))
}
