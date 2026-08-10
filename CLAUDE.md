# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working conventions

Never create a git commit or push to any remote in this repository. Staging, committing, and pushing
are handled by the user — leave changes in the working tree for them to review and commit themselves.

## Project state

This repo is pre-implementation: it currently contains only `go.mod` (module
`github.com/benipranata/tack`, go 1.26.5), a reference fixture under `openspec/initial-idea/`, and an
OpenSpec change (`openspec/changes/add-tack-wiring-generator/`) describing the tool to be built.
There is no `cmd/tack` or `internal/` code yet — implementing it is the work tracked by that change's
`tasks.md`.

## What tack is

tack is a Go CLI that generates per-request dependency-wiring code from a `tack.yaml` config and a
set of `Provide*` functions, without a dependency-graph solver (no topological sort, no cycle
detection — providers only ever depend on `context.Context`, never on each other). It reads as an
alternative to hand-written wiring boilerplate or a full DI framework (google/wire, luno/weld) when
that graph-solving machinery is overkill.

For a given configured interface, tack emits one generated file containing:
- a constructor `New<Name><Interface>(ctx) (<Interface>, func(), error)` that calls each resolved
  provider once (in method-declaration order), accumulates cleanups, and unwinds them in reverse
  order on any provider error
- a private struct implementing the interface, with one field + accessor method per interface method
- a `<Name>Test<Interface>` struct + `New<Name>Test<Interface>(t testing.TB, s ...) <Interface>` test
  helper that `t.Fatalf`s on any unset (zero-value) field

See `openspec/initial-idea/` for a complete worked example: `tack.yaml`, hand-written source
(`src/a`, `src/b`, `src/c`, `src/d`, `src/provider-01`, `src/provider-02`, `src/iface`), and the
expected generated output (`src/iface/app_iface_gen.go`). This fixture is the golden-file target for
tack's own test suite (see tasks.md §7) — it already builds clean and matches the final schema.

## Design source of truth

Read in this order when working on the generator:
- `openspec/changes/add-tack-wiring-generator/proposal.md` — why, and what's in/out of scope
- `openspec/changes/add-tack-wiring-generator/design.md` — concrete technical decisions (library
  choices, indexing strategy, error formatting, CLI shape) with rejected alternatives
- `openspec/changes/add-tack-wiring-generator/specs/wiring-generation/spec.md` and `specs/cli/spec.md`
  — the behavior contract, as OpenSpec requirement/scenario pairs
- `openspec/changes/add-tack-wiring-generator/tasks.md` — the implementation checklist, in
  dependency order (scaffolding → config → scanning → resolution → emission → CLI → golden tests →
  distribution)

This project uses the OpenSpec workflow (`openspec/config.yaml`, schema `spec-driven`) for planning:
proposals, design, and delta specs live under `openspec/changes/<change-id>/` until archived into
`openspec/specs/`. Use the `openspec-*` skills (propose/update/apply/archive/sync/explore) rather than
editing `openspec/changes/**` by hand.

## Planned architecture (per tasks.md §1)

- `internal/config` — `tack.yaml` parsing/validation (strict decoder, rejects unknown/removed keys
  such as a package- or interface-level `providers:`)
- `internal/scan` — provider discovery via `golang.org/x/tools/go/packages` (load with
  `NeedTypes | NeedSyntax | ...`) and `go/types` (signature qualification, `types.Identical` for
  type-keyed indexing)
- `internal/resolve` — per-type resolution and ambiguity detection
- `internal/gen` — code emission (`text/template` + `golang.org/x/tools/imports`)
- `cmd/tack` — CLI entry point (stdlib `flag`, no cobra)

Key design invariants to preserve when implementing:
- **Provider qualification is signature-exact**: a function counts as a provider for `T` only if its
  signature is exactly `func(context.Context) (T, func(), error)`. The `Provide` name prefix is a
  pre-filter for near-miss error messages, never the qualifier.
- **Two scopes only, per interface**: the global `providers:` package list, and an implicit
  (non-nested) scan of the interface's own package directory, which shadows the global scope
  *per type*, not per package. `localScan: false` opts an interface out of the local scope entirely.
- **Resolution is exact-type**, never assignability-based; ambiguity (two qualifying providers for
  one type in one scope) and unsatisfied dependencies are both hard errors that name the offending
  method/type/candidates — nothing is silently guessed or partially emitted.
- **Stale output is deleted before scanning**: a target package's configured output files are removed
  before `packages.Load` runs on it, so a previously generated file referencing a since-renamed
  provider can't break the load that's supposed to report that rename.
- **Non-nilable accessor types are rejected** (not pointer/interface/map/slice/chan/func), since the
  test helper's zero-value guard depends on nilability.
- **Identifier allocation is centralized**: one seeded set per generated file (Go keywords,
  predeclared identifiers, `ctx`/`cleanup`/`cleanups`/`err`, every import alias) shared across
  local-variable and import-alias naming, suffixing on collision — so a local name can never shadow
  an import alias or vice versa.

## Commands

No build/lint/test tooling exists yet (no source files outside `openspec/initial-idea`, which is its
own module with its own `go.mod`). Once `cmd/tack` exists, the standard Go toolchain applies from the
repo root:
- `go build ./...`
- `go test ./...` / `go test ./internal/scan/... -run TestName` for a single package or test
- `go vet ./...`

The golden-file test (tasks.md §7) copies `openspec/initial-idea` to a temp dir, runs the generator
in-process (library call, not `exec.Command`) against its `tack.yaml`, asserts the regenerated
`app_iface_gen.go` matches the checked-in file byte-for-byte, then `go build ./...` the temp copy.
The intended CI-freshness check for consumers of tack is `tack && git diff --exit-code` (to be
scripted per tasks.md §7.3).
