# config-schema Specification

## Purpose

Provides a published JSON Schema for `tack.yaml` so editors can offer autocompletion and validation
while a user edits the config, and so the schema stays honest against what `internal/config` actually
accepts.

## Requirements

### Requirement: Schema describes the current config shape
The repository SHALL include a JSON Schema file, `tack.schema.json` at the repository root,
describing every key `internal/config` accepts: top-level `providers` (a list of strings) and
top-level `targets` (a list of objects, each with a required `package` string, a required `interface`
string, and a required `output` list of one or more objects). Each output object SHALL require a
`name` string and MAY include an optional `package` string, an optional `file` string, and an optional
`localScan` boolean.

#### Scenario: Valid config validates
- **WHEN** the golden `openspec/initial-idea/tack.yaml` is validated against `tack.schema.json`
- **THEN** validation succeeds

#### Scenario: Multiple output variants validate
- **WHEN** a `targets` entry's `output` list has more than one entry, each with its own `name` and its
  own `package`
- **THEN** validation succeeds

#### Scenario: Output entry without package validates
- **WHEN** an output entry omits `package`
- **THEN** validation succeeds, since `package` is optional at the output level

### Requirement: Schema rejects unrecognized and removed keys
The schema SHALL set `additionalProperties: false` at the top level, at each `targets` entry's own
object, and at each output entry's own object, so keys that `internal/config` rejects — including the
removed top-level `packages:` key, a `providers:` key nested under a `targets` entry, a `providers:`
key nested under an output entry, and any other unknown key — also fail schema validation.

#### Scenario: Removed top-level packages key rejected
- **WHEN** a config uses the removed `packages:` top-level key instead of `targets:`
- **THEN** validation fails

#### Scenario: Target-level providers key rejected
- **WHEN** a config has a `providers:` key nested under a `targets` entry
- **THEN** validation fails

#### Scenario: Output-level providers key rejected
- **WHEN** a config has a `providers:` key nested under an output entry
- **THEN** validation fails

#### Scenario: Unknown top-level key rejected
- **WHEN** a config with a key at the top level other than `providers` or `targets` is validated
  against `tack.schema.json`
- **THEN** validation fails

### Requirement: Schema drift is caught by a dedicated test
The repository SHALL include an automated test that validates `tack.schema.json` against the golden
config fixture and against the known-invalid config cases covered by `internal/config/config_test.go`
— including the removed top-level `packages:` key and target-/output-level `providers:` keys — so a
future change to `internal/config`'s accepted keys that isn't mirrored in the schema is caught by that
test rather than discovered by a user's editor.

#### Scenario: Schema falls out of sync with the parser
- **WHEN** `internal/config` starts accepting or rejecting a key that `tack.schema.json` does not
  agree with
- **THEN** the schema-sync test fails
