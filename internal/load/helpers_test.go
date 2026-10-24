package load

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

const testModule = "example.com/app"

// typecheck type-checks one source file and may import the standard library.
func typecheck(t *testing.T, src string) *types.Package {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	require.NoError(t, err)

	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check(testModule+"/x", fset, []*ast.File{f}, nil)
	require.NoError(t, err)
	return pkg
}

// fakePackage wraps source as a loaded package living in dir of the test module.
func fakePackage(t *testing.T, dir, src string) *packages.Package {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	require.NoError(t, err)

	pkgPath := path.Join(testModule, dir)
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check(pkgPath, fset, []*ast.File{f}, nil)
	require.NoError(t, err)
	return &packages.Package{
		Name:    pkg.Name(),
		PkgPath: pkgPath,
		Types:   pkg,
		Module:  &packages.Module{Path: testModule},
	}
}

// writeModule creates a Go module in a temp dir and returns its root.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module " + testModule + "\n\ngo 1.21\n"
	for name, content := range files {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	return root
}

func typeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string { return p.Name() })
}
