package config

type Config struct {
	Prefix    string   `yaml:"prefix"`
	Test      bool     `yaml:"test"`
	Providers []string `yaml:"providers"`
	Targets   []Target `yaml:"targets"`
}

type Target struct {
	Package   string   `yaml:"package"`
	Interface string   `yaml:"interface"`
	Outputs   []Output `yaml:"output"`
}

type Output struct {
	Name     string `yaml:"name"`
	Package  string `yaml:"package"`
	Filename string `yaml:"filename"`
	// Provider is searched first, then the global providers, then the output package.
	Provider string `yaml:"provider"`
	// Test overrides the global setting; it is never nil once the config is parsed.
	Test *bool `yaml:"test"`
}
