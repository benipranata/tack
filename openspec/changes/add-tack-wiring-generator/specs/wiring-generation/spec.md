## Purpose

Generates Go dependency-wiring code for a configured interface from `tack.yaml`, resolving each
accessor method's dependency from a flat, non-graph provider model and failing loudly on any
ambiguity, gap, or malformed input rather than emitting partial or guessed wiring.

## ADDED Requirements

### Requirement: Provider signature qualification
The system SHALL treat a function as a provider for type `T` only if its signature is exactly
`func(context.Context) (T, func(), error)`. A `Provide`-prefixed name MUST be treated as a
pre-filter, not a qualifier — the signature alone determines whether a function is used as a provider.

#### Scenario: Exact-signature function is recognized
- **WHEN** a package being scanned contains a function matching `func(context.Context) (T, func(), error)`
- **THEN** the system treats it as a provider for `T`

#### Scenario: Provide-prefixed function with a different signature is ignored
- **WHEN** a scanned package contains a function named `Provide*` whose signature does not match the
  required shape (extra parameters, wrong return count, wrong error position, etc.)
- **THEN** the system does not treat it as a provider, and does not error solely because of its existence

### Requirement: Global provider scope
The system SHALL scan every package listed in `tack.yaml`'s top-level `providers:` list for
qualifying providers, indexed by return type, forming the global scope available to every configured
interface.

#### Scenario: Global provider satisfies a dependency
- **WHEN** an interface's accessor method needs type `T` and no local provider for `T` exists
- **THEN** the system uses the provider for `T` found in the global scope

### Requirement: Local provider scope shadows global, per type
The system SHALL always scan a configured interface's own package directory (non-nested) for
qualifying providers. A local provider for a type MUST take precedence over a global provider for
that exact type only — shadowing MUST NOT apply to an entire package or to other types.

#### Scenario: Local provider overrides global for its own type only
- **WHEN** an interface's own package directory has a qualifying provider for type `T`, and the
  global scope also has a (different) provider for `T`, and a separate type `U` is only provided
  globally
- **THEN** the interface resolves `T` from the local provider and `U` from the global provider

### Requirement: Interface-level local-scan opt-out
An interface MAY set `localScan: false` (default `true`) to skip consulting its directory's local
scope entirely; the system SHALL then resolve every dependency for that interface from the global
scope instead. The setting MUST affect only that interface's resolution, never a sibling interface
configured in the same directory.

#### Scenario: Opted-out interface uses global despite a local candidate
- **WHEN** an interface has `localScan: false` and its own directory contains a qualifying provider
  for a type it needs
- **THEN** the system resolves that type from the global scope, ignoring the local candidate

#### Scenario: Sibling interface is unaffected
- **WHEN** two interfaces are configured from the same package directory, one with `localScan: false`
  and one without
- **THEN** the interface without `localScan: false` still resolves its types from that directory's
  local scope as normal

### Requirement: Self-listing a local directory in the global list
The system SHALL allow a package directory to appear in both the global `providers:` list and as an
interface's own (implicitly local-scanned) directory without triggering an error, so its providers
can also serve as a global fallback for other interfaces.

#### Scenario: No spurious ambiguity from self-listing
- **WHEN** a directory is both an interface's own package and a member of the global `providers:` list
- **THEN** generation for that interface succeeds using local-beats-global precedence as normal, with
  no ambiguity error raised solely from the directory appearing in both places

### Requirement: Same-scope provider ambiguity is a hard error
If a single scope (global, or one directory's local scope) contains more than one qualifying provider
for the same type, the system SHALL fail generation with an error naming every candidate.

#### Scenario: Two global providers for one type
- **WHEN** two different packages in the global `providers:` list each contain a qualifying provider
  returning the same type
- **THEN** generation fails with an error naming both candidate functions and their packages

### Requirement: Unsatisfied dependency is a hard error
If no provider for a required type is found in any scope the interface consults, the system SHALL
fail generation with an error naming the accessor method and the required type. The error MUST also
list any `Provide`-prefixed function returning that type whose signature didn't qualify, so a
near-miss (e.g. a forgotten cleanup return) is visible immediately.

#### Scenario: No provider anywhere
- **WHEN** an interface's accessor method requires a type with no qualifying provider in any scope it
  consults
- **THEN** generation fails, naming the method and the required type

#### Scenario: Near-miss surfaced
- **WHEN** no qualifying provider exists for a required type, but a `Provide`-prefixed function
  returning that type exists with a non-matching signature
- **THEN** the unsatisfied-dependency error also names that near-miss function

### Requirement: Config schema rejects removed provider keys
The system SHALL treat a `tack.yaml` containing a package-level or interface-level `providers:` key,
or any other key not part of the current schema, as a hard validation error naming the offending key
and its location. Nothing MUST be silently ignored.

#### Scenario: Stale interface-level providers key
- **WHEN** `tack.yaml` has a `providers:` list nested under a package's interface entry
- **THEN** validation fails, naming that key and its location, before any scanning or generation runs

### Requirement: Wireable methods are accessors only
Every method on a configured interface MUST take zero parameters and return exactly one result. The
system SHALL treat any other method shape as a hard error naming the offending method.

#### Scenario: Method with parameters or extra returns
- **WHEN** a configured interface declares a method that takes parameters, or returns zero or more
  than one result
- **THEN** generation fails, naming the offending method and its interface

### Requirement: Non-nilable dependency types are rejected
The system SHALL treat an accessor method whose result type is not nilable (not a pointer, interface,
map, slice, channel, or func) as a hard error, since the generated test helper's per-field guard only
compiles for nilable types.

#### Scenario: Value-typed or primitive accessor
- **WHEN** a configured interface has an accessor method returning a non-nilable type (e.g. `int`,
  `string`, or a non-pointer struct)
- **THEN** generation fails, naming the offending method and its type

### Requirement: Generated constructor wires every accessor
For a successfully validated interface, the system SHALL emit a constructor
(`New<Name><Interface>(ctx) (<Interface>, func(), error)`) that calls each resolved provider once, in
method-declaration order, accumulates every non-nil cleanup, and on any provider error runs the
cleanups accumulated so far before returning a wrapped error.

#### Scenario: All providers succeed
- **WHEN** the constructor is called and every resolved provider succeeds
- **THEN** it returns a populated implementation of the interface and an aggregate cleanup function
  that runs every accumulated cleanup in reverse order

#### Scenario: A provider fails partway through
- **WHEN** the constructor is called and a provider after the first returns a non-nil error
- **THEN** the constructor runs the cleanups accumulated from earlier successful providers, then
  returns a nil implementation, a nil cleanup, and an error identifying which provider failed

### Requirement: Generated test helper validates every field
For every generated interface, the system SHALL also emit a `<Name>Test<Interface>` struct with one
exported field per accessor method, and a `New<Name>Test<Interface>(t testing.TB, s <Name>Test<Interface>) <Interface>`
constructor that MUST fail the test immediately if any field is left at its zero value.

#### Scenario: All fields set
- **WHEN** the test constructor is called with every field of the struct populated
- **THEN** it returns an implementation of the interface backed directly by those field values

#### Scenario: A field left unset
- **WHEN** the test constructor is called with one field left at its zero value
- **THEN** it calls `t.Fatalf`, naming the struct and the unset field, before returning

### Requirement: Output filename defaults from name and interface
When an interface entry has no `output:` key, the system SHALL name the generated file
`strings.ToLower(name) + "_" + strings.ToLower(interface) + "_gen.go"` and write it into the
configured package's own directory.

#### Scenario: Default filename derivation
- **WHEN** an interface entry has `name: MyApp`, interface name `HTTPStore`, and no `output:` key
- **THEN** the system writes the generated file as `myapp_httpstore_gen.go` in that package's directory

### Requirement: Stale generated output is removed before scanning
Before loading and scanning a package that is a generation target, the system SHALL delete every
output file already configured for that package, so scanning always reflects current source rather
than a previous run's output.

#### Scenario: Renamed provider invalidates stale output
- **WHEN** a provider function referenced by a previously generated file has been renamed, and
  generation is run again
- **THEN** the system deletes the previous output file(s) for that package before scanning, so the
  scan reflects only current source and reports the dependency as unsatisfied (or resolves it to the
  renamed provider, if reconfigured) rather than failing to load the package at all

### Requirement: Identifier collisions are avoided, never left to fail silently
The system SHALL check every local variable, cleanup variable, and import alias name it introduces
against every identifier already in use in the generated file (including derived names, Go keywords,
and predeclared identifiers) and MUST give it a numeric suffix on collision, so no generated
identifier shadows another.

#### Scenario: Method name collides with a reserved local
- **WHEN** a configured interface has a method whose derived local variable name would collide with
  a name the generator itself needs (e.g. `ctx`, `err`, `cleanup`)
- **THEN** the generated code uses a suffixed variant of that name instead of reusing it, and the
  file still compiles with each identifier referring to the intended value

### Requirement: Generated files carry a DO NOT EDIT header
Every file the system generates MUST begin with the marker comment
`// Code generated by tack. DO NOT EDIT.`.

#### Scenario: Header present on every generated file
- **WHEN** the system successfully generates an output file for any configured interface
- **THEN** the file's first line is `// Code generated by tack. DO NOT EDIT.`
