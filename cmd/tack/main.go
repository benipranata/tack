package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/benipranata/tack/internal/generate"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("tack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFile := fs.String("config", "", "path to tack.yaml (default: the nearest one above the working directory)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %v\n", fs.Args())
		return 2
	}

	if err := generate.Run(*configFile); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
