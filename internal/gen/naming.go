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

// OutputFilename returns the configured output filename for an interface
// entry, or the default derivation (lower(name)_lower(interface)_gen.go)
// when none is configured.
func OutputFilename(icfg config.InterfaceConfig, ifaceName string) string {
	if icfg.Output != "" {
		return icfg.Output
	}
	return strings.ToLower(icfg.Name) + "_" + strings.ToLower(ifaceName) + "_gen.go"
}

// DeleteStaleOutputs removes every configured output file for every
// configured package in cfg. It must run before the packages that are
// generation targets are loaded/scanned, so scanning always reflects
// current source rather than a previous run's output (e.g. one that
// references a since-renamed provider).
func DeleteStaleOutputs(cfg *config.Config) error {
	for pkgDir, ifaces := range cfg.Packages {
		for ifaceName, icfg := range ifaces {
			filename := OutputFilename(icfg, ifaceName)
			outPath := filepath.Join(cfg.ModuleRoot, pkgDir, filename)
			if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("tack: remove stale output %s: %w", outPath, err)
			}
		}
	}
	return nil
}
