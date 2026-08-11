# tack

**Dead simple dependency wiring.**

tack generates the dependency-wiring code you'd otherwise hand-write — a constructor, a struct, and
a test double — straight from a `tack.yaml` config and the `Provide*` functions you already have.
No graph solver, no topological sort, no cycle detection, no framework to learn. Point it at your
providers, run one command, and get a plain Go file you can read top to bottom.

## Why tack

- **Built for simplicity.** One config file, one command, one generated file per output variant.
  That's the whole surface area.
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

One provider package with two providers, wiring a `Service` interface:

```yaml
# tack.yaml
providers:
  - internal/providers
targets:
  - package: internal/service
    interface: Service
    output:
      - name: Prod
```

```go
// internal/service/service.go
type Service interface {
    Redis() *redis.Client
    Logger() *log.Logger
}
```

```go
// internal/providers/providers.go
func ProvideRedisClient(ctx context.Context) (*redis.Client, func(), error) {
    client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    return client, func() { client.Close() }, nil
}

func ProvideLogger(ctx context.Context) (*log.Logger, func(), error) {
    return log.New(os.Stdout, "", log.LstdFlags), nil, nil
}
```

Running `tack` generates `internal/service/prod_service_gen.go`:

```go
func NewProdService(ctx context.Context) (Service, func(), error) {
    var cleanups []func()
    cleanup := func() {
        for i := len(cleanups) - 1; i >= 0; i-- {
            cleanups[i]()
        }
    }

    redisClient, redisClientCleanup, err := providers.ProvideRedisClient(ctx)
    if err != nil {
        cleanup()
        return nil, nil, fmt.Errorf("ProvideRedisClient: %w", err)
    }
    if redisClientCleanup != nil {
        cleanups = append(cleanups, redisClientCleanup)
    }

    logger, loggerCleanup, err := providers.ProvideLogger(ctx)
    if err != nil {
        cleanup()
        return nil, nil, fmt.Errorf("ProvideLogger: %w", err)
    }
    if loggerCleanup != nil {
        cleanups = append(cleanups, loggerCleanup)
    }

    return &prodService{redis: redisClient, logger: logger}, cleanup, nil
}

type prodService struct {
    redis  *redis.Client
    logger *log.Logger
}

func (s *prodService) Redis() *redis.Client { return s.redis }
func (s *prodService) Logger() *log.Logger  { return s.logger }

func NewProdTestService(t testing.TB, s ...) Service {
    // t.Fatalf's on any unset field — drop in fakes for tests in one call
}
```

Using it is just a constructor call and a deferred cleanup:

```go
svc, closeFn, err := NewProdService(ctx)
if err != nil {
    // handle error
}
defer closeFn()

// use svc
svc.Redis()
svc.Logger()
```

For a complete, buildable worked example — config, hand-written providers, and the exact generated
output — see [`openspec/initial-idea/`](openspec/initial-idea/).

## How it works

Every method on a configured interface names a dependency by its return type. tack resolves exactly
one provider per type from two scopes, in this order:

1. **Local** — the output variant's own effective directory (its `output.package`, or the target's
   `package` when unset), scanned automatically. Created on demand if it doesn't exist yet.
2. **Global** — every package directory listed under top-level `providers:`, used for anything the
   local scope doesn't cover.

A provider is any function with the signature `func(context.Context) (T, func(), error)` — matching
is exact on the type, not on naming convention, so there's never any ambiguity about what counts.

A single interface can have more than one named output variant, each writing to its own directory and
scanning its own local scope — useful for generating multiple implementations of the same interface
(e.g. `ProdState`/`StagingState` for a `State` interface), each pulling from different providers. See
[`openspec/initial-idea/tack.yaml`](openspec/initial-idea/tack.yaml) for a worked example
(`src/state`'s `State` interface, generated as both `src/state/prod` and `src/state/staging`).

## Manual

### CLI

| Command                | What it does                                                                              |
| ---------------------- | ----------------------------------------------------------------------------------------- |
| `tack`                 | Generate (or regenerate) wiring for every interface configured in `tack.yaml`.            |
| `tack generate`        | Alias for the bare `tack` command above.                                                  |
| `tack init`            | Write a starter `tack.yaml` into the current directory. Leaves an existing one untouched. |
| `tack --config <path>` | Use `<path>` directly instead of discovering `tack.yaml` automatically.                   |

Without `--config`, tack walks up from the current directory looking for `tack.yaml`, so you can run
it from any subdirectory of your project.

### Config reference

```yaml
providers: # package directories forming the global provider scope
  - src/provider-01

targets: # interfaces to generate wiring for
  - package: src/iface # the interface's own package directory
    interface: Iface # the interface name
    output: # one or more named implementations to generate
      - name: App # constructor/struct name prefix -> NewAppIface
        # package: src/iface/app  # optional; defaults to the target's own package; created if missing
        # file: app_iface_gen.go  # optional; defaults to lower(name)_lower(interface)_gen.go
        # localScan: true         # optional; set to false to resolve only from the global scope
```

A JSON Schema for `tack.yaml` is published at
[`tack.schema.json`](tack.schema.json). `tack init` scaffolds every starter config with a
`# yaml-language-server: $schema=...` comment pointing at it, so any YAML-aware editor (VS Code +
[redhat.vscode-yaml](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml),
GoLand, Zed, ...) gets autocompletion and inline validation with no further setup.

## Development

```sh
make build             # go build ./...
make test              # go test ./...
make generate-fixture  # regenerate the checked-in golden fixture under openspec/initial-idea
make check-fixture     # verify the generator's checked-in output is still up to date
```

See [`CLAUDE.md`](CLAUDE.md) for architecture details and [`openspec/specs/`](openspec/specs/) for
the full behavior contract.
