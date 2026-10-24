package discover

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// lock makes dir unreadable for the test; it skips where permission bits are not enforced.
func lock(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced")
	}
	require.NoError(t, os.Chmod(dir, 0))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })
}

func write(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, nil, 0o600))
	}
}

func TestDiscover(t *testing.T) {
	tests := []struct {
		name string
		// setup prepares root and returns the directory to start from.
		setup   func(t *testing.T, root string) string
		want    string
		wantErr string
	}{
		{
			name: "in the start directory",
			setup: func(t *testing.T, root string) string {
				write(t, root, "tack.yaml")
				return root
			},
			want: "tack.yaml",
		},
		{
			name: "in a parent",
			setup: func(t *testing.T, root string) string {
				write(t, root, "tack.yaml", "a/b/x")
				return filepath.Join(root, "a/b")
			},
			want: "tack.yaml",
		},
		{
			name: "nearest wins",
			setup: func(t *testing.T, root string) string {
				write(t, root, "tack.yaml", "a/tack.yaml", "a/b/x")
				return filepath.Join(root, "a/b")
			},
			want: "a/tack.yaml",
		},
		{
			name: "next to go.mod",
			setup: func(t *testing.T, root string) string {
				write(t, root, "tack.yaml", "go.mod", "a/x")
				return filepath.Join(root, "a")
			},
			want: "tack.yaml",
		},
		{
			name: "stops at go.mod",
			setup: func(t *testing.T, root string) string {
				write(t, root, "tack.yaml", "m/go.mod", "m/a/x")
				return filepath.Join(root, "m/a")
			},
			wantErr: "tack.yaml not found",
		},
		{
			name: "directory named tack.yaml is skipped",
			setup: func(t *testing.T, root string) string {
				write(t, root, "go.mod", "tack.yaml/x")
				return root
			},
			wantErr: "tack.yaml not found",
		},
		{
			name: "relative start",
			setup: func(t *testing.T, root string) string {
				write(t, root, "tack.yaml", "a/x")
				t.Chdir(root)
				return "a"
			},
			want: "tack.yaml",
		},
		{
			name: "filesystem root",
			setup: func(*testing.T, string) string {
				return string(filepath.Separator)
			},
			wantErr: "tack.yaml not found",
		},
		{
			name: "go.mod stat error",
			setup: func(t *testing.T, root string) string {
				if runtime.GOOS == "windows" {
					t.Skip("symlinks need privileges")
				}
				require.NoError(t, os.Symlink("go.mod", filepath.Join(root, "go.mod")))
				return root
			},
			wantErr: "too many levels of symbolic links",
		},
		{
			name: "unreadable directory",
			setup: func(t *testing.T, root string) string {
				write(t, root, "locked/x")
				lock(t, filepath.Join(root, "locked"))
				return filepath.Join(root, "locked", "child")
			},
			wantErr: "permission denied",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)

			got, err := Discover(tc.setup(t, root))
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, filepath.Join(root, tc.want), got)
		})
	}
}

func TestIsFile(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) string
		want    bool
		wantErr string
	}{
		{
			name: "file",
			setup: func(t *testing.T, root string) string {
				write(t, root, "f")
				return filepath.Join(root, "f")
			},
			want: true,
		},
		{
			name:  "directory",
			setup: func(_ *testing.T, root string) string { return root },
		},
		{
			name:  "missing",
			setup: func(_ *testing.T, root string) string { return filepath.Join(root, "nope") },
		},
		{
			name: "stat error",
			setup: func(t *testing.T, root string) string {
				write(t, root, "locked/f")
				lock(t, filepath.Join(root, "locked"))
				return filepath.Join(root, "locked", "f")
			},
			wantErr: "permission denied",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := isFile(tc.setup(t, t.TempDir()))
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
