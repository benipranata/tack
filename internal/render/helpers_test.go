package render

import (
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benipranata/goldie/pkg/goldie"

	"github.com/benipranata/tack/internal/load"
)

const testModule = "example.com/app"

// named returns the struct type name declared in the package called pkg at importPath.
func named(importPath, pkg, name string) *types.Named {
	obj := types.NewTypeName(token.NoPos, types.NewPackage(importPath, pkg), name, nil)
	return types.NewNamed(obj, types.NewStruct(nil, nil), nil)
}

func sqlDB() types.Type {
	return types.NewPointer(named("database/sql", "sql", "DB"))
}

func slogLogger() types.Type {
	return types.NewPointer(named("log/slog", "slog", "Logger"))
}

// pkgAt describes the package in dir of the test module; its name is the last element of dir.
func pkgAt(dir string) load.Package {
	return load.Package{Dir: dir, Path: path.Join(testModule, dir), Name: path.Base(dir)}
}

func provider(dir, name string) load.Provider {
	return load.Provider{Package: pkgAt(dir), Name: name}
}

// testdata is absolute so that a test which changes the working directory still finds it.
var testdata = func() string {
	dir, err := filepath.Abs("testdata")
	if err != nil {
		panic(err)
	}
	return dir
}()

// goldenEqual compares got with the golden file of the running subtest. Each top-level test keeps its files in its
// own folder, because goldie's -clean removes every golden file of its fixture folder: with one shared folder,
// the go:generate directive of a single test would delete the files of all the others.
func goldenEqual(t *testing.T, got []byte) {
	t.Helper()
	top, name, found := strings.Cut(t.Name(), "/")
	if !found {
		name = top
	}
	goldie.New(t, goldie.WithFixtureDir(filepath.Join(testdata, top))).Equal(name, got)
}
