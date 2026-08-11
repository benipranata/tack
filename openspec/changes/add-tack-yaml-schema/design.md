## Context

`internal/config` parses `tack.yaml` by hand-walking `yaml.Node`s (no struct tags, no reflection-based
decoding) specifically so it can reject removed/unknown keys with pointed error messages (see
`parsePackages`/`parseInterfaceConfig` in `internal/config/config.go`). That means there's no
automatic way to derive a JSON Schema from the Go types — it has to be hand-authored and kept in sync
deliberately. See `proposal.md` for why this is worth doing now (IDE autocompletion, a canonical
reference) and the delivery/versioning/sync decisions already made.

## Goals / Non-Goals

**Goals:**
- A JSON Schema that accepts exactly what `internal/config` accepts and rejects the same
  known-invalid shapes (unknown keys, the removed nested `providers:` key).
- A test that fails loudly when the schema and the parser disagree, without requiring the schema to
  be regenerated from Go source.
- `tack init` output that's autocomplete-ready with no action from the user.

**Non-Goals:**
- Runtime schema validation inside the `tack` binary — `internal/config` remains the sole runtime
  validator; the schema is an IDE/documentation aid only, with the sync test as its only automated
  guard.
- SchemaStore catalog submission — out of scope for this change (see proposal's delivery decision);
  may be a later follow-up once the schema has stabilized.
- Modeling cross-field constraints beyond structural shape (e.g. "if `output` is set it must end in
  `.go`") — not something `internal/config` enforces today, so the schema doesn't invent stricter
  rules than the parser has.

## Decisions

**Schema location and format**: `tack.schema.json` at the repository root (not a `schemas/`
subdirectory), matching this project's flat, single-purpose layout (`go.mod`, `Makefile`, `README.md`
all live at root). JSON Schema files are conventionally `.json` even when the document they describe
is YAML — this is the universal convention (SchemaStore, redhat-yaml, every major tool) — so
`tack.schema.json`, not `tack.schema.yaml`, even though the tool it describes reads YAML.

**Delivery mechanism**: a `# yaml-language-server: $schema=<url>` comment in the `tack init`-scaffolded
config, pointing at a raw GitHub URL on `main`
(`https://raw.githubusercontent.com/benipranata/tack/main/tack.schema.json`). Rejected: SchemaStore
catalog submission, because it requires an external PR/review process outside this repo's control and
claims a generic filename (`tack.yaml`) in a shared global namespace — worth revisiting later, but not
blocking this change.

**Versioning**: the URL always points at `main`, not a release-pinned tag. Rejected: pinning to a
release tag, because that requires `cmd/tack` to know its own version at scaffold time (via
`-ldflags`), which isn't wired up anywhere in this codebase today and would be new scope beyond a
docs/IDE-aid change. Accepted trade-off: a user's installed `tack` binary and the schema their editor
fetches can drift out of version-lockstep; acceptable because the schema only ever describes
*structural* shape, and that shape has changed rarely (once, historically — the nested `providers:`
key removal already reflected in today's `internal/config`).

**Sync-test approach**: hand-author the schema (no generation step), and add a Go test that loads
`tack.schema.json` with a JSON Schema validator library and runs it against:
- the golden `openspec/initial-idea/tack.yaml` (must validate)
- the same known-invalid YAML snippets already exercised in
  `internal/config/config_test.go`'s `TestLoad_PackageLevelProvidersKeyRejected`,
  `TestLoad_InterfaceLevelProvidersKeyRejected`, and `TestLoad_UnknownTopLevelKeyRejected` (must fail
  validation)

This reuses existing fixtures/cases rather than inventing new ones, keeping the two validators
(`internal/config`'s parser and the schema) anchored to the same examples. The new test needs a JSON
Schema validation library as a Go dependency (e.g. `github.com/santhosh-tekuri/jsonschema`) — this is
a test-only dependency, not linked into the `tack` binary.

**Where the sync test lives**: a new file colocated with the schema (e.g. `tack_schema_test.go` at
repo root, in `package tack_test` or similar) rather than inside `internal/config`, since it tests an
artifact (`tack.schema.json`) that isn't part of `internal/config`'s own package and shouldn't pull a
schema-validation dependency into that package's test binary.

## Risks / Trade-offs

- **Schema/parser drift between releases** → mitigated by the sync test, but the sync test only runs
  in this repo's CI, not in a consumer's repo — the risk is caught at development time here, not at
  the moment a user's `tack.yaml` disagrees with an already-shipped schema.
- **`main`-tracking URL can serve a schema ahead of a user's installed binary** → accepted trade-off
  documented in `proposal.md`; revisit if this becomes a real source of confusion (e.g. by adding
  version pinning later).
- **New Go dependency for the sync test** → scoped to a test-only import; if the added dependency ever
  looks heavier than justified, the sync test could instead shell out to an existing `ajv`-style CLI
  validator, but that trades a Go dependency for a Node/toolchain one, which fits this repo's
  toolchain (stdlib `flag`, no cobra, minimal deps) worse.
