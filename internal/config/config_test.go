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
					Outputs:   []Output{{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)}},
				}},
			},
		},
		{
			name: "suffix names the constructor",
			in: `
suffix: State
targets:
  - package: cache
    interface: Redis
`,
			want: &Config{
				Suffix: "State",
				Targets: []Target{{
					Package:   "cache",
					Interface: "Redis",
					Outputs:   []Output{{Name: "RedisState", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)}},
				}},
			},
		},
		{
			name: "no prefix keeps the interface name",
			in: `
targets:
  - package: cache
    interface: Redis
    output:
      - package: cache/prod
`,
			want: &Config{
				Targets: []Target{{
					Package:   "cache",
					Interface: "Redis",
					Outputs:   []Output{{Name: "Redis", Package: "cache/prod", Filename: "redis_gen.go", Test: ptr(false)}},
				}},
			},
		},
		{
			name: "test setting is global with an output override",
			in: `
test: true
targets:
  - package: a
    interface: A
    output:
      - name: NewA
      - name: Other
        filename: other_gen.go
        test: false
`,
			want: &Config{
				Test: true,
				Targets: []Target{{
					Package:   "a",
					Interface: "A",
					Outputs: []Output{
						{Name: "NewA", Package: "a", Filename: "a_gen.go", Test: ptr(true)},
						{Name: "Other", Package: "a", Filename: "other_gen.go", Test: ptr(false)},
					},
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
		// bare means the error must not mention the file path.
		bare bool
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
					Outputs:   []Output{{Name: "NewA", Package: "a", Filename: "a_gen.go", Test: ptr(false)}},
				}},
			},
		},
		{
			name:    "empty file",
			content: ptr(""),
			wantErr: "config is empty",
			bare:    true,
		},
		{
			name:    "every validation error starts with its field",
			content: ptr("targets:\n  - package: a\n  - interface: B\n"),
			wantErr: "targets[0].interface: required\ntargets[1].package: required",
			bare:    true,
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
				if tc.bare {
					require.NotContains(t, err.Error(), path, "the file name belongs on read errors only")
				}
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
					Outputs:   []Output{{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)}},
				}},
			},
		},
		{
			name: "prefix and suffix reach every target",
			in:   Config{Prefix: "New", Suffix: "State", Targets: []Target{redis, {Package: "cache", Interface: "lru"}}},
			want: Config{
				Prefix: "New",
				Suffix: "State",
				Targets: []Target{
					{
						Package:   "cache",
						Interface: "Redis",
						Outputs:   []Output{{Name: "NewRedisState", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)}},
					},
					{
						Package:   "cache",
						Interface: "lru",
						Outputs:   []Output{{Name: "newLruState", Package: "cache", Filename: "lru_gen.go", Test: ptr(false)}},
					},
				},
			},
		},
		{
			name: "global test setting reaches every output",
			in:   Config{Prefix: "New", Test: true, Targets: []Target{redis}},
			want: Config{
				Prefix: "New",
				Test:   true,
				Targets: []Target{{
					Package:   "cache",
					Interface: "Redis",
					Outputs:   []Output{{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go", Test: ptr(true)}},
				}},
			},
		},
		{
			name:    "test double names count as duplicates",
			in:      Config{Prefix: "New", Test: true, Targets: []Target{redis, {Package: "cache", Interface: "RedisTest"}}},
			wantErr: []string{`targets[1].output[0].name: "NewRedisTest" already declared by targets[0].output[0]`},
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
		suffix  string
		test    bool
		want    Target
		wantErr []string
	}{
		{
			name:   "suffix reaches the default name",
			in:     Target{Package: "cache", Interface: "Redis"},
			suffix: "State",
			want: Target{
				Package:   "cache",
				Interface: "Redis",
				Outputs:   []Output{{Name: "RedisState", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)}},
			},
		},
		{
			name:   "defaults one output",
			in:     Target{Package: "./cache", Interface: "Redis"},
			prefix: "New",
			want: Target{
				Package:   "cache",
				Interface: "Redis",
				Outputs:   []Output{{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)}},
			},
		},
		{
			name:   "global test setting reaches the default output",
			in:     Target{Package: "cache", Interface: "Redis"},
			prefix: "New",
			test:   true,
			want: Target{
				Package:   "cache",
				Interface: "Redis",
				Outputs:   []Output{{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go", Test: ptr(true)}},
			},
		},
		{
			name: "keeps explicit outputs",
			in: Target{
				Package:   "cache",
				Interface: "Redis",
				Outputs:   []Output{{Name: "Open", Package: "wire", Filename: "open_gen.go"}},
			},
			want: Target{
				Package:   "cache",
				Interface: "Redis",
				Outputs:   []Output{{Name: "Open", Package: "wire", Filename: "open_gen.go", Test: ptr(false)}},
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
			err := resolveTarget(&got, tc.prefix, tc.suffix, tc.test, "targets[0]")
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
		suffix  string
		test    bool
		want    Output
		wantErr []string
	}{
		{
			name:   "all defaults",
			in:     Output{},
			prefix: "New",
			want:   Output{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)},
		},
		{
			name:   "prefix and suffix",
			in:     Output{},
			prefix: "New",
			suffix: "State",
			want:   Output{Name: "NewRedisState", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)},
		},
		{
			name:   "suffix alone avoids the collision with the interface",
			in:     Output{},
			suffix: "State",
			want:   Output{Name: "RedisState", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)},
		},
		{
			name:   "an explicit name ignores prefix and suffix",
			in:     Output{Name: "Make"},
			prefix: "New",
			suffix: "State",
			want:   Output{Name: "Make", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)},
		},
		{
			name:   "inherits the global test setting",
			in:     Output{},
			prefix: "New",
			test:   true,
			want:   Output{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go", Test: ptr(true)},
		},
		{
			name:   "turns the global test setting off",
			in:     Output{Test: ptr(false)},
			prefix: "New",
			test:   true,
			want:   Output{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go", Test: ptr(false)},
		},
		{
			name:   "turns the test setting on over a global off",
			in:     Output{Test: ptr(true)},
			prefix: "New",
			want:   Output{Name: "NewRedis", Package: "cache", Filename: "redis_gen.go", Test: ptr(true)},
		},
		{
			name: "explicit values are kept, paths normalised",
			in:   Output{Name: "Open", Package: "./wire/", Filename: "open_gen.go", Provider: "./p/"},
			want: Output{Name: "Open", Package: "wire", Filename: "open_gen.go", Provider: "p", Test: ptr(false)},
		},
		{
			name:    "invalid provider",
			in:      Output{Name: "Open", Provider: "/p"},
			wantErr: []string{`o[0].provider: "/p" must be relative to the module root`},
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
			want: Output{Name: "Redis", Package: "wire", Filename: "redis_gen.go", Test: ptr(false)},
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
			err := resolveOutput(&got, &redis, tc.prefix, tc.suffix, tc.test, "o[0]")
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
		return []Output{{Name: name, Package: pkg, Filename: file, Test: ptr(false)}}
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
				{Name: "New", Package: "a", Filename: "gen.go", Test: ptr(false)},
				{Name: "New", Package: "a", Filename: "gen.go", Test: ptr(false)},
			}}},
			wantErr: []string{
				`targets[0].output[1].name: "New" already declared by targets[0].output[0]`,
				`targets[0].output[1].filename: "gen.go" already written by targets[0].output[0]`,
			},
		},
		{
			name: "test double names and file are reserved when test is on",
			in: []Target{{Outputs: []Output{
				{Name: "New", Package: "a", Filename: "x_gen.go", Test: ptr(true)},
				{Name: "NewTest", Package: "a", Filename: "x_test_gen.go", Test: ptr(false)},
			}}},
			wantErr: []string{
				`targets[0].output[1].name: "NewTest" already declared by targets[0].output[0]`,
				`targets[0].output[1].filename: "x_test_gen.go" already written by targets[0].output[0]`,
			},
		},
		{
			name: "test double fields struct is reserved when test is on",
			in: []Target{{Outputs: []Output{
				{Name: "New", Package: "a", Filename: "x_gen.go", Test: ptr(true)},
				{Name: "NewTestFields", Package: "a", Filename: "y_gen.go", Test: ptr(false)},
			}}},
			wantErr: []string{`targets[0].output[1].name: "NewTestFields" already declared by targets[0].output[0]`},
		},
		{
			name: "name equal to another output's test constructor, both with test on",
			in: []Target{{Outputs: []Output{
				{Name: "New", Package: "a", Filename: "x_gen.go", Test: ptr(true)},
				{Name: "NewTest", Package: "a", Filename: "y_gen.go", Test: ptr(true)},
			}}},
			wantErr: []string{`targets[0].output[1].name: "NewTest" already declared by targets[0].output[0]`},
		},
		{
			name: "nothing is reserved when test is off",
			in: []Target{{Outputs: []Output{
				{Name: "New", Package: "a", Filename: "x_gen.go", Test: ptr(false)},
				{Name: "NewTest", Package: "a", Filename: "x_test_gen.go", Test: ptr(false)},
			}}},
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

func TestDoubleName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "exported", in: "NewRedis", want: "NewRedisTest"},
		{name: "unexported", in: "newRedis", want: "newRedisTest"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, DoubleName(tc.in))
		})
	}
}

func TestDoubleFields(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "exported", in: "NewRedis", want: "NewRedisTestFields"},
		{name: "unexported", in: "newRedis", want: "newRedisTestFields"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, DoubleFields(tc.in))
		})
	}
}

func TestDoubleFilename(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "default generated name", in: "service_gen.go", want: "service_test_gen.go"},
		{name: "custom name without the gen suffix", in: "service.go", want: "service_test_gen.go"},
		{name: "gen only in the middle", in: "gen_service.go", want: "gen_service_test_gen.go"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, DoubleFilename(tc.in))
		})
	}
}

func TestDefaultName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		suffix string
		iface  string
		want   string
	}{
		{name: "no prefix keeps the interface name", iface: "Redis", want: "Redis"},
		{name: "no prefix keeps an unexported name", iface: "redis", want: "redis"},
		{name: "suffix only", suffix: "State", iface: "Redis", want: "RedisState"},
		{name: "suffix on an unexported interface", suffix: "State", iface: "redis", want: "redisState"},
		{name: "prefix and suffix", prefix: "New", suffix: "State", iface: "Redis", want: "NewRedisState"},
		{name: "prefix and suffix on an unexported interface", prefix: "New", suffix: "State", iface: "redis", want: "newRedisState"},
		{name: "exported interface", prefix: "New", iface: "Redis", want: "NewRedis"},
		{name: "unexported interface", prefix: "New", iface: "redis", want: "newRedis"},
		{name: "lowercase prefix stays lowercase", prefix: "new", iface: "Redis", want: "newRedis"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, defaultName(tc.prefix, tc.iface, tc.suffix))
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
