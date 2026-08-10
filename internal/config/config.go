// Package config parses and validates tack.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// InterfaceConfig is one entry under packages.<dir>.<InterfaceName> in tack.yaml.
type InterfaceConfig struct {
	// Name is the constructor/struct name prefix (New<Name><Interface>).
	Name string
	// Output is the generated file name. Empty means the default
	// derivation (lower(Name)_lower(interface)_gen.go) applies.
	Output string
	// LocalScan controls whether the interface's own package directory is
	// scanned as a local provider scope. Defaults to true.
	LocalScan bool
}

// Config is a fully parsed and validated tack.yaml.
type Config struct {
	// Path is the absolute path to the tack.yaml file that was loaded.
	Path string
	// ModuleRoot is the absolute path to the directory containing go.mod,
	// against which Providers and Packages directories are resolved.
	ModuleRoot string
	// Providers is the module-relative directory list forming the global
	// provider scope.
	Providers []string
	// Packages maps a module-relative package directory to its configured
	// interfaces, keyed by interface name.
	Packages map[string]map[string]InterfaceConfig
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

	cfg := &Config{Packages: map[string]map[string]InterfaceConfig{}}

	for i := 0; i < len(doc.Content); i += 2 {
		keyNode, valNode := doc.Content[i], doc.Content[i+1]
		switch keyNode.Value {
		case "providers":
			if err := valNode.Decode(&cfg.Providers); err != nil {
				return nil, fmt.Errorf("tack: config %s: providers: %w", path, err)
			}
		case "packages":
			if err := parsePackages(path, valNode, cfg); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("tack: config %s: unknown key %q at top level", path, keyNode.Value)
		}
	}

	return cfg, nil
}

func parsePackages(path string, node *yaml.Node, cfg *Config) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("tack: config %s: packages: expected a mapping of package directory to interfaces", path)
	}
	for i := 0; i < len(node.Content); i += 2 {
		pkgKeyNode, pkgValNode := node.Content[i], node.Content[i+1]
		pkgPath := pkgKeyNode.Value
		if pkgValNode.Kind != yaml.MappingNode {
			return fmt.Errorf("tack: config %s: packages.%s: expected a mapping of interface name to config", path, pkgPath)
		}

		interfaces := map[string]InterfaceConfig{}
		for j := 0; j < len(pkgValNode.Content); j += 2 {
			ifaceKeyNode, ifaceValNode := pkgValNode.Content[j], pkgValNode.Content[j+1]
			ifaceName := ifaceKeyNode.Value

			if ifaceName == "providers" {
				return fmt.Errorf("tack: config %s: packages.%s.providers: package-level providers is no longer supported; use the top-level providers list", path, pkgPath)
			}

			ic, err := parseInterfaceConfig(path, pkgPath, ifaceName, ifaceValNode)
			if err != nil {
				return err
			}
			interfaces[ifaceName] = ic
		}
		cfg.Packages[pkgPath] = interfaces
	}
	return nil
}

func parseInterfaceConfig(path, pkgPath, ifaceName string, node *yaml.Node) (InterfaceConfig, error) {
	if node.Kind != yaml.MappingNode {
		return InterfaceConfig{}, fmt.Errorf("tack: config %s: packages.%s.%s: expected a mapping", path, pkgPath, ifaceName)
	}

	ic := InterfaceConfig{LocalScan: true}
	for k := 0; k < len(node.Content); k += 2 {
		fieldKeyNode, fieldValNode := node.Content[k], node.Content[k+1]
		switch fieldKeyNode.Value {
		case "name":
			ic.Name = fieldValNode.Value
		case "output":
			ic.Output = fieldValNode.Value
		case "localScan":
			var b bool
			if err := fieldValNode.Decode(&b); err != nil {
				return InterfaceConfig{}, fmt.Errorf("tack: config %s: packages.%s.%s.localScan: %w", path, pkgPath, ifaceName, err)
			}
			ic.LocalScan = b
		case "providers":
			return InterfaceConfig{}, fmt.Errorf("tack: config %s: packages.%s.%s.providers: interface-level providers is no longer supported; use the top-level providers list", path, pkgPath, ifaceName)
		default:
			return InterfaceConfig{}, fmt.Errorf("tack: config %s: packages.%s.%s.%s: unknown key", path, pkgPath, ifaceName, fieldKeyNode.Value)
		}
	}
	if ic.Name == "" {
		return InterfaceConfig{}, fmt.Errorf("tack: config %s: packages.%s.%s: missing required key %q", path, pkgPath, ifaceName, "name")
	}
	return ic, nil
}
