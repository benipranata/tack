# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working conventions

Never create a git commit or push to any remote in this repository. Staging, committing, and pushing
are handled by the user — leave changes in the working tree for them to review and commit themselves.

## Project state

The `add-tack-wiring-generator` change has been implemented and archived
(`openspec/changes/archive/2026-08-10-add-tack-wiring-generator/`): `cmd/tack` and `internal/{config,
scan, resolve, gen}` all exist and the golden-file test passes. `openspec/specs/wiring-generation/` and
`openspec/specs/cli/` are now the source of truth for behavior. The reference fixture under
`openspec/initial-idea/` remains as the golden-file target for the generator's own test suite.

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
tack's own test suite (`cmd/tack/golden_test.go`) — it builds clean and matches the current schema.

## Design source of truth

Read in this order when working on the generator:
- `openspec/specs/wiring-generation/spec.md` and `openspec/specs/cli/spec.md` — the current behavior
  contract, as OpenSpec requirement/scenario pairs; treat these as authoritative over any narrative
  docs (including this file and README.md) when they disagree
- `openspec/changes/archive/2026-08-10-add-tack-wiring-generator/proposal.md` and `design.md` — why,
  and the concrete technical decisions (library choices, indexing strategy, error formatting, CLI
  shape) with rejected alternatives, kept as historical record of the original implementation
- `openspec/initial-idea/` — a complete worked example (`tack.yaml`, hand-written `src/*`, and the
  expected generated output `src/iface/app_iface_gen.go`); this fixture is the golden-file target for
  tack's own test suite (`cmd/tack/golden_test.go`)

This project uses the OpenSpec workflow (`openspec/config.yaml`, schema `spec-driven`) for planning:
proposals, design, and delta specs live under `openspec/changes/<change-id>/` until archived into
`openspec/specs/`. Use the `openspec-*` skills (propose/update/apply/archive/sync/explore) rather than
editing `openspec/changes/**` or `openspec/specs/**` by hand.

## Architecture

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

Standard Go toolchain from the repo root, or the equivalent `Makefile` targets:
- `go build ./...` / `make build`
- `go test ./...` / `make test`; `go test ./internal/scan/... -run TestName` for a single package or test
- `go vet ./...`
- `make generate-fixture` — regenerates the checked-in golden fixture under `openspec/initial-idea`
  using the current source of `cmd/tack`
- `make check-fixture` — the CI-freshness check for this repo's own checked-in golden fixture:
  regenerates it and fails (`git diff --exit-code`) if that changes anything, catching drift between
  the generator and the checked-in output

`openspec/initial-idea` is its own Go module (its own `go.mod`) and is not part of `go build ./...`
from the repo root; it's exercised via `cmd/tack/golden_test.go`, which copies it to a temp dir, runs
the generator in-process (library call, not `exec.Command`) against its `tack.yaml`, asserts the
regenerated `app_iface_gen.go` matches the checked-in file byte-for-byte, then `go build ./...` the
temp copy. The intended freshness check for consumers of tack (not this repo) is `tack && git diff
--exit-code`.
