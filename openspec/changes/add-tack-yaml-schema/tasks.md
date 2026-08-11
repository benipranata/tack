## 1. Schema

- [x] 1.1 Author `tack.schema.json` at the repository root: top-level `providers` (array of strings)
      and `packages` (object keyed by package directory, whose values are objects keyed by interface
      name, each with required `name` string, optional `output` string, optional `localScan`
      boolean); `additionalProperties: false` at the top level, the packages-value level, and the
      interface-object level.
- [x] 1.2 Validate the schema by hand against `openspec/initial-idea/tack.yaml` and against the
      invalid-config snippets in `internal/config/config_test.go` before wiring up the automated test.

## 2. Sync test

- [x] 2.1 Add a JSON Schema validation library to `go.mod` (test-only usage).
- [x] 2.2 Add a test that loads `tack.schema.json` and asserts `openspec/initial-idea/tack.yaml`
      validates successfully.
- [x] 2.3 Add test cases mirroring `TestLoad_PackageLevelProvidersKeyRejected`,
      `TestLoad_InterfaceLevelProvidersKeyRejected`, and `TestLoad_UnknownTopLevelKeyRejected` from
      `internal/config/config_test.go`, asserting each fails schema validation.

## 3. CLI

- [x] 3.1 Prepend a `# yaml-language-server: $schema=https://raw.githubusercontent.com/benipranata/tack/main/tack.schema.json`
      comment line to the `starterConfig` constant in `cmd/tack/main.go`.
- [x] 3.2 Update or add a test covering `tack init`'s scaffolded output to assert the new first line.

## 4. Docs

- [x] 4.1 Update README's "Config reference" section to mention `tack.schema.json` and how editors
      pick it up.

## 5. Verification

- [x] 5.1 `make build` and `make test` pass.
- [x] 5.2 `make check-fixture` still passes (schema/init changes don't touch generator output).
