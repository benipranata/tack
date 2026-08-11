# config-schema Specification

## Purpose

Provides a published JSON Schema for `tack.yaml` so editors can offer autocompletion and validation
while a user edits the config, and so the schema stays honest against what `internal/config` actually
accepts.

## Requirements

### Requirement: Schema describes the current config shape
The repository SHALL include a JSON Schema file, `tack.schema.json` at the repository root,
describing every key `internal/config` accepts: top-level `providers` (a list of strings) and
top-level `packages` (a mapping of package directory to a mapping of interface name to an object with
a required `name` string, an optional `output` string, and an optional `localScan` boolean).

#### Scenario: Valid config validates
- **WHEN** the golden `openspec/initial-idea/tack.yaml` is validated against `tack.schema.json`
- **THEN** validation succeeds

### Requirement: Schema rejects what the config parser rejects
The schema SHALL set `additionalProperties: false` at the top level, at each package's interface
mapping, and at each interface's own object, so keys that `internal/config` rejects — including the
removed package-level and interface-level `providers:` key, and any other unknown key — also fail
schema validation.

#### Scenario: Package-level providers key rejected
- **WHEN** a config with a `providers:` key nested under a package's interface entry (as previously
  supported and later removed) is validated against `tack.schema.json`
- **THEN** validation fails

#### Scenario: Interface-level providers key rejected
- **WHEN** a config with a `providers:` key nested under an interface's own entry is validated against
  `tack.schema.json`
- **THEN** validation fails

#### Scenario: Unknown top-level key rejected
- **WHEN** a config with a key at the top level other than `providers` or `packages` is validated
  against `tack.schema.json`
- **THEN** validation fails

### Requirement: Schema drift is caught by a dedicated test
The repository SHALL include an automated test that validates `tack.schema.json` against the golden
config fixture and against the known-invalid config cases covered by
`internal/config/config_test.go`, so a future change to `internal/config`'s accepted keys that isn't
mirrored in the schema is caught by that test rather than discovered by a user's editor.

#### Scenario: Schema falls out of sync with the parser
- **WHEN** `internal/config` starts accepting or rejecting a key that `tack.schema.json` does not
  agree with
- **THEN** the schema-sync test fails
