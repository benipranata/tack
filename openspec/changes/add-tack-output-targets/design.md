## Context

Today `internal/config.Config.Packages` is `map[pkgDir]map[ifaceName]InterfaceConfig` — one directory
is simultaneously where an interface is declared, where its local provider scope is scanned
(`internal/resolve.ResolveAll` builds one local `scan.Index` per `pkgDir`, shared by every interface
configured there), and where `gen.Generate` writes the output file (`internal/gen/template.go:98,178`
hardcodes `PackageName: iface.Pkg.Types.Name()`, the same package the interface itself lives in, and
never imports it). See `proposal.md` for why that coupling blocks both a differentiated output package
and multiple implementations of one interface.

## Goals / Non-Goals

**Goals:**
- Replace `packages:` with `targets:` end to end: config parsing, resolution, code emission, stale-file
  cleanup, the CLI's starter config, and `tack.schema.json`.
- Make local provider scanning follow each output variant's own effective directory, independently of
  its sibling variants and of the interface's declaring directory.
- Support writing generated code into a package other than the interface's own, including one that
  doesn't exist on disk yet.

**Non-Goals:**
- No change to the two-scope resolution model itself (global list + one local scan) — an output
  variant still gets exactly one local scope, never a list or a merge of several directories. This
  change relocates *which* directory that is, it doesn't add scope-composition machinery.
- No change to how a single-variant target (no `output.package`, one `output` entry) behaves or reads
  in `tack.yaml` beyond the `packages:` → `targets:` rename — the golden fixture's generated output
  stays byte-identical.
- No automatic migration tooling for existing `tack.yaml` files (e.g. no `tack migrate` command) — the
  breaking rename fails loudly with a clear error, per proposal.md's decision to skip a shim.

## Decisions

**Config shape**: `Config.Targets []TargetConfig`, where `TargetConfig{Package, Interface string,
Output []OutputConfig}` and `OutputConfig{Name, Package, File string, LocalScan bool}`. This mirrors
the nesting the user confirmed (group by `package`+`interface` once, list variants under `output`)
rather than a fully flat list repeating `package`/`interface` per variant — less repetition, and it
keeps "these are all the same interface" structurally visible instead of inferred by matching fields
across separate entries.

**Effective directory**: introduce one small helper, `OutputConfig.EffectiveDir(target TargetConfig)
string`, returning `Package` if set else `target.Package`. Every place that today reads `iface.PkgDir`
for local-scan, stale-deletion, output-writing, or filename derivation switches to this per-variant
value instead. This is the single change that both new capabilities (differentiated output package,
multiple implementations) fall out of — everything else in `internal/resolve` and `internal/gen`
already loops over "the thing to resolve/emit"; it just starts looping over output variants instead of
interfaces.

**Resolution loop restructuring**: `resolve.ResolveAll` currently loops `pkgDir → ifaceName → config`
with one local `scan.Index` built per `pkgDir` and shared across every interface configured there. It
becomes: for each target, load and type-check the interface once (parsing/method-order/type-checking is
identical across that target's variants — no reason to repeat it); for each of the target's output
entries, build a local `scan.Index` scoped to that entry's effective directory (built once per distinct
effective directory actually referenced, so two variants that happen to share a directory — e.g. a
target with a single unset-`output.package` entry, today's common case — don't redundantly rebuild the
same index) and resolve independently. `resolve.Interface` gains an `OutputDir`/`OutputConfig` field
(or becomes one struct per output variant, e.g. rename to `Variant`) carrying what `gen.Generate` and
`cmd/tack` need to know where to write.

**Directory scaffolding order**: `packages.Load` fails on a directory with zero `.go` files, so an
output variant whose effective directory doesn't exist yet needs that directory created (empty is
fine — `packages.Load` accepts a directory with no non-test `.go` files as an empty package as long as
the directory itself exists... verify during implementation; if `packages.Load` actually requires at
least one file, the safe order is: create the directory, then treat "no local providers found there" as
the empty-scope case explicitly in `internal/scan` rather than relying on `packages.Load` to succeed on
zero files) before that variant's local scan runs, and before `packages.Load` is called on it at all.
This slots into the same place `gen.DeleteStaleOutputs` already runs before `resolve.ResolveAll` in
`cmd/tack/main.go` — both are pre-scan directory prep, so scaffolding belongs alongside stale-deletion,
not inside `resolve`.

**Package-name derivation for a new directory**: `filepath.Base(effectiveDir)`, matching `goimports`'
own convention for package name vs. import path when there's no existing `.go` file to read a `package`
clause from. No new config field for it, per the user's confirmation.

**Cross-package emission**: `gen.Generate` currently treats the interface's own package as the
generation target's package unconditionally (`targetPkgPath := iface.Pkg.PkgPath`, template's
`{{.IfaceName}}` always unqualified). When `EffectiveDir != target.Package`, the interface's package
becomes just another import via the existing `importSet`/`Allocator` machinery (`internal/gen/imports.go`,
`internal/gen/ident.go`) — the same mechanism already used for cross-package provider calls — and every
template site that emits `{{.IfaceName}}` needs the qualifier applied consistently (constructor return
type, `var _ ... = (*Struct)(nil)`, test constructor return type). This reuses the identifier-allocation
invariant already documented in CLAUDE.md ("one seeded set per generated file... shared across
local-variable and import-alias naming") rather than introducing a second one.

**Collision detection timing**: computed from `Config.Targets` alone (effective directory + filename
per output entry), so it's a config-validation step in `internal/config` (or a small validation pass
`resolve` runs before touching `packages.Load`), not something discovered mid-scan. Consistent with
this project's existing philosophy of failing before any expensive/partial work starts (see
`DeleteStaleOutputs` running before `ResolveAll`, and ambiguity/unsatisfied-dependency being resolve-time
hard errors).

**Golden fixture migration**: `openspec/initial-idea/tack.yaml`'s single `packages: {src/iface: {Iface:
{name: App, output: app_iface_gen.go}}}` becomes `targets: [{package: src/iface, interface: Iface,
output: [{name: App, file: app_iface_gen.go}]}]` — no `output.package`, so `EffectiveDir == src/iface`
and generated output is byte-identical; `cmd/tack/golden_test.go`'s existing byte-for-byte assertion
doesn't need to change, only the fixture's `tack.yaml` and (if the design needs a second fixture case to
exercise differentiated output / multiple variants) a new golden case — left to `tasks.md` to decide
scope on.

## Risks / Trade-offs

- [Restructuring `resolve.ResolveAll`'s loop nesting touches the most load-bearing file in the
  generator] → mitigation: the effective-directory indirection is additive at the type level (new
  field/helper), and the existing per-package `scan.Index` caching pattern is preserved, just keyed by
  effective directory instead of `pkgDir` — same shape, different key.
- [A target with many output variants pointing at distinct new directories means many `packages.Load`
  calls per `tack` run] → accepted, matching the existing non-goal ("cost scales with configured
  surface, not repo size") from the original wiring-generator design; differentiated-output configs are
  opt-in and expected to be smaller in count than a project's total package surface.
- [Silently creating directories on disk is a more surprising side effect than writing a file into an
  existing one] → mitigated by CLI behavior: `tack`/`tack generate` already writes files without
  prompting, and this only creates a directory that a generated file is about to be written into
  immediately after — no directory is created without also being populated in the same run.
- [`packages.Load`'s behavior on a directory with zero `.go` files is untested in this codebase] →
  needs a spike during implementation (see Decisions); if it doesn't return a usable empty package,
  `internal/scan.LoadDirs` needs an explicit early-return for "directory exists but has no source" that
  fabricates the empty case rather than treating it as a load error.

## Fixture scope (resolved)

This change adds a second golden fixture exercising differentiated output end-to-end: a `State`
interface with `Prod`/`Staging` output variants, each writing into its own package, alongside the
existing `openspec/initial-idea` single-variant case. `cmd/tack/golden_test.go` gains a byte-for-byte
assertion for this second case in addition to the existing one. See `tasks.md` for the breakdown.
