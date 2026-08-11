## MODIFIED Requirements

### Requirement: Init command scaffolds a starter config
`tack init` SHALL write an example `tack.yaml` into the current directory when none exists yet there.
If a `tack.yaml` already exists in the current directory, `tack init` MUST NOT overwrite it. The
written starter config's first line SHALL be a `# yaml-language-server: $schema=<url>` comment
pointing at the schema published from this repository's `main` branch (see the `config-schema`
capability), so a freshly scaffolded `tack.yaml` gets editor autocompletion with no further setup.

#### Scenario: No existing config
- **WHEN** a user runs `tack init` in a directory with no `tack.yaml`
- **THEN** the system writes a starter `tack.yaml` there, whose first line is a `yaml-language-server`
  `$schema` comment pointing at the published `tack.schema.json`

#### Scenario: Existing config is not clobbered
- **WHEN** a user runs `tack init` in a directory that already has a `tack.yaml`
- **THEN** the system leaves the existing file untouched and reports that a config already exists,
  instead of overwriting it
