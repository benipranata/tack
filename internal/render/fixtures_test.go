package render

import (
	"go/types"

	"github.com/benipranata/tack/internal/load"
)

// outputCase is a bound output together with a name for the test table.
type outputCase struct {
	name string
	out  load.Output
}

func outputCases() []outputCase {
	db := provider("internal/providers", "ProvideDB")
	logger := provider("internal/providers", "ProvideLogger")
	storeA := named(testModule+"/internal/a/store", "store", "Store")
	storeB := named(testModule+"/internal/b/store", "store", "Store")
	rootPkg := load.Package{Dir: ".", Path: testModule, Name: "app"}
	config := named(testModule+"/internal/app", "app", "Config")

	return []outputCase{
		{name: "quickstart", out: load.Output{
			Name: "NewService", Filename: "service_gen.go", Interface: "Service",
			Package: pkgAt("internal/app"), Target: pkgAt("internal/app"),
			Methods: []load.Method{
				{Name: "DB", Type: sqlDB(), Provider: db},
				{Name: "Logger", Type: slogLogger(), Provider: logger},
			},
		}},
		{name: "test double", out: load.Output{
			Name: "NewService", Filename: "service_gen.go", Interface: "Service", Test: true,
			Package: pkgAt("internal/app"), Target: pkgAt("internal/app"),
			Methods: []load.Method{
				{Name: "DB", Type: sqlDB(), Provider: db},
				{Name: "Logger", Type: slogLogger(), Provider: logger},
				{Name: "Hook", Type: types.NewSignature(nil, nil, nil, false), Provider: provider("internal/providers", "ProvideHook")},
				{Name: "Retries", Type: types.Typ[types.Int], Provider: provider("internal/providers", "ProvideRetries")},
				{Name: "Config", Type: config, Provider: provider("internal/app", "provideConfig")},
			},
		}},
		{name: "other package", out: load.Output{
			Name: "NewProdState", Filename: "state_gen.go", Interface: "State", Test: true,
			Package: pkgAt("internal/state/prod"), Target: pkgAt("internal/state"),
			Methods: []load.Method{
				{Name: "DB", Type: sqlDB(), Provider: provider("internal/providers/prod", "ProvideDB")},
				{Name: "Logger", Type: slogLogger(), Provider: logger},
			},
		}},
		{name: "unexported interface", out: load.Output{
			Name: "newRedis", Filename: "redis_gen.go", Interface: "redis", Test: true,
			Package: pkgAt("internal/cache"), Target: pkgAt("internal/cache"),
			Methods: []load.Method{{Name: "Logger", Type: slogLogger(), Provider: logger}},
		}},
		{name: "no methods", out: load.Output{
			Name: "NewEmpty", Filename: "empty_gen.go", Interface: "Empty", Test: true,
			Package: pkgAt("internal/app"), Target: pkgAt("internal/app"),
		}},
		{name: "shared provider", out: load.Output{
			Name: "NewService", Filename: "service_gen.go", Interface: "Service", Test: true,
			Package: pkgAt("internal/app"), Target: pkgAt("internal/app"),
			Methods: []load.Method{
				{Name: "Primary", Type: sqlDB(), Provider: db},
				{Name: "Replica", Type: sqlDB(), Provider: db},
			},
		}},
		{name: "colliding names", out: load.Output{
			Name: "NewService", Filename: "service_gen.go", Interface: "Service", Test: true,
			Package: pkgAt("internal/app"), Target: pkgAt("internal/app"),
			Methods: []load.Method{
				{Name: "HTTPClient", Type: types.NewPointer(named("net/http", "http", "Client")), Provider: provider("internal/a", "ProvideClient")},
				{Name: "Len", Type: types.Typ[types.Int], Provider: provider("internal/a", "ProvideLen")},
				{Name: "Type", Type: types.Typ[types.String], Provider: provider("internal/a", "ProvideType")},
				{Name: "Err", Type: types.Universe.Lookup("error").Type(), Provider: provider("internal/a", "ProvideErr")},
				{Name: "Cleanup", Type: types.Typ[types.Bool], Provider: provider("internal/a", "ProvideCleanup")},
			},
		}},
		{name: "same package name", out: load.Output{
			Name: "NewService", Filename: "service_gen.go", Interface: "Service", Test: true,
			Package: rootPkg, Target: rootPkg,
			Methods: []load.Method{
				{Name: "A", Type: types.NewPointer(storeA), Provider: provider("internal/a/store", "Provide")},
				{Name: "B", Type: types.NewPointer(storeB), Provider: provider("internal/b/store", "Provide")},
			},
		}},
	}
}
