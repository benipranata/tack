package config

import (
	"os"
	"path/filepath"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    *Config
		wantErr string
	}{
		{
			name: "decodes and resolves",
			in: `
prefix: New
targets:
  - package: ./cache
    interface: Redis
`,
			want: &Config{
				Prefix: "New",
				Targets: []Target{{
					Package:   "cache",
					Interface: "Redis",
					Outputs:   []Output{{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go"}},
				}},
			},
		},
		{
			name:    "empty file",
			in:      "",
			wantErr: "config is empty",
		},
		{
			name:    "invalid yaml",
			in:      "targets: [",
			wantErr: "yaml:",
		},
		{
			name: "unknown key",
			in: `
targets:
  - package: a
    interface: A
    providers:
      - b
`,
			wantErr: "field providers not found",
		},
		{
			name:    "resolve error is returned",
			in:      "prefix: New",
			wantErr: "targets: at least one target is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse([]byte(tc.in))
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content *string
		want    *Config
		wantErr string
	}{
		{
			name: "valid file",
			content: ptr(`
prefix: New
targets:
  - package: a
    interface: A
`),
			want: &Config{
				Prefix: "New",
				Targets: []Target{{
					Package:   "a",
					Interface: "A",
					Outputs:   []Output{{Name: "NewA", Package: "a", Filename: "a_gen.go"}},
				}},
			},
		},
		{
			name:    "invalid file names the path",
			content: ptr(""),
			wantErr: "tack.yaml: config is empty",
		},
		{
			name:    "missing file",
			wantErr: "no such file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tack.yaml")
			if tc.content != nil {
				require.NoError(t, os.WriteFile(path, []byte(*tc.content), 0o600))
			}

			got, err := Load(path)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestResolveConfig(t *testing.T) {
	redis := Target{Package: "cache", Interface: "Redis"}

	tests := []struct {
		name    string
		in      Config
		want    Config
		wantErr []string
	}{
		{
			name: "normalises providers and resolves targets",
			in:   Config{Prefix: "New", Providers: []string{"./infra/", "pkg"}, Targets: []Target{redis}},
			want: Config{
				Prefix:    "New",
				Providers: []string{"infra", "pkg"},
				Targets: []Target{{
					Package:   "cache",
					Interface: "Redis",
					Outputs:   []Output{{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go"}},
				}},
			},
		},
		{
			name:    "no targets",
			in:      Config{},
			wantErr: []string{"targets: at least one target is required"},
		},
		{
			name:    "invalid provider",
			in:      Config{Providers: []string{"/abs"}, Targets: []Target{redis}, Prefix: "New"},
			wantErr: []string{`providers[0]: "/abs" must be relative to the module root`},
		},
		{
			name:    "target error carries its index",
			in:      Config{Prefix: "New", Targets: []Target{redis, {Interface: "B"}}},
			wantErr: []string{"targets[1].package: required"},
		},
		{
			name:    "duplicates are reported",
			in:      Config{Prefix: "New", Targets: []Target{redis, redis}},
			wantErr: []string{`targets[1].output[0].name: "NewRedis" already declared by targets[0].output[0]`},
		},
		{
			name:    "other errors suppress the duplicate check",
			in:      Config{Prefix: "New", Providers: []string{"/abs"}, Targets: []Target{redis, redis}},
			wantErr: []string{"providers[0]"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			err := resolveConfig(&got)
			if tc.wantErr != nil {
				require.Error(t, err)
				for _, w := range tc.wantErr {
					require.ErrorContains(t, err, w)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestResolveTarget(t *testing.T) {
	tests := []struct {
		name    string
		in      Target
		prefix  string
		want    Target
		wantErr []string
	}{
		{
			name:   "defaults one output",
			in:     Target{Package: "./cache", Interface: "Redis"},
			prefix: "New",
			want: Target{
				Package:   "cache",
				Interface: "Redis",
				Outputs:   []Output{{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go"}},
			},
		},
		{
			name: "keeps explicit outputs and normalises provider",
			in: Target{
				Package:   "cache",
				Interface: "Redis",
				Provider:  "./p/",
				Outputs:   []Output{{Name: "Open", Package: "wire", Filename: "open_gen.go"}},
			},
			want: Target{
				Package:   "cache",
				Interface: "Redis",
				Provider:  "p",
				Outputs:   []Output{{Name: "Open", Package: "wire", Filename: "open_gen.go"}},
			},
		},
		{
			name:    "missing package",
			in:      Target{Interface: "A"},
			wantErr: []string{"targets[0].package: required"},
		},
		{
			name:    "missing interface",
			in:      Target{Package: "a"},
			wantErr: []string{"targets[0].interface: required"},
		},
		{
			name:    "invalid interface",
			in:      Target{Package: "a", Interface: "1x"},
			wantErr: []string{`targets[0].interface: "1x" is not a valid Go identifier`},
		},
		{
			name:    "invalid provider",
			in:      Target{Package: "a", Interface: "A", Provider: "/p"},
			wantErr: []string{`targets[0].provider: "/p" must be relative to the module root`},
		},
		{
			name:    "reports every target problem together",
			in:      Target{},
			wantErr: []string{"targets[0].package: required", "targets[0].interface: required"},
		},
		{
			name:    "output error carries the output index",
			in:      Target{Package: "a", Interface: "A"},
			wantErr: []string{`targets[0].output[0].name: "A" collides with the interface`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			err := resolveTarget(&got, tc.prefix, "targets[0]")
			if tc.wantErr != nil {
				require.Error(t, err)
				for _, w := range tc.wantErr {
					require.ErrorContains(t, err, w)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestResolveOutput(t *testing.T) {
	redis := Target{Package: "cache", Interface: "Redis"}

	tests := []struct {
		name    string
		in      Output
		prefix  string
		want    Output
		wantErr []string
	}{
		{
			name:   "all defaults",
			in:     Output{},
			prefix: "New",
			want:   Output{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go"},
		},
		{
			name: "explicit values are kept, package normalised",
			in:   Output{Name: "Open", Package: "./wire/", Filename: "open_gen.go"},
			want: Output{Name: "Open", Package: "wire", Filename: "open_gen.go"},
		},
		{
			name:    "invalid package",
			in:      Output{Name: "Open", Package: "../x"},
			wantErr: []string{`o[0].package: "../x" must stay inside the module`},
		},
		{
			name:    "invalid name",
			in:      Output{Name: "1x"},
			wantErr: []string{`o[0].name: "1x" is not a valid Go identifier`},
		},
		{
			name:    "default name collides with the interface",
			in:      Output{},
			wantErr: []string{`o[0].name: "Redis" collides with the interface`},
		},
		{
			name:    "explicit name collides with the interface",
			in:      Output{Name: "Redis"},
			prefix:  "New",
			wantErr: []string{`o[0].name: "Redis" collides with the interface`},
		},
		{
			name: "same name as the interface is fine in another package",
			in:   Output{Package: "wire"},
			want: Output{Name: "Redis", Package: "wire", Filename: "redis_gen.go"},
		},
		{
			name:    "filename is a path",
			in:      Output{Name: "Open", Filename: "x/y.go"},
			wantErr: []string{`o[0].filename: "x/y.go" must be a file name, not a path`},
		},
		{
			name:    "filename is not a go file",
			in:      Output{Name: "Open", Filename: "x.txt"},
			wantErr: []string{`o[0].filename: "x.txt" must end in .go and not _test.go`},
		},
		{
			name:    "filename is a test file",
			in:      Output{Name: "Open", Filename: "x_test.go"},
			wantErr: []string{`o[0].filename: "x_test.go" must end in .go and not _test.go`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			err := resolveOutput(&got, &redis, tc.prefix, "o[0]")
			if tc.wantErr != nil {
				require.Error(t, err)
				for _, w := range tc.wantErr {
					require.ErrorContains(t, err, w)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCheckDuplicates(t *testing.T) {
	out := func(pkg, name, file string) []Output {
		return []Output{{Name: name, Package: pkg, Filename: file}}
	}

	tests := []struct {
		name    string
		in      []Target
		wantErr []string
	}{
		{
			name: "no targets",
		},
		{
			name: "distinct outputs",
			in: []Target{
				{Outputs: out("a", "NewA", "a_gen.go")},
				{Outputs: out("a", "NewB", "b_gen.go")},
			},
		},
		{
			name: "same name and file in different packages",
			in: []Target{
				{Outputs: out("a", "New", "gen.go")},
				{Outputs: out("b", "New", "gen.go")},
			},
		},
		{
			name: "duplicate name",
			in: []Target{
				{Outputs: out("a", "New", "x_gen.go")},
				{Outputs: out("a", "New", "y_gen.go")},
			},
			wantErr: []string{`targets[1].output[0].name: "New" already declared by targets[0].output[0]`},
		},
		{
			name: "duplicate filename",
			in: []Target{
				{Outputs: out("a", "NewX", "gen.go")},
				{Outputs: out("a", "NewY", "gen.go")},
			},
			wantErr: []string{`targets[1].output[0].filename: "gen.go" already written by targets[0].output[0]`},
		},
		{
			name: "duplicates within one target",
			in: []Target{{Outputs: []Output{
				{Name: "New", Package: "a", Filename: "gen.go"},
				{Name: "New", Package: "a", Filename: "gen.go"},
			}}},
			wantErr: []string{
				`targets[0].output[1].name: "New" already declared by targets[0].output[0]`,
				`targets[0].output[1].filename: "gen.go" already written by targets[0].output[0]`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkDuplicates(tc.in)
			if tc.wantErr != nil {
				require.Error(t, err)
				for _, w := range tc.wantErr {
					require.ErrorContains(t, err, w)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDefaultName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		iface  string
		want   string
	}{
		{name: "no prefix keeps the interface name", iface: "Redis", want: "Redis"},
		{name: "no prefix keeps an unexported name", iface: "redis", want: "redis"},
		{name: "exported interface", prefix: "New", iface: "Redis", want: "NewRedis"},
		{name: "unexported interface", prefix: "New", iface: "redis", want: "newRedis"},
		{name: "lowercase prefix stays lowercase", prefix: "new", iface: "Redis", want: "newRedis"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, defaultName(tc.prefix, tc.iface))
		})
	}
}

func TestMapFirst(t *testing.T) {
	tests := []struct {
		name string
		in   string
		f    func(rune) rune
		want string
	}{
		{name: "upper", in: "redis", f: unicode.ToUpper, want: "Redis"},
		{name: "lower", in: "Redis", f: unicode.ToLower, want: "redis"},
		{name: "multi-byte first rune", in: "élan", f: unicode.ToUpper, want: "Élan"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, mapFirst(tc.in, tc.f))
		})
	}
}

func TestCleanPath(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "plain", in: "internal/x", want: "internal/x"},
		{name: "dot slash and trailing slash", in: "./internal/x/", want: "internal/x"},
		{name: "repeated separators", in: "a//b", want: "a/b"},
		{name: "module root", in: ".", want: "."},
		{name: "empty", in: "", wantErr: "required"},
		{name: "absolute", in: "/abs", wantErr: `"/abs" must be relative to the module root`},
		{name: "parent", in: "..", wantErr: `".." must stay inside the module`},
		{name: "parent prefix", in: "../x", wantErr: `"../x" must stay inside the module`},
		{name: "escapes after cleaning", in: "a/../../x", wantErr: `"a/../../x" must stay inside the module`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cleanPath(tc.in)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func ptr[T any](v T) *T { return &v }
