package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benipranata/tack/internal/config"
)

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// OutputFilename returns the configured output filename for an output
// variant, or the default derivation (lower(name)_lower(interface)_gen.go)
// when none is configured.
func OutputFilename(ocfg config.OutputConfig, ifaceName string) string {
	return ocfg.Filename(ifaceName)
}

// DeleteStaleOutputs removes every output variant's own configured output
// file, from its own effective directory. It must run before the
// directories that are generation targets are loaded/scanned, so scanning
// always reflects current source rather than a previous run's output (e.g.
// one that references a since-renamed provider). Deleting one variant's
// stale output never affects another variant's output file, even when two
// variants share the same effective directory.
func DeleteStaleOutputs(cfg *config.Config) error {
	for _, t := range cfg.Targets {
		for _, o := range t.Output {
			filename := OutputFilename(o, t.Interface)
			outPath := filepath.Join(cfg.ModuleRoot, o.EffectiveDir(t), filename)
			if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("tack: remove stale output %s: %w", outPath, err)
			}
		}
	}
	return nil
}

// ScaffoldOutputDirs creates every output variant's effective directory that
// doesn't already exist, so a differentiated output.package can be written
// into (and, if it already has provider source, scanned as a local scope)
// even on the very first run. It must run before packages.Load is called on
// any configured directory.
func ScaffoldOutputDirs(cfg *config.Config) error {
	for _, t := range cfg.Targets {
		for _, o := range t.Output {
			full := filepath.Join(cfg.ModuleRoot, o.EffectiveDir(t))
			if err := os.MkdirAll(full, 0o755); err != nil {
				return fmt.Errorf("tack: create output directory %s: %w", full, err)
			}
		}
	}
	return nil
}
