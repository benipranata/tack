# cli Specification

## Purpose

Defines the command-line entry points through which a user invokes wiring generation, scaffolds a
new config, and selects a config file — the surface `wiring-generation` is exercised through.

## Requirements

### Requirement: Generate command and its bare alias
Running `tack` with no subcommand SHALL behave identically to `tack generate`: both MUST generate
wiring for every package configured in the resolved `tack.yaml`.

#### Scenario: Bare invocation generates
- **WHEN** a user runs `tack` in a project with a valid `tack.yaml`
- **THEN** the system generates (or regenerates) the wiring output file for every configured
  interface, identically to running `tack generate`

### Requirement: Generation failure reports and exits non-zero
If generation fails for any configured interface (a hard error from `wiring-generation`, e.g.
ambiguity, an unsatisfied dependency, or an invalid config), the command SHALL print the error and
MUST exit with a non-zero status without leaving a partially written output file for that interface.

#### Scenario: Hard error surfaces and blocks partial output
- **WHEN** a configured interface fails validation or resolution during a `tack` / `tack generate` run
- **THEN** the command exits non-zero, prints the specific error naming the offending method, type,
  or config key, and does not leave a partially written generated file for that interface

### Requirement: Init command scaffolds a starter config
`tack init` SHALL write an example `tack.yaml` into the current directory when none exists yet there.
If a `tack.yaml` already exists in the current directory, `tack init` MUST NOT overwrite it.

#### Scenario: No existing config
- **WHEN** a user runs `tack init` in a directory with no `tack.yaml`
- **THEN** the system writes a starter `tack.yaml` there

#### Scenario: Existing config is not clobbered
- **WHEN** a user runs `tack init` in a directory that already has a `tack.yaml`
- **THEN** the system leaves the existing file untouched and reports that a config already exists,
  instead of overwriting it

### Requirement: Config discovery walks up from the working directory
Without `--config`, the system SHALL locate `tack.yaml` by walking up from the current working
directory toward the filesystem root, so `tack` can be invoked from any subdirectory of the project.

#### Scenario: Invocation from a nested directory
- **WHEN** a user runs `tack` from a subdirectory of a project whose `tack.yaml` lives at the project
  root
- **THEN** the system finds and uses that `tack.yaml`

#### Scenario: No config found
- **WHEN** a user runs `tack` in a directory tree with no `tack.yaml` anywhere from the working
  directory up to the filesystem root, and no `--config` flag is given
- **THEN** the command exits non-zero with an error that no config was found

### Requirement: --config selects an explicit config file
The `--config <path>` flag SHALL make the system use that file directly instead of walking up from
the working directory to discover one.

#### Scenario: Explicit config path
- **WHEN** a user runs `tack --config path/to/other.yaml`
- **THEN** the system loads and generates from `path/to/other.yaml`, without searching for any other
  `tack.yaml`
