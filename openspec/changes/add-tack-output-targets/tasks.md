## 1. Config parsing (`internal/config`)

- [x] 1.1 Replace `InterfaceConfig` with `OutputConfig{Name, Package, File string, LocalScan bool}` and
      add `TargetConfig{Package, Interface string, Output []OutputConfig}`; replace
      `Config.Packages map[string]map[string]InterfaceConfig` with `Config.Targets []TargetConfig`.
- [x] 1.2 Rewrite `parseFile`/`parsePackages` into `parseTargets`, parsing the top-level `targets:` list
      (each entry: required `package`, required `interface`, required non-empty `output` list) and
      each output entry (required `name`, optional `package`, `file`, `localScan` default `true`).
- [x] 1.3 Make the top-level `packages:` key a hard validation error naming it as removed (same
      treatment as the existing removed-key errors), and add equivalent rejections for a `providers:`
      key nested under a `targets` entry or an output entry.
- [x] 1.4 Add an `OutputConfig.EffectiveDir(target TargetConfig) string` helper (or equivalent) returning
      `Package` if set, else `target.Package`.
- [x] 1.5 Add config-level validation that rejects two output entries (within a target or across
      different targets) resolving to the same `(effective directory, filename)`, naming both entries.
- [x] 1.6 Update `internal/config/config_test.go`: rename/replace the removed package-/interface-level
      `providers:` tests for the new target-/output-level equivalents, add tests for the removed
      top-level `packages:` key, the new `targets:` shape (including multiple output variants), and the
      output-collision validation error.

## 2. Resolution (`internal/resolve`)

- [x] 2.1 Restructure `ResolveAll` to loop per `TargetConfig`: parse/type-check the interface once per
      target (reusing `findInterface`), then loop its `Output` entries, building (and caching by
      effective directory, so entries sharing a directory don't rebuild) one local `scan.Index` per
      distinct effective directory.
- [x] 2.2 Rename/extend the `Interface` result type (or introduce a `Variant` type) to carry the
      resolved output entry's effective directory, filename, and `LocalScan`, alongside the existing
      `PkgDir`/`Pkg`/`IfaceName`/`Methods`, so `internal/gen` and `cmd/tack` know where to write and
      whether emission needs cross-package qualification.
- [x] 2.3 Update `resolveType`'s local-scan branch to key off the output entry's `LocalScan` field
      instead of the removed interface-level setting.
- [x] 2.4 Update `internal/resolve/resolve_test.go` for the new loop shape: local-scan-per-variant,
      opted-out sibling variant unaffected, self-listing a variant's effective directory in the global
      list, and differentiated variants not seeing each other's local providers.

## 3. Directory scaffolding & stale-output cleanup (`internal/gen`, `internal/scan`)

- [x] 3.1 Update `OutputFilename` to take an `OutputConfig` + interface name (unchanged derivation
      logic, updated field names).
- [x] 3.2 Update `DeleteStaleOutputs` to iterate `Config.Targets` → output entries, deleting each
      entry's own configured output file from its effective directory, without needing that directory
      to already contain other entries' files.
- [x] 3.3 Add directory scaffolding: before `packages.Load` runs for a target, create any output
      entry's effective directory that doesn't yet exist. Confirm (spike) whether `packages.Load`
      succeeds on an empty, newly created directory; if not, add an explicit `internal/scan` path that
      treats "directory exists, no `.go` files" as an empty local index rather than a load error.
      (Spike confirmed `packages.Load` returns a package with `Errors: ["no Go files in ..."]`, not a
      usable empty package — implemented instead as a pre-check in `internal/resolve` that skips
      `packages.Load` entirely for a directory with zero `.go` files, treating it as an empty local
      scope; `internal/scan` itself is unchanged.)
- [x] 3.4 Wire scaffolding into `cmd/tack/main.go`'s pre-scan phase, alongside the existing
      `gen.DeleteStaleOutputs` call.

## 4. Code emission (`internal/gen`)

- [x] 4.1 In `Generate`, compute whether the output variant's effective directory differs from the
      target's `package` (the interface's declaring package); when it does, add the interface's package
      to the import set (reusing `importSet`/`Allocator` from `internal/gen/imports.go` and `ident.go`)
      and derive a qualified interface reference.
- [x] 4.2 Update `fileTemplate` (or `fileData`) so every site that emits the interface name
      (constructor return type, `var _ {{.IfaceName}} = (*{{.StructName}})(nil)`, test constructor
      return type) uses the qualified form when cross-package, unqualified otherwise.
- [x] 4.3 Set the generated file's `PackageName` from the output variant's effective directory (basename
      when newly created and not yet loadable, else the loaded package's own name) instead of always
      using `iface.Pkg.Types.Name()`.
- [x] 4.4 Update `internal/gen/gen_test.go` for: same-package emission unchanged (regression), and new
      cross-package emission (qualified interface reference, correct import).

## 5. CLI (`cmd/tack`)

- [x] 5.1 Update `runGenerate`'s output-writing loop to write into each output variant's effective
      directory instead of `iface.PkgDir`.
- [x] 5.2 Update `starterConfig` in `main.go` to the `targets:` shape (single target, single output
      entry, no `output.package` — today's default behavior).

## 6. Schema & docs

- [x] 6.1 Rewrite `tack.schema.json` for `targets:` (list of `{package, interface, output: [...]}`),
      `additionalProperties: false` at every level, matching `internal/config`'s accepted/rejected keys
      per `specs/config-schema/spec.md`.
- [x] 6.2 Update the schema-sync test to validate against the migrated golden `tack.yaml` fixtures and
      the new known-invalid cases (removed `packages:` key, target-/output-level `providers:` keys).
- [x] 6.3 Update README's config reference section for the `targets:` shape.

## 7. Fixtures

- [x] 7.1 Migrate `openspec/initial-idea/tack.yaml` from `packages:` to `targets:` (single target,
      single output entry, no `output.package`); confirm `app_iface_gen.go` stays byte-identical via
      `make generate-fixture` / `make check-fixture`.
- [x] 7.2 Add a second golden fixture exercising differentiated output: a `State` interface with `Prod`
      and `Staging` output variants written into separate packages, each with its own local provider
      (e.g. a variant-specific `ProvideDB`), demonstrating the scaffold-on-demand path for at least one
      variant's directory. (Prod gets a checked-in local provider; Staging's directory has no
      hand-written source at all, so the checked-in `staging_state_gen.go` is itself proof the
      generator scaffolds the directory and falls back to the global scope.)
- [x] 7.3 Wire the new fixture into `cmd/tack/golden_test.go` alongside the existing case: copy to a
      temp dir, run the generator in-process, assert both variants' generated files match byte-for-byte,
      then `go build ./...` the temp copy.

## 8. Validation

- [x] 8.1 `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- [x] 8.2 `make check-fixture` regenerates both golden fixtures with zero drift from what's on disk
      (confirmed via two consecutive regenerations producing an identical result); the Makefile target
      itself naturally shows a diff right now only because this change's fixture edits are still
      uncommitted, not because of generator drift.
- [x] 8.3 `openspec validate --strict` passes for this change.
