## Context

The repository is currently just `go.mod` (`github.com/benipranata/tack`, go 1.26.5) plus a design
record under `specs/`: `specs/idea/initial-idea.md` (original brief), `specs/idea/grill-tack-wiring-tool.md`
(main design, now reconciled), `specs/grill-tack-provider-scoping.md` and
`specs/grill-tack-wiring-reconciliation.md` (follow-up decisions), and a hand-written reference
fixture at `specs/idea/case-01/` that already builds clean and matches the final schema. See
`proposal.md` for motivation. This design covers how to turn that settled behavior contract (see
`specs/wiring-generation/spec.md` and `specs/cli/spec.md` in this change) into the actual tool.

## Goals / Non-Goals

**Goals:**
- Implement the scanner, resolver, and emitter described in `specs/wiring-generation/spec.md`, and
  the three-command CLI in `specs/cli/spec.md`.
- Make `case-01` an executable golden-file test, not just a hand-verified reference.
- Ship a first installable release via `go install` and a Homebrew tap.

**Non-Goals:**
- No dependency graph, topological sort, or cycle detection — the flat, context-only provider model
  is the entire point (see proposal's Why).
- No per-type `localScan` granularity — the interface-level `localScan: false` in the spec is the
  only opt-out; finer control is not being built.
- No provider fallback lists or assignability-based matching — resolution is exact-type, single
  provider per scope, by design.
- No `--version` flag or positional package filters — the CLI surface is exactly what's specified.

## Decisions

- **Loading & type identity**: use `golang.org/x/tools/go/packages` to load each configured package
  (`packages.NeedTypes | packages.NeedSyntax | ...`), and `go/types.Identical` to key providers and
  resolve accessor return types. This is what makes exact-type matching (no assignability) a one-line
  predicate rather than custom type-comparison logic, and it's the standard tool for this — same
  approach google/wire itself uses for loading.
- **Provider indexing**: two structures — one `map[types.Type]providerFunc` for the global scope
  (built once from every package in `providers:`), and one such map per interface's local directory,
  built lazily per target package. Ambiguity is detected at map-build time (a second qualifying
  function for a type already in the map), not at lookup time, so the error can name both candidates
  immediately.
- **Stale-output deletion ordering**: delete every configured output file for a target package
  *before* calling `packages.Load` on it, per the grill log's "Bootstrap" decision — Go type-checks
  the whole package, so a previously generated file with a since-renamed provider reference would
  otherwise break the very load that's supposed to detect and report that.
- **Code emission**: `text/template` for the fixed per-interface structure (constructor, struct,
  accessors, test helper), followed by `golang.org/x/tools/imports` (goimports) to resolve and format
  the import block. Considered `go/ast` + `go/printer` construction instead — more robust against
  malformed templates, but far more verbose for output this structurally regular; considered
  `dave/jennifer` — a cleaner AST-builder API, but an extra dependency for a case `text/template`
  handles fine once goimports cleans up imports. Import correctness (qualified vs. unqualified local
  calls, aliasing) is exactly what goimports is for, so hand-rolling that logic isn't worth it.
- **Identifier allocator**: a single seeded set per generated file — pre-loaded with `ctx`, `cleanup`,
  `cleanups`, `err`, every import alias, Go keywords, and predeclared identifiers — handing out names
  and appending a numeric counter on collision, per the grill log's "Identifier hygiene" decision.
  Shared by local/field-name allocation and import-alias allocation so one can't shadow the other.
- **CLI implementation**: stdlib `flag` + a manual subcommand switch (`init` vs. everything else
  falling through to generate), not a CLI framework like cobra. Three entry points and one flag don't
  justify the dependency, and it keeps the "smaller than wire" footprint the design commits to.
- **Config discovery**: walk up from `os.Getwd()` looking for `tack.yaml` at each level, matching the
  spec; `--config` short-circuits discovery entirely.
- **Golden-file testing of tack itself**: a Go test copies `specs/idea/case-01` to a temp directory,
  runs the generator against it in-process (as a library call, not `exec.Command`, so failures surface
  as normal Go test failures with real stack traces), and asserts the regenerated
  `app_iface_gen.go` byte-for-byte matches the checked-in one, then `go build ./...` the temp copy.
  This turns the existing hand-verified fixture into a real regression test with no new fixture to
  design.
- **Error formatting**: plain `fmt.Errorf` with a consistent `tack: ...` prefix for the generator's
  own validation/resolution errors (config errors, ambiguity, unsatisfied dependency, etc.) — no
  custom error type hierarchy, since nothing downstream needs to programmatically distinguish error
  kinds; the CLI just prints and exits non-zero. This is distinct from the *generated* code's own
  error wrapping (`fmt.Errorf("ProvideX: %w", err)`), which is already fully specified.
- **Distribution pipeline**: GoReleaser config building the `cmd/tack` binary for standard
  GOOS/GOARCH targets, publishing a formula to `benipranata/homebrew-tap` (a general-purpose tap, not
  tack-specific) alongside a GitHub release, matching the proposal's distribution goal.

## Risks / Trade-offs

- [Loading a large module's packages could be slow] → mitigation: `packages.Load` is only ever
  called on the specific configured package paths, never the whole module; cost scales with configured
  surface, not repo size.
- [Deleting output files before a failing run leaves them missing until the next successful run] →
  accepted trade-off, already decided in the grill log: generated output is fully reproducible from
  source + config, so a failed run costs nothing but a re-run. The CLI's non-zero exit on failure
  (per `specs/cli/spec.md`) makes this visible immediately rather than silently.
- [goimports invokes Go's own tooling under the hood and adds a heavier dependency than manual import
  management] → accepted: correctness of a per-file import block (mixing local unqualified calls,
  aliased provider packages, `testing`, `context`, `fmt`) is exactly its job; hand-rolling it risks
  the aliasing/collision bugs the identifier-allocator decision already goes out of its way to avoid
  elsewhere.
- [Golden-file test is a single fixture — collision-heavy configs and multi-package repos aren't
  covered by it alone] → mitigation: unit tests for the identifier allocator, ambiguity detection, and
  config validation exercise those paths directly with small synthetic fixtures, rather than growing
  `case-01` into something that has to model every edge case at once.
- [First real GoReleaser + Homebrew tap release is unverified until it ships] → mitigation: run
  `goreleaser release --snapshot --clean` locally before tagging the first real version.

## Open Questions

- [ ] Exact scaffold content `tack init` writes (which example package/provider names, if any, vs. a
      minimal annotated skeleton) — deferred to implementation; doesn't affect the spec's two
      scenarios (creates if absent, doesn't clobber) or any other decision here.
