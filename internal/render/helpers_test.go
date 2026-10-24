package render

import (
	"go/token"
	"go/types"
	"path"

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
