// Package discover locates tack.yaml by searching parent directories up to the module root.
package discover

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const configFile = "tack.yaml"

// Discover returns the path of the nearest tack.yaml at or above dir, stopping at the module root.
func Discover(dir string) (string, error) {
	start, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	dir = start
	for {
		file := filepath.Join(dir, configFile)
		found, err := isFile(file)
		if err != nil {
			return "", err
		}
		if found {
			return file, nil
		}

		atModuleRoot, err := isFile(filepath.Join(dir, "go.mod"))
		if err != nil {
			return "", err
		}
		parent := filepath.Dir(dir)
		if atModuleRoot || parent == dir {
			return "", fmt.Errorf("%s not found in %s or any parent up to the module root", configFile, start)
		}
		dir = parent
	}
}

func isFile(p string) (bool, error) {
	info, err := os.Stat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	}
	return !info.IsDir(), nil
}
