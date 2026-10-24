// Package config parses and validates tack.yaml; package paths are relative to the module root.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"io"
	"os"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// Parse returns a Config with defaults applied and paths normalised.
func Parse(data []byte) (*Config, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("config is empty")
		}
		return nil, err
	}
	if err := resolveConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Load reads and parses the config at file. Parse errors are returned as they are: each already names its field,
// and wrapping the joined errors would put the file name on the first line only.
func Load(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func resolveConfig(c *Config) error {
	var errs []error

	for i, p := range c.Providers {
		clean, err := cleanPath(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("providers[%d]: %w", i, err))
			continue
		}
		c.Providers[i] = clean
	}

	if len(c.Targets) == 0 {
		errs = append(errs, errors.New("targets: at least one target is required"))
	}
	for i := range c.Targets {
		at := fmt.Sprintf("targets[%d]", i)
		errs = append(errs, resolveTarget(&c.Targets[i], c.Prefix, c.Suffix, c.Test, at))
	}

	// Unresolved outputs would look like duplicates of each other.
	if err := errors.Join(errs...); err != nil {
		return err
	}
	return checkDuplicates(c.Targets)
}

func resolveTarget(t *Target, prefix, suffix string, test bool, at string) error {
	var errs []error

	pkg, err := cleanPath(t.Package)
	if err != nil {
		errs = append(errs, fmt.Errorf("%s.package: %w", at, err))
	}
	t.Package = pkg

	switch {
	case t.Interface == "":
		errs = append(errs, fmt.Errorf("%s.interface: required", at))
	case !token.IsIdentifier(t.Interface):
		errs = append(errs, fmt.Errorf("%s.interface: %q is not a valid Go identifier", at, t.Interface))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	if len(t.Outputs) == 0 {
		t.Outputs = []Output{{}}
	}
	for i := range t.Outputs {
		at := fmt.Sprintf("%s.output[%d]", at, i)
		errs = append(errs, resolveOutput(&t.Outputs[i], t, prefix, suffix, test, at))
	}
	return errors.Join(errs...)
}

func resolveOutput(o *Output, t *Target, prefix, suffix string, test bool, at string) error {
	var errs []error

	if o.Test == nil {
		o.Test = &test
	}

	if o.Package == "" {
		o.Package = t.Package
	} else {
		p, err := cleanPath(o.Package)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.package: %w", at, err))
		}
		o.Package = p
	}

	if o.Provider != "" {
		p, err := cleanPath(o.Provider)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.provider: %w", at, err))
		}
		o.Provider = p
	}

	if o.Name == "" {
		o.Name = defaultName(prefix, t.Interface, suffix)
	}
	switch {
	case !token.IsIdentifier(o.Name):
		errs = append(errs, fmt.Errorf("%s.name: %q is not a valid Go identifier", at, o.Name))
	case o.Name == t.Interface && o.Package == t.Package:
		errs = append(errs, fmt.Errorf("%s.name: %q collides with the interface; set a prefix, a suffix or a name", at, o.Name))
	}

	if o.Filename == "" {
		o.Filename = strings.ToLower(t.Interface) + "_gen.go"
	}
	switch {
	case strings.ContainsAny(o.Filename, `/\`):
		errs = append(errs, fmt.Errorf("%s.filename: %q must be a file name, not a path", at, o.Filename))
	case !strings.HasSuffix(o.Filename, ".go"), strings.HasSuffix(o.Filename, "_test.go"):
		errs = append(errs, fmt.Errorf("%s.filename: %q must end in .go and not _test.go", at, o.Filename))
	}

	return errors.Join(errs...)
}

func checkDuplicates(targets []Target) error {
	type key struct{ pkg, name string }
	funcs := map[key]string{}
	files := map[key]string{}
	var errs []error

	for i, t := range targets {
		for j, o := range t.Outputs {
			at := fmt.Sprintf("targets[%d].output[%d]", i, j)

			names, filenames := []string{o.Name}, []string{o.Filename}
			if *o.Test {
				names = append(names, DoubleName(o.Name), DoubleFields(o.Name))
				filenames = append(filenames, DoubleFilename(o.Filename))
			}

			for _, name := range names {
				if prev, ok := funcs[key{o.Package, name}]; ok {
					errs = append(errs, fmt.Errorf("%s.name: %q already declared by %s", at, name, prev))
				}
				funcs[key{o.Package, name}] = at
			}
			for _, filename := range filenames {
				if prev, ok := files[key{o.Package, filename}]; ok {
					errs = append(errs, fmt.Errorf("%s.filename: %q already written by %s", at, filename, prev))
				}
				files[key{o.Package, filename}] = at
			}
		}
	}
	return errors.Join(errs...)
}

// DoubleName is the test constructor generated for an output named name.
func DoubleName(name string) string {
	return name + "Test"
}

// DoubleFields is the struct that carries the fakes into the test constructor.
func DoubleFields(name string) string {
	return name + "TestFields"
}

// DoubleFilename is the file holding the test double: service_gen.go gives service_test_gen.go.
func DoubleFilename(filename string) string {
	stem := strings.TrimSuffix(strings.TrimSuffix(filename, ".go"), "_gen")
	return stem + "_test_gen.go"
}

// defaultName is prefix + interface + suffix, and stays unexported for an unexported interface: redis gives newRedis.
func defaultName(prefix, iface, suffix string) string {
	if prefix == "" {
		return iface + suffix
	}
	if r, _ := utf8.DecodeRuneInString(iface); !unicode.IsUpper(r) {
		prefix = mapFirst(prefix, unicode.ToLower)
	}
	return prefix + mapFirst(iface, unicode.ToUpper) + suffix
}

func mapFirst(s string, f func(rune) rune) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(f(r)) + s[n:]
}

func cleanPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("required")
	}
	if path.IsAbs(p) {
		return "", fmt.Errorf("%q must be relative to the module root", p)
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%q must stay inside the module", p)
	}
	return clean, nil
}
