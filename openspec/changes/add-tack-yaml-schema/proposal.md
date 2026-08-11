## Why

`tack.yaml` has no machine-readable schema, so IDEs can't offer autocompletion, inline docs, or
validation while a user edits it, and there's no canonical reference for the config shape beyond the
README's prose table and `internal/config`'s hand-rolled parser. A published JSON Schema fixes both:
it becomes the reference, and any YAML-aware editor (VS Code + redhat.vscode-yaml, GoLand, Zed, ...)
picks it up via a `$schema` comment with zero other setup.

## What Changes

- Add `tack.schema.json` (JSON Schema, repo root) describing the current `tack.yaml` shape:
  top-level `providers: [string]` and `packages.<dir>.<Interface>.{name (required), output,
  localScan}`, with `additionalProperties: false` at every level so it also flags the removed
  package-/interface-level `providers:` key and any unknown key, matching `internal/config`'s strict
  validation.
- `tack init`'s scaffolded starter config gains a leading
  `# yaml-language-server: $schema=https://raw.githubusercontent.com/benipranata/tack/main/tack.schema.json`
  comment line, so generated configs are IDE-ready out of the box.
- Add a schema-sync test that validates `tack.schema.json` against the golden
  `openspec/initial-idea/tack.yaml` (must pass) and against the known-invalid configs already covered
  by `internal/config/config_test.go` (removed package-/interface-level `providers:` key, unknown
  top-level key — must fail), to catch drift if `internal/config`'s accepted keys change without the
  schema being updated. Requires adding a JSON Schema validation library as a new Go dependency.
- README's "Config reference" section gets a pointer to the schema file/URL.

## Capabilities

### New Capabilities
- `config-schema`: the JSON Schema artifact for `tack.yaml` — its content contract (what it must
  accept/reject relative to `internal/config`) and the sync test that guards against drift.

### Modified Capabilities
- `cli`: the "Init command scaffolds a starter config" requirement gains a scenario for the
  `$schema` comment line now present in the scaffolded output.

## Impact

- New file: `tack.schema.json` (repo root).
- `cmd/tack/main.go`: `starterConfig` constant gains a leading comment line.
- New Go dependency: a JSON Schema validator (e.g. `santhosh-tekuri/jsonschema`) used only by the new
  sync test, not by the `tack` binary itself at runtime.
- `README.md`: "Config reference" section updated with a schema pointer.
- No change to `internal/config`'s runtime validation behavior or to generated wiring output.
