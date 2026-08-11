## Why

`tack.yaml`'s `packages:` map ties three distinct concerns to one directory: where an interface is
declared, where its local provider scope is scanned, and where its generated file is written. That
makes it impossible to generate wiring into a package other than the interface's own, and impossible
to generate more than one differently-wired implementation of the same interface (e.g. a `State`
interface with a `ProdState` and a `StagingState`, each pulling from its own provider set). Both are
real needs once a project wants environment-specific implementations of one interface without hand
duplicating the constructor/struct/test-helper boilerplate tack already generates for the single-target
case.

## What Changes

- **BREAKING**: `packages:` is removed from `tack.yaml` and replaced by a top-level `targets:` list.
  Each entry groups one `package` (module-relative dir where the interface is declared) + `interface`
  (its name) with an `output:` list of one or more named variants. A `tack.yaml` still using `packages:`
  fails validation naming it as a removed key, the same treatment already given the earlier removed
  nested `providers:` key — no dual support, no migration shim.
- Each `output:` entry gets `name` (required, as today), an optional `package` (module-relative dir to
  write the generated file into and scan as that variant's local provider scope; defaults to the
  target's `package` — i.e. today's behavior when omitted), an optional `file` (as today), and its own
  `localScan` (default `true`, moved from the interface level to the output-variant level).
- Local provider scanning is generalized to always follow wherever a variant's output is written: when
  `output.package` differs from the target's `package`, that variant's local scope is its own
  `output.package` directory only — the interface's home directory is not also consulted, and one
  variant's directory is never visible to a sibling variant. Shared providers used by multiple variants
  must live in the global `providers:` list.
- A configured `output.package` directory that does not yet exist is created automatically before
  generation; its generated file's `package` clause is derived from the directory's basename (matching
  `goimports` convention), with no separate package-name field.
- Two output entries (within a target or across different targets) that resolve to the same
  `(output directory, filename)` is a hard config-validation error, caught before any scanning runs.
- Code emission gains a qualified-interface path: when `output.package` differs from the interface's
  own package, the generated file imports the interface's package and refers to it qualified (e.g.
  `state.State`, `var _ state.State = (*prodState)(nil)`) instead of assuming same-package, unqualified
  use.
- `tack.schema.json` and `tack init`'s scaffolded starter config are updated to the `targets:` shape.

## Capabilities

### New Capabilities
(none — this extends the existing `config-schema` and `wiring-generation` capabilities rather than
introducing a new domain)

### Modified Capabilities
- `config-schema`: `tack.schema.json` describes `targets:` (list of `{package, interface, output:
  [{name, package?, file?, localScan?}]}`) instead of `packages:`, and rejects the removed `packages:`
  key the same way it already rejects the removed nested `providers:` key.
- `wiring-generation`: local provider scope resolution, output file location, stale-output deletion,
  and code emission all change to be keyed per output variant (directory it writes into) rather than
  per interface's home package directory; adds directory scaffolding and cross-package qualified
  emission; adds the output-collision hard error.

## Impact

- `internal/config`: schema parsing rewritten for `targets:`; `packages:` key becomes a hard rejection.
- `internal/resolve`: `ResolveAll`/`resolveInterface` restructured to loop per output variant, each
  with its own local index scoped to that variant's output directory; interface parsing/type-checking
  stays shared once per target.
- `internal/gen`: `Generate` gains a qualified-interface emission path (import + alias when
  `output.package != package`); output-directory scaffolding (mkdir + basename-derived package name) on
  first generation for a new variant directory.
- `internal/gen` stale-output deletion: keyed per output-variant directory instead of per target
  package directory.
- `cmd/tack`: output file writing loop keyed per output variant; starter config in `runInit` updated to
  `targets:` shape.
- `tack.schema.json`, README's config reference, and the schema-sync test: updated to `targets:` shape
  and the removed `packages:` key.
- `openspec/initial-idea` golden fixture: `tack.yaml` migrates `packages:` to `targets:` (single
  variant, no `output.package`, so generated output is byte-identical); this remains the golden-file
  target for `cmd/tack/golden_test.go`.
- No published downstream consumer of `packages:` besides `core/identity` is known to this repo; that
  consumer's `tack.yaml` will need the same `packages:` → `targets:` migration when it upgrades.
