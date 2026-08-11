// Package config parses and validates tack.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// OutputConfig is one entry under a target's output list in tack.yaml: one
// named implementation to generate for that target's interface.
type OutputConfig struct {
	// Name is the constructor/struct name prefix (New<Name><Interface>).
	Name string
	// Package is the module-relative directory to write the generated file
	// into and scan as this variant's local provider scope. Empty means the
	// target's own Package (see EffectiveDir).
	Package string
	// File is the generated file name. Empty means the default derivation
	// (lower(name)_lower(interface)_gen.go) applies.
	File string
	// LocalScan controls whether this variant's effective directory is
	// scanned as a local provider scope. Defaults to true.
	LocalScan bool
}

// EffectiveDir returns the module-relative directory this output variant
// writes into and scans as its local provider scope: its own Package if set,
// otherwise t's Package.
func (o OutputConfig) EffectiveDir(t TargetConfig) string {
	if o.Package != "" {
		return o.Package
	}
	return t.Package
}

// Filename returns the configured output filename for o, or the default
// derivation (lower(name)_lower(interface)_gen.go) when none is configured.
func (o OutputConfig) Filename(interfaceName string) string {
	if o.File != "" {
		return o.File
	}
	return strings.ToLower(o.Name) + "_" + strings.ToLower(interfaceName) + "_gen.go"
}

// TargetConfig is one entry under top-level targets: in tack.yaml: an
// interface, declared in Package, and one or more named Output variants to
// generate for it.
type TargetConfig struct {
	// Package is the module-relative directory where Interface is declared.
	Package string
	// Interface is the Go interface name declared in Package.
	Interface string
	// Output is the list of named implementations to generate. Always
	// non-empty for a successfully parsed config.
	Output []OutputConfig
}

// Config is a fully parsed and validated tack.yaml.
type Config struct {
	// Path is the absolute path to the tack.yaml file that was loaded.
	Path string
	// ModuleRoot is the absolute path to the directory containing go.mod,
	// against which Providers and Targets directories are resolved.
	ModuleRoot string
	// Providers is the module-relative directory list forming the global
	// provider scope.
	Providers []string
	// Targets is the list of configured interfaces and their output
	// variants, in file declaration order.
	Targets []TargetConfig
}

// Discover walks up from startDir looking for tack.yaml, returning the path
// to the first one found.
func Discover(startDir string) (string, error) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, "tack.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("tack: no tack.yaml found from %s up to filesystem root", startDir)
		}
		dir = parent
	}
}

// FindModuleRoot walks up from startDir looking for go.mod, returning its
// containing directory.
func FindModuleRoot(startDir string) (string, error) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("tack: no go.mod found from %s up to filesystem root", startDir)
		}
		dir = parent
	}
}

// Load resolves the config file to use (explicitPath if non-empty, otherwise
// discovered by walking up from startDir), parses and validates it, and
// resolves its module root.
func Load(explicitPath, startDir string) (*Config, error) {
	configPath := explicitPath
	if configPath == "" {
		p, err := Discover(startDir)
		if err != nil {
			return nil, err
		}
		configPath = p
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("tack: config %s: %w", configPath, err)
	}

	cfg, err := parseFile(abs)
	if err != nil {
		return nil, err
	}

	moduleRoot, err := FindModuleRoot(filepath.Dir(abs))
	if err != nil {
		return nil, err
	}
	cfg.ModuleRoot = moduleRoot
	cfg.Path = abs
	return cfg, nil
}

func parseFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tack: read config %s: %w", path, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("tack: parse config %s: %w", path, err)
	}
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("tack: config %s is empty", path)
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("tack: config %s: expected a mapping at the top level", path)
	}

	cfg := &Config{}

	for i := 0; i < len(doc.Content); i += 2 {
		keyNode, valNode := doc.Content[i], doc.Content[i+1]
		switch keyNode.Value {
		case "providers":
			if err := valNode.Decode(&cfg.Providers); err != nil {
				return nil, fmt.Errorf("tack: config %s: providers: %w", path, err)
			}
		case "targets":
			if err := parseTargets(path, valNode, cfg); err != nil {
				return nil, err
			}
		case "packages":
			return nil, fmt.Errorf("tack: config %s: packages: top-level packages is no longer supported; use targets instead", path)
		default:
			return nil, fmt.Errorf("tack: config %s: unknown key %q at top level", path, keyNode.Value)
		}
	}

	if err := validateNoOutputCollisions(path, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func parseTargets(path string, node *yaml.Node, cfg *Config) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("tack: config %s: targets: expected a list of target entries", path)
	}
	for i, item := range node.Content {
		tc, err := parseTargetConfig(path, fmt.Sprintf("targets[%d]", i), item)
		if err != nil {
			return err
		}
		cfg.Targets = append(cfg.Targets, tc)
	}
	return nil
}

func parseTargetConfig(path, loc string, node *yaml.Node) (TargetConfig, error) {
	if node.Kind != yaml.MappingNode {
		return TargetConfig{}, fmt.Errorf("tack: config %s: %s: expected a mapping", path, loc)
	}

	tc := TargetConfig{}
	var outputNode *yaml.Node
	for i := 0; i < len(node.Content); i += 2 {
		keyNode, valNode := node.Content[i], node.Content[i+1]
		switch keyNode.Value {
		case "package":
			tc.Package = valNode.Value
		case "interface":
			tc.Interface = valNode.Value
		case "output":
			outputNode = valNode
		case "providers":
			return TargetConfig{}, fmt.Errorf("tack: config %s: %s.providers: target-level providers is no longer supported; use the top-level providers list", path, loc)
		default:
			return TargetConfig{}, fmt.Errorf("tack: config %s: %s.%s: unknown key", path, loc, keyNode.Value)
		}
	}

	if tc.Package == "" {
		return TargetConfig{}, fmt.Errorf("tack: config %s: %s: missing required key %q", path, loc, "package")
	}
	if tc.Interface == "" {
		return TargetConfig{}, fmt.Errorf("tack: config %s: %s: missing required key %q", path, loc, "interface")
	}
	if outputNode == nil {
		return TargetConfig{}, fmt.Errorf("tack: config %s: %s: missing required key %q", path, loc, "output")
	}
	if outputNode.Kind != yaml.SequenceNode || len(outputNode.Content) == 0 {
		return TargetConfig{}, fmt.Errorf("tack: config %s: %s.output: expected a non-empty list", path, loc)
	}
	for i, item := range outputNode.Content {
		oc, err := parseOutputConfig(path, fmt.Sprintf("%s.output[%d]", loc, i), item)
		if err != nil {
			return TargetConfig{}, err
		}
		tc.Output = append(tc.Output, oc)
	}

	return tc, nil
}

func parseOutputConfig(path, loc string, node *yaml.Node) (OutputConfig, error) {
	if node.Kind != yaml.MappingNode {
		return OutputConfig{}, fmt.Errorf("tack: config %s: %s: expected a mapping", path, loc)
	}

	oc := OutputConfig{LocalScan: true}
	for i := 0; i < len(node.Content); i += 2 {
		fieldKeyNode, fieldValNode := node.Content[i], node.Content[i+1]
		switch fieldKeyNode.Value {
		case "name":
			oc.Name = fieldValNode.Value
		case "package":
			oc.Package = fieldValNode.Value
		case "file":
			oc.File = fieldValNode.Value
		case "localScan":
			var b bool
			if err := fieldValNode.Decode(&b); err != nil {
				return OutputConfig{}, fmt.Errorf("tack: config %s: %s.localScan: %w", path, loc, err)
			}
			oc.LocalScan = b
		case "providers":
			return OutputConfig{}, fmt.Errorf("tack: config %s: %s.providers: output-level providers is no longer supported; use the top-level providers list", path, loc)
		default:
			return OutputConfig{}, fmt.Errorf("tack: config %s: %s.%s: unknown key", path, loc, fieldKeyNode.Value)
		}
	}
	if oc.Name == "" {
		return OutputConfig{}, fmt.Errorf("tack: config %s: %s: missing required key %q", path, loc, "name")
	}
	return oc, nil
}

// validateNoOutputCollisions fails if two output entries — within a target
// or across different targets — resolve to the same effective directory and
// filename, before any scanning or generation runs.
func validateNoOutputCollisions(path string, cfg *Config) error {
	type key struct{ dir, file string }
	seen := map[key]string{}
	for ti, t := range cfg.Targets {
		for oi, o := range t.Output {
			k := key{o.EffectiveDir(t), o.Filename(t.Interface)}
			loc := fmt.Sprintf("targets[%d].output[%d]", ti, oi)
			if prev, ok := seen[k]; ok {
				return fmt.Errorf("tack: config %s: %s and %s both write %s/%s", path, prev, loc, k.dir, k.file)
			}
			seen[k] = loc
		}
	}
	return nil
}
