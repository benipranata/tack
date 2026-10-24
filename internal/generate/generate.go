// Package generate runs tack: it finds the config, resolves every output and writes the generated files.
package generate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/benipranata/tack/internal/config"
	"github.com/benipranata/tack/internal/discover"
	"github.com/benipranata/tack/internal/load"
	"github.com/benipranata/tack/internal/render"
)

// Run generates every output of the config at configFile, or of the nearest tack.yaml above the
// working directory when configFile is empty. The directory of the config is the module root.
func Run(configFile string) error {
	var err error
	if configFile == "" {
		configFile, err = discover.Discover(".")
	} else {
		configFile, err = filepath.Abs(configFile)
	}
	if err != nil {
		return err
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}
	root := filepath.Dir(configFile)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("the config must be in the module root, next to go.mod: %w", err)
	}
	outs, err := load.Load(root, cfg)
	if err != nil {
		return err
	}

	var files []render.File
	for _, out := range outs {
		rendered, err := render.Render(out)
		if err != nil {
			return err
		}
		files = append(files, rendered...)
	}
	return Write(root, files)
}
