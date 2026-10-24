package config

// Config is a parsed tack.yaml; defaults are applied once it is parsed.
type Config struct {
	Prefix    string   `yaml:"prefix"`
	Suffix    string   `yaml:"suffix"`
	Test      bool     `yaml:"test"`
	Providers []string `yaml:"providers"`
	Targets   []Target `yaml:"targets"`
}

// Target is an interface to generate constructors for.
type Target struct {
	Package   string   `yaml:"package"`
	Interface string   `yaml:"interface"`
	Outputs   []Output `yaml:"output"`
}

// Output is one generated constructor of a Target.
type Output struct {
	Name     string `yaml:"name"`
	Package  string `yaml:"package"`
	Filename string `yaml:"filename"`
	// Provider is searched first, then the global providers, then the output package.
	Provider string `yaml:"provider"`
	// Test overrides the global setting; it is never nil once the config is parsed.
	Test *bool `yaml:"test"`
}
