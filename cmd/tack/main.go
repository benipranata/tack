// Command tack generates per-request dependency-wiring code from a
// tack.yaml config and a set of Provide* functions.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/benipranata/tack/internal/config"
	"github.com/benipranata/tack/internal/gen"
	"github.com/benipranata/tack/internal/resolve"
)

const starterConfig = `# yaml-language-server: $schema=https://raw.githubusercontent.com/benipranata/tack/main/tack.schema.json
# tack.yaml
#
# providers: package directories (relative to the go.mod that owns this
# config) scanned for the global provider scope, shared by every configured
# output variant unless shadowed by a local provider of the same type.
providers: []

# targets: interfaces to generate wiring for. Each target names the package
# directory an interface is declared in, plus one or more named output
# variants (e.g. multiple implementations of the same interface). An output
# variant's own directory is scanned as a local provider scope too, unless
# localScan is set to false; it defaults to the target's own package
# directory when not set, and is created automatically if it doesn't exist.
targets: []
#   - package: src/example
#     interface: Example
#     output:
#       - name: App
#         # package: src/example/app   # default: same as the target's package
#         # file: app_example_gen.go   # default: lower(name)_lower(interface)_gen.go
#         # localScan: true            # set to false to resolve only from the global scope
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "init" {
		return runInit(args[1:])
	}

	rest := args
	if len(args) > 0 && args[0] == "generate" {
		rest = args[1:]
	}

	fs := flag.NewFlagSet("tack", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to tack.yaml (skips discovery)")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("tack: unrecognized argument %q", fs.Arg(0))
	}

	return runGenerate(*configPath)
}

func runGenerate(configPath string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("tack: %w", err)
	}

	cfg, err := config.Load(configPath, wd)
	if err != nil {
		return err
	}

	if err := gen.DeleteStaleOutputs(cfg); err != nil {
		return err
	}
	if err := gen.ScaffoldOutputDirs(cfg); err != nil {
		return err
	}

	ifaces, err := resolve.ResolveAll(cfg)
	if err != nil {
		return err
	}

	for _, iface := range ifaces {
		out, err := gen.Generate(iface)
		if err != nil {
			return err
		}
		outPath := filepath.Join(cfg.ModuleRoot, iface.EffectiveDir, gen.OutputFilename(iface.Output, iface.IfaceName))
		if err := os.WriteFile(outPath, out, 0o644); err != nil {
			return fmt.Errorf("tack: write %s: %w", outPath, err)
		}
	}

	return nil
}

func runInit(_ []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("tack: %w", err)
	}

	path := filepath.Join(wd, "tack.yaml")
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("tack: %s already exists; leaving it untouched\n", path)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("tack: %w", err)
	}

	if err := os.WriteFile(path, []byte(starterConfig), 0o644); err != nil {
		return fmt.Errorf("tack: write %s: %w", path, err)
	}
	fmt.Printf("tack: wrote starter config to %s\n", path)
	return nil
}
