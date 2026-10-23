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

func Load(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return cfg, nil
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
		errs = append(errs, resolveTarget(&c.Targets[i], c.Prefix, at))
	}

	// Unresolved outputs would look like duplicates of each other.
	if err := errors.Join(errs...); err != nil {
		return err
	}
	return checkDuplicates(c.Targets)
}

func resolveTarget(t *Target, prefix, at string) error {
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

	if t.Provider != "" {
		p, err := cleanPath(t.Provider)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.provider: %w", at, err))
		}
		t.Provider = p
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	if len(t.Outputs) == 0 {
		t.Outputs = []Output{{}}
	}
	for i := range t.Outputs {
		at := fmt.Sprintf("%s.output[%d]", at, i)
		errs = append(errs, resolveOutput(&t.Outputs[i], t, prefix, at))
	}
	return errors.Join(errs...)
}

func resolveOutput(o *Output, t *Target, prefix, at string) error {
	var errs []error

	if o.Package == "" {
		o.Package = t.Package
	} else {
		p, err := cleanPath(o.Package)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.package: %w", at, err))
		}
		o.Package = p
	}

	if o.Name == "" {
		o.Name = defaultName(prefix, t.Interface)
	}
	switch {
	case !token.IsIdentifier(o.Name):
		errs = append(errs, fmt.Errorf("%s.name: %q is not a valid Go identifier", at, o.Name))
	case o.Name == t.Interface && o.Package == t.Package:
		errs = append(errs, fmt.Errorf("%s.name: %q collides with the interface; set a prefix or a name", at, o.Name))
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

			if prev, ok := funcs[key{o.Package, o.Name}]; ok {
				errs = append(errs, fmt.Errorf("%s.name: %q already declared by %s", at, o.Name, prev))
			}
			funcs[key{o.Package, o.Name}] = at

			if prev, ok := files[key{o.Package, o.Filename}]; ok {
				errs = append(errs, fmt.Errorf("%s.filename: %q already written by %s", at, o.Filename, prev))
			}
			files[key{o.Package, o.Filename}] = at
		}
	}
	return errors.Join(errs...)
}

// defaultName keeps the constructor unexported for an unexported interface: redis gives newRedis.
func defaultName(prefix, iface string) string {
	if prefix == "" {
		return iface
	}
	if r, _ := utf8.DecodeRuneInString(iface); !unicode.IsUpper(r) {
		prefix = mapFirst(prefix, unicode.ToLower)
	}
	return prefix + mapFirst(iface, unicode.ToUpper)
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
