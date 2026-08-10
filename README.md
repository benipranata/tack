# tack

**Simple dependency wiring. No bloat, no bullshit, no blunder, no extra features.**

tack generates the dependency-wiring code you'd otherwise hand-write — a constructor, a struct, and
a test double — straight from a `tack.yaml` config and the `Provide*` functions you already have.
No graph solver, no topological sort, no cycle detection, no framework to learn. Point it at your
providers, run one command, and get a plain Go file you can read top to bottom.

## Why tack

- **Built for simplicity.** One config file, one command, one generated file per interface. That's
  the whole surface area.
- **No graph solver.** Providers only ever depend on `context.Context` — never on each other — so
  there's no dependency graph to build, sort, or debug. Just a flat, direct lookup.
- **Plain generated Go.** The output is ordinary, readable Go: a constructor, a struct, and accessor
  methods. Nothing reflection-based, nothing magic, nothing to trace through at runtime.
- **Testable by default.** Every generated interface comes with a matching test helper, so swapping
  in fakes for tests is a one-line call, not hand-rolled boilerplate.
- **Fails loud, never silent.** Ambiguous or missing providers are a hard error naming the exact
  method and type at fault — tack never guesses and never wires the wrong thing quietly.
- **Deterministic.** Same config and source, same output, every time. Diff it, commit it, review it
  like any other code.

## Install

**Go install**

```sh
go install github.com/benipranata/tack@latest
```

**Homebrew**

```sh
brew install --cask benipranata/tap/tack
```

## Quickstart

```sh
tack init       # scaffold a starter tack.yaml in the current directory
tack            # generate wiring for every configured interface
```

That's it — two commands, and your wiring code is generated.

## Example

A one-provider setup: a `Deps` interface with a single `Logger()` method, and a `ProvideLogger`
function to satisfy it.

```yaml
# tack.yaml
providers:
  - internal/logger
packages:
  internal/app:
    Deps:
      name: App
```

```go
// internal/app/deps.go
type Deps interface {
    Logger() *log.Logger
}
```

```go
// internal/logger/logger.go
func ProvideLogger(ctx context.Context) (*log.Logger, func(), error) {
    return log.New(os.Stdout, "", log.LstdFlags), nil, nil
}
```

Running `tack` generates `internal/app/app_deps_gen.go`:

```go
func NewAppDeps(ctx context.Context) (Deps, func(), error) {
    logger, loggerCleanup, err := logger.ProvideLogger(ctx)
    if err != nil {
        return nil, nil, fmt.Errorf("ProvideLogger: %w", err)
    }
    return &appDeps{logger: logger}, func() {
        if loggerCleanup != nil {
            loggerCleanup()
        }
    }, nil
}

type appDeps struct{ logger *log.Logger }

func (d *appDeps) Logger() *log.Logger { return d.logger }

func NewAppTestDeps(t testing.TB, s ...) Deps {
    // t.Fatalf's on any unset field — drop in a fake logger for tests in one call
}
```

For a complete, buildable worked example — config, hand-written providers, and the exact generated
output — see [`openspec/initial-idea/`](openspec/initial-idea/).

## How it works

Every method on a configured interface names a dependency by its return type. tack resolves exactly
one provider per type from two scopes, in this order:

1. **Local** — the interface's own package directory, scanned automatically.
2. **Global** — every package directory listed under top-level `providers:`, used for anything the
   local scope doesn't cover.

A provider is any function with the signature `func(context.Context) (T, func(), error)` — matching
is exact on the type, not on naming convention, so there's never any ambiguity about what counts.

## Manual

### CLI

| Command | What it does |
|---|---|
| `tack` | Generate (or regenerate) wiring for every interface configured in `tack.yaml`. |
| `tack generate` | Alias for the bare `tack` command above. |
| `tack init` | Write a starter `tack.yaml` into the current directory. Leaves an existing one untouched. |
| `tack --config <path>` | Use `<path>` directly instead of discovering `tack.yaml` automatically. |

Without `--config`, tack walks up from the current directory looking for `tack.yaml`, so you can run
it from any subdirectory of your project.

### Config reference

```yaml
providers:            # package directories forming the global provider scope
  - src/provider-01

packages:              # interfaces to generate wiring for
  src/iface:            # the interface's own package directory
    Iface:               # the interface name
      name: App            # constructor/struct name prefix -> NewAppIface
      output: app_iface_gen.go  # optional; defaults to lower(name)_lower(interface)_gen.go
      localScan: true      # optional; set to false to resolve only from the global scope
```

## Development

```sh
make build             # go build ./...
make test              # go test ./...
make generate-fixture  # regenerate the checked-in golden fixture under openspec/initial-idea
make check-fixture     # verify the generator's checked-in output is still up to date
```

See [`CLAUDE.md`](CLAUDE.md) for architecture details and [`openspec/specs/`](openspec/specs/) for
the full behavior contract.
