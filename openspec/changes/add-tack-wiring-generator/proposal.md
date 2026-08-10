## Why

Wiring per-request dependencies together in Go is either hand-written boilerplate that silently
drifts from its providers, or bought at the cost of a full DI framework (google/wire, luno/weld)
whose graph solver — topological sort, cycle detection, provider sets — is overkill when providers
never depend on each other, only on `context.Context`. tack generates that wiring code straight
from a small `tack.yaml` and a `Provide*` function per dependency, with the graph engine designed
away entirely. The design has been through three grill sessions (`specs/idea/grill-tack-wiring-tool.md`,
`specs/grill-tack-provider-scoping.md`, `specs/grill-tack-wiring-reconciliation.md`) and a working
reference example (`specs/idea/case-01`); this change turns that settled design into the tool itself.

## What Changes

- Introduce `tack`, a Go CLI that reads `tack.yaml` and generates dependency-wiring code for each
  configured interface: a constructor, a private struct, accessor methods, and a test helper struct
  + constructor, all in one generated file per interface.
- Provider resolution: a flat, non-graph model — every provider is `func(context.Context) (T, func(), error)`
  — resolved per dependency type from exactly two scopes: a global `providers:` package list, and an
  implicit scan of the interface's own package directory, which shadows the global scope per-type.
  An interface may opt out of the local scan with `localScan: false`.
- Config validation: a `tack.yaml` with a removed key (package-level or interface-level `providers:`)
  is a hard validation error, not silently ignored.
- Diagnostics: same-scope provider ambiguity, an unsatisfied dependency, a non-nilable dependency
  type, or a non-accessor interface method are all hard errors naming the offending method/type.
- CLI surface: `tack` and `tack generate` both generate (bare form is an alias); `tack init` scaffolds
  an example `tack.yaml`; `--config` selects an explicit config file.
- Distribution: `go install github.com/benipranata/tack@latest`, plus a GoReleaser-published
  `benipranata/homebrew-tap` formula.

## Capabilities

### New Capabilities
- `wiring-generation`: parsing and validating `tack.yaml`, scanning global and local-directory
  provider scopes, resolving exactly one provider per dependency type per interface, and emitting
  the generated wiring file (constructor, struct, accessors, test helper) — erroring loudly on any
  ambiguity, missing provider, invalid method shape, or non-nilable dependency type.
- `cli`: the `tack` / `tack generate` / `tack init` entry points and the `--config` flag that wrap
  `wiring-generation`.

### Modified Capabilities
(none — this is a greenfield project; no existing specs to modify)

## Impact

- New Go source under the module root: config loading, `go/packages`/`go/types`-based scanning,
  provider indexing, code emission, and a `cmd/tack` entry point. The repository currently contains
  only `go.mod` — no existing runtime code is affected.
- `specs/idea/*` (initial idea, refined grill logs, `case-01` reference fixture) remain as the design
  record this change is derived from; `openspec/specs/**` becomes the source of truth for behavior
  going forward.
- Packaging/release impact (GoReleaser config, `homebrew-tap` formula) is implementation detail,
  covered in `design.md` rather than as its own spec capability — it doesn't change observable
  system behavior.
