## MODIFIED Requirements

### Requirement: Local provider scope shadows global, per type
Every output variant configured under a `targets` entry has an effective local-scan directory: its own
`output.package` if set, otherwise the target's `package`. The system SHALL scan that directory
(non-nested) for qualifying providers when the variant's `localScan` is enabled (default `true`). A
local provider for a type MUST take precedence over a global provider for that exact type only —
shadowing MUST NOT apply to an entire package or to other types, and MUST NOT extend to any other
output variant's directory or to the target's `package` directory when a variant sets its own
`output.package`.

#### Scenario: Local provider overrides global for its own type only
- **WHEN** an output variant's effective local-scan directory has a qualifying provider for type `T`,
  and the global scope also has a (different) provider for `T`, and a separate type `U` is only
  provided globally
- **THEN** the variant resolves `T` from its own local provider and `U` from the global provider

#### Scenario: Differentiated output variants scan only their own directory
- **WHEN** two output variants of the same target each set a different `output.package`, and each of
  those directories has its own qualifying provider for the same type `T`
- **THEN** each variant resolves `T` from its own directory's provider only — neither variant sees the
  other's provider, and neither consults the target's `package` directory unless one of them also sets
  `output.package` to that same directory

### Requirement: Self-listing a local directory in the global list
The system SHALL allow a directory to appear in both the global `providers:` list and as an output
variant's effective local-scan directory (its own `output.package`, or the target's `package` when
`output.package` is unset) without triggering an error, so its providers can also serve as a global
fallback for other targets or variants.

#### Scenario: No spurious ambiguity from self-listing
- **WHEN** a directory is both an output variant's effective local-scan directory and a member of the
  global `providers:` list
- **THEN** generation for that variant succeeds using local-beats-global precedence as normal, with no
  ambiguity error raised solely from the directory appearing in both places

### Requirement: Output filename defaults from name and interface
When an output entry has no `file:` key, the system SHALL name the generated file
`strings.ToLower(name) + "_" + strings.ToLower(interface) + "_gen.go"` and write it into that output
entry's effective directory (its own `output.package` if set, otherwise the target's `package`).

#### Scenario: Default filename derivation
- **WHEN** an output entry has `name: MyApp`, its target's interface name is `HTTPStore`, and no
  `file:` key
- **THEN** the system writes the generated file as `myapp_httpstore_gen.go` in that output entry's
  effective directory

### Requirement: Stale generated output is removed before scanning
Before loading and scanning an output variant's effective directory, the system SHALL delete that
variant's own configured output file, so scanning always reflects current source rather than a
previous run's output. Deleting one variant's stale output MUST NOT delete or otherwise affect another
variant's output file, even when two variants share the same effective directory.

#### Scenario: Renamed provider invalidates stale output
- **WHEN** a provider function referenced by a previously generated file has been renamed, and
  generation is run again
- **THEN** the system deletes that output variant's previous output file before scanning its effective
  directory, so the scan reflects only current source and reports the dependency as unsatisfied (or
  resolves it to the renamed provider, if reconfigured) rather than failing to load the package at all

## REMOVED Requirements

### Requirement: Interface-level local-scan opt-out
**Reason**: `localScan` no longer applies to a whole interface — it moves to each output variant, since
different variants of the same interface now scan different effective directories and need
independent opt-outs.
**Migration**: Move any `localScan: false` from an interface's old `packages.<dir>.<Interface>` entry
to the corresponding output entry under the new `targets` shape (see "Output-variant local-scan
opt-out" below). A single-variant interface with `localScan: false` migrates to one `output` entry with
the same `localScan: false`.

### Requirement: Config schema rejects removed provider keys
**Reason**: This requirement's scenario was written against the `packages.<dir>.<Interface>` nesting,
which no longer exists at all under the `targets:` shape — `packages:` itself is now a removed
top-level key, so "a `providers:` list nested under a package's interface entry" can no longer occur as
a distinct case.
**Migration**: Superseded by the new "Config schema rejects removed provider keys" requirement below,
covering the removed top-level `packages:` key and the analogous `providers:`-nesting rejections under
`targets:`/output entries.

The system SHALL treat a `tack.yaml` containing a package-level or interface-level `providers:` key,
or any other key not part of the current schema, as a hard validation error naming the offending key
and its location. Nothing MUST be silently ignored.

#### Scenario: Stale interface-level providers key
- **WHEN** `tack.yaml` has a `providers:` list nested under a package's interface entry
- **THEN** validation fails, naming that key and its location, before any scanning or generation runs

## ADDED Requirements

### Requirement: Config schema rejects unrecognized and removed keys
The system SHALL treat a `tack.yaml` containing the removed top-level `packages:` key, a `providers:`
key nested under a `targets` entry, a `providers:` key nested under an output entry, or any other key
not part of the current schema, as a hard validation error naming the offending key and its location.
Nothing MUST be silently ignored.

#### Scenario: Removed top-level packages key
- **WHEN** `tack.yaml` uses the removed top-level `packages:` key
- **THEN** validation fails, naming that key, before any scanning or generation runs

#### Scenario: Stale target-level providers key
- **WHEN** `tack.yaml` has a `providers:` list nested under a `targets` entry
- **THEN** validation fails, naming that key and its location, before any scanning or generation runs

#### Scenario: Stale output-level providers key
- **WHEN** `tack.yaml` has a `providers:` list nested under an output entry
- **THEN** validation fails, naming that key and its location, before any scanning or generation runs

### Requirement: Output-variant local-scan opt-out
An output entry MAY set `localScan: false` (default `true`) to skip consulting its effective
directory's local scope entirely; the system SHALL then resolve every dependency for that variant from
the global scope instead. The setting MUST affect only that output variant's resolution, never a
sibling output variant of the same target.

#### Scenario: Opted-out variant uses global despite a local candidate
- **WHEN** an output variant has `localScan: false` and its effective directory contains a qualifying
  provider for a type it needs
- **THEN** the system resolves that type from the global scope, ignoring the local candidate

#### Scenario: Sibling output variant is unaffected
- **WHEN** two output variants are configured under the same target, one with `localScan: false` and
  one without
- **THEN** the variant without `localScan: false` still resolves its types from its own effective
  directory's local scope as normal

### Requirement: Output variant directory is created on demand
If an output entry's effective directory (its own `output.package`, when set) does not yet exist on
disk, the system SHALL create it before scanning and generation, rather than failing because the
directory is missing. A newly created directory has no existing providers, so it contributes nothing to
that variant's local scope until the user adds provider source files there and regenerates.

#### Scenario: First generation into a new output package
- **WHEN** an output entry's `output.package` names a directory that does not yet exist
- **THEN** the system creates that directory before resolving the target's dependencies, and the
  variant's local scope is empty (contributing no local providers) for that run

### Requirement: Output directory package name is derived from its basename
For a newly created output directory, the system SHALL derive the generated file's `package` clause
from that directory's basename, following the same convention `goimports` assumes for a directory name.
There is no separate configuration key for an output entry's Go package name.

#### Scenario: Package name matches directory basename
- **WHEN** an output entry's `output.package` is `src/state/prod` and that directory does not yet
  exist
- **THEN** the generated file's package clause is `package prod`

### Requirement: Output collision is a hard error
If two output entries — whether under the same `targets` entry or different ones — resolve to the same
effective directory and the same generated filename, the system SHALL fail validation before any
scanning or generation runs, naming both conflicting entries.

#### Scenario: Two variants collide on directory and filename
- **WHEN** two output entries resolve to the same effective directory and the same filename (whether
  through matching explicit `file:` values or matching defaults)
- **THEN** generation fails before scanning, naming both output entries

### Requirement: Cross-package interface emission is qualified
When an output variant's effective directory differs from its target's `package` (the directory where
the interface is declared), the system SHALL emit an import for the interface's own package in the
generated file and refer to the interface by its qualified name (e.g. `state.State`) everywhere the
unqualified interface name would otherwise appear, including the constructor's return type and the
generated implementation's interface-satisfaction assertion. When the variant's effective directory is
the same as the target's `package` (the default, unset-`output.package` case), the interface continues
to be referenced unqualified, matching current behavior.

#### Scenario: Differentiated output imports the interface's package
- **WHEN** an output variant's `output.package` differs from its target's `package`
- **THEN** the generated file imports the target's package and every reference to the interface type
  (constructor return type, `var _ <Interface> = (*<Struct>)(nil)`, test helper return type) is
  qualified with that package's identifier

#### Scenario: Same-package output stays unqualified
- **WHEN** an output entry has no `output.package` (or it matches the target's `package`)
- **THEN** the generated file references the interface unqualified, exactly as it does today
