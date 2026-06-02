# Plan: Add --skip-brew-update flag to update command

**Change**: 260531-dx50-skip-brew-update-flag
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Requirements

<!-- migrated from spec.md on 2026-06-02 -->

## Non-Goals

- Skipping `brew info` or `brew upgrade` — the flag gates ONLY the `brew update` tap-metadata refresh. No other step is affected.
- Configuring the flag via `hop.yaml` or environment variables — Constitution III (Convention Over Configuration): this is a per-invocation CLI flag, not persisted state.
- Refactoring `internal/proc` or the subprocess-execution convention — the contract explicitly forbids refactoring. Any test seam stays local to the `update` package.
- Adding a short alias (e.g. `-s`) — the cross-toolkit contract fixes the long form only.

## CLI: `hop update --skip-brew-update`

### Requirement: Flag declaration
The `hop update` subcommand MUST expose a boolean flag named exactly `--skip-brew-update`, registered via cobra's `BoolVar` with default `false`, matching the repo's existing flag-wiring convention (`clone.go`, `sync.go`). The subcommand SHALL retain `cobra.NoArgs`. The flag's help text SHALL state that it skips the internal `brew update` tap-metadata refresh while the version check and upgrade still run.

#### Scenario: Flag present in help
- **GIVEN** the hop binary is built
- **WHEN** a user runs `hop update --help`
- **THEN** the output lists `--skip-brew-update` with descriptive help text
- **AND** the flag's default is `false`

#### Scenario: Flag rejects values (boolean)
- **GIVEN** the hop binary is built
- **WHEN** a user runs `hop update --skip-brew-update`
- **THEN** the flag parses as `true` with no value argument required

### Requirement: Skip semantics
When `--skip-brew-update` is set, `update.Run` MUST skip ONLY the `brew update --quiet` invocation. The `brew info --json=v2` version query, the up-to-date short-circuit (`Already up to date`), and the `brew upgrade <formula>` invocation SHALL all execute exactly as they do without the flag. The `isBrewInstalled()` short-circuit and all wrapper messages (`Current version:`, `Checking for updates...`, `Updating … → …`, `Updated to …`) SHALL be unaffected.

#### Scenario: Flag set — brew update skipped, upgrade runs
- **GIVEN** the binary is a Homebrew install and a newer version is available
- **WHEN** `hop update --skip-brew-update` runs
- **THEN** `brew update` is NOT invoked
- **AND** `brew info --json=v2 sahil87/tap/hop` is invoked (version check)
- **AND** `brew upgrade sahil87/tap/hop` is invoked

#### Scenario: Flag set — already up to date short-circuit preserved
- **GIVEN** the binary is a Homebrew install and the installed version equals brew's reported stable version
- **WHEN** `hop update --skip-brew-update` runs
- **THEN** `brew update` is NOT invoked
- **AND** `brew info` IS invoked
- **AND** `brew upgrade` is NOT invoked (up-to-date short-circuit reached)
- **AND** the `Already up to date (<version>).` wrapper line is printed to `out`

### Requirement: Default behavior preserved exactly
When `--skip-brew-update` is absent (default `false`), `update.Run` MUST reproduce the current behavior byte-for-byte: `brew update --quiet` runs first (gated by `if !skipBrewUpdate`), followed by the version check, short-circuit, and upgrade. No output, ordering, error-handling, or exit-code behavior changes in the default path.

#### Scenario: Default — brew update still runs
- **GIVEN** the binary is a Homebrew install and a newer version is available
- **WHEN** `hop update` runs without the flag
- **THEN** `brew update` IS invoked before the version check
- **AND** `brew upgrade` IS invoked

#### Scenario: Default — brew update failure still surfaces
- **GIVEN** the binary is a Homebrew install
- **WHEN** `hop update` runs without the flag and `brew update` fails (non-`ErrNotFound`)
- **THEN** `Run` returns a `brew update failed: …` wrapped error, exactly as today

### Requirement: Signature threading
`update.Run` MUST accept the skip decision as a `skipBrewUpdate bool` parameter threaded from `cmd/hop/update.go`. The new signature SHALL be `Run(currentVersion string, skipBrewUpdate bool, out, errOut io.Writer) error`. The single existing call site in `cmd/hop/update.go` SHALL pass the flag value. The `proc.ErrNotFound` → `errSilent` mapping in the `RunE` SHALL be preserved.

#### Scenario: Call site threads the flag
- **GIVEN** `cmd/hop/update.go`'s `RunE`
- **WHEN** the command executes
- **THEN** it calls `update.Run(version, skipBrewUpdate, cmd.OutOrStdout(), cmd.ErrOrStderr())`
- **AND** a returned `proc.ErrNotFound` is mapped to `errSilent`

### Requirement: Output routing preserved
The deliberate output split documented in the `Run` doc comment MUST be preserved: wrapper messages go to `out`/`errOut`; subprocess stdout/stderr routing stays owned by `internal/proc`. The guard SHALL introduce no new writes to `out`/`errOut` and SHALL NOT alter how `brew info` / `brew upgrade` streams are routed.

#### Scenario: No new wrapper output from the guard
- **GIVEN** `--skip-brew-update` is set
- **WHEN** `update.Run` skips the refresh
- **THEN** no additional line is written to `out` or `errOut` for the skip itself (the skip is silent; only the existing wrapper lines appear)

## Testing: subprocess-invocation assertion

### Requirement: Test asserts skip omits `brew update` but runs `brew upgrade`
The `update` package MUST include a test (in `src/internal/update/update_test.go`, following the existing repo test pattern: `testing`-only, `bytes.Buffer` writers, no external dependencies) that asserts: with `skipBrewUpdate == true`, the sequence of brew subcommands invoked omits `brew update` but includes `brew upgrade`; and as a baseline, with `skipBrewUpdate == false`, the sequence includes `brew update`. The test SHALL NOT require Homebrew, network access, or a `/Cellar/` install to be present, and SHALL NOT refactor `internal/proc`.

To make brew invocations observable without refactoring the subprocess convention, the `update` package MAY introduce a minimal unexported package-level indirection (e.g. a `var brewRun = func(...) ...` and/or `var brewRunForeground = func(...) ...`) that defaults to the real `internal/proc` calls in production and is overridden with a recording stub inside the test via `t.Cleanup`/`defer` restore. The `isBrewInstalled` short-circuit MAY likewise be made overridable via an unexported package var so the test can exercise the brew code path deterministically. Production behavior MUST remain identical: the defaults point at the real `internal/proc` invocations and the explicit-argument-slice convention (Constitution I) is preserved.

#### Scenario: Skip path recorded
- **GIVEN** the test overrides the brew-invocation indirection with a recorder and forces the brew-install path
- **WHEN** `Run("v0.0.1", true, …)` runs against a recorder that reports a newer available version
- **THEN** the recorded invocations include an entry beginning `brew upgrade`
- **AND** the recorded invocations include NO entry beginning `brew update`

#### Scenario: Default path recorded (regression guard)
- **GIVEN** the same recorder setup
- **WHEN** `Run("v0.0.1", false, …)` runs against a recorder that reports a newer available version
- **THEN** the recorded invocations include an entry beginning `brew update`
- **AND** the recorded invocations include an entry beginning `brew upgrade`

#### Scenario: Existing non-brew test still passes
- **GIVEN** the test process is not a `/Cellar/` install
- **WHEN** the existing `TestRunNonBrewInstall` runs against the new signature
- **THEN** it calls `Run("v0.0.3", false, &stdout, &stderr)` and still asserts the manual-update hint with no error

### Requirement: Build and package tests pass
`go build ./...` and `go test ./internal/update/...` (run from `src/`) MUST pass before the PR is opened.

#### Scenario: Verification gate
- **GIVEN** the implementation is complete
- **WHEN** `go build ./...` and `go test ./internal/update/...` run from `src/`
- **THEN** both succeed with no failures

## Design Decisions

1. **Guard the existing block with `if !skipBrewUpdate` rather than restructuring `Run`**: keeps the default path byte-for-byte identical and the diff minimal.
   - *Why*: Contract demands "Default (absent) = current behavior exactly preserved" and "do NOT refactor". A wrapping conditional is the smallest faithful change.
   - *Rejected*: Extracting the refresh into a helper and conditionally calling it — larger diff, more surface area for behavior drift, unjustified for a one-line gate.

2. **Minimal unexported package-local test seam (function-var indirection) instead of refactoring `internal/proc`**: brew invocations route through a `var` that defaults to the real `proc.Run`/`proc.RunForeground`; tests swap in a recorder.
   - *Why*: No injection seam exists today (hardcoded `"brew"`, `isBrewInstalled` short-circuit), and the contract forbids refactoring `internal/proc`. A package-local var is the least-invasive, fully-reversible way to make "which brew subcommands ran" observable, with production defaults unchanged (Constitution I preserved — args stay explicit slices via `internal/proc`).
   - *Rejected*: (a) Refactoring `internal/proc` to accept an injectable runner — explicitly forbidden by the contract and broader blast radius. (b) An integration test that shells out to a fake `brew` on `PATH` — flaky, environment-dependent, and still blocked by the `isBrewInstalled` short-circuit. (c) No test — violates the contract's explicit test requirement.

## Tasks

<!-- Sequential work items for the apply stage. Checked off [x] as completed. -->

### Phase 1: Core Implementation

<!-- Primary functionality. Order by dependency — earlier tasks are prerequisites for later ones. -->

- [x] T001 Add `skipBrewUpdate bool` parameter to `update.Run` in `src/internal/update/update.go`. New signature: `func Run(currentVersion string, skipBrewUpdate bool, out, errOut io.Writer) error`. Wrap ONLY the existing `brew update --quiet` block (~L63–L72) in `if !skipBrewUpdate { ... }`; leave the isBrewInstalled short-circuit, wrapper lines, brewLatestVersion call, up-to-date short-circuit, and `brew upgrade` foreground call byte-for-byte identical. Update the `Run` doc comment to mention `skipBrewUpdate`. <!-- A-001, A-003, A-004, A-005 -->
- [x] T002 Introduce minimal unexported package-level seam in `src/internal/update/update.go` so brew invocations are observable in tests without refactoring `internal/proc`: `var brewRun = func(ctx, name, args...) ([]byte, error) { return proc.Run(...) }`, `var brewRunForeground = func(ctx, dir, name, args...) (int, error) { return proc.RunForeground(...) }`, and `var brewInstalledCheck = isBrewInstalled`. Route `Run`/`brewLatestVersion` through these vars. Defaults preserve production behavior (Constitution I — explicit arg slices via internal/proc). <!-- A-006 -->

### Phase 2: Integration

<!-- Wire components together. -->

- [x] T003 Wire a real cobra bool flag `--skip-brew-update` (default false) in `src/cmd/hop/update.go` via `cmd.Flags().BoolVar(...)`, matching the clone.go/sync.go convention (local `var skipBrewUpdate bool` inside `newUpdateCmd`, assign `cmd`, register flag, return cmd). Retain `cobra.NoArgs`. Pass the flag into `update.Run(version, skipBrewUpdate, cmd.OutOrStdout(), cmd.ErrOrStderr())`. Preserve the `errors.Is(err, proc.ErrNotFound) -> errSilent` mapping. Help text explains it skips the internal `brew update` tap-metadata refresh while the version check and upgrade still run. <!-- A-002 -->

### Phase 3: Testing

<!-- Subprocess-invocation assertions and signature migration. -->

- [x] T004 In `src/internal/update/update_test.go`, add a test that overrides `brewInstalledCheck` (→ true), `brewRun`, and `brewRunForeground` with a recorder appending normalized command strings ("brew update", "brew info ...", "brew upgrade ..."), with the `brew info --json=v2` stub returning valid JSON reporting a NEWER stable version than the currentVersion. Restore overrides via `t.Cleanup`/defer. Assert: `Run("v0.0.1", true, ...)` records an entry starting "brew upgrade" and NO entry starting "brew update"; `Run("v0.0.1", false, ...)` records both "brew update" and "brew upgrade". <!-- A-007, A-008 -->
- [x] T005 Update `TestRunNonBrewInstall` in `src/internal/update/update_test.go` to the new signature: `Run("v0.0.3", false, &stdout, &stderr)`. Leave its `isBrewInstalled()` skip guard behavior intact so it still passes regardless of environment. <!-- A-009 -->

### Phase 4: Verification

- [x] T006 From `src/`: run `go build ./...`, `go test ./internal/update/...`, and `go vet ./internal/update/... ./cmd/...`. All MUST pass. <!-- A-010 -->

## Acceptance

### Functional Completeness

- [ ] A-001 Skip semantics: When `skipBrewUpdate == true`, `update.Run` skips ONLY `brew update --quiet`; `brew info --json=v2`, the `Already up to date` short-circuit, and `brew upgrade <formula>` all still execute. `isBrewInstalled` short-circuit and wrapper messages unaffected.
- [ ] A-002 Flag declaration: `hop update` exposes a boolean `--skip-brew-update` flag via cobra `BoolVar`, default `false`, with `cobra.NoArgs` retained and descriptive help text stating it skips the internal `brew update` tap-metadata refresh while version check and upgrade still run.
- [ ] A-003 Default behavior preserved: With `skipBrewUpdate == false`, `brew update --quiet` runs first (gated by `if !skipBrewUpdate`), then version check, short-circuit, upgrade — no output, ordering, error-handling, or exit-code change in the default path.
- [ ] A-004 Signature threading: `update.Run` signature is `Run(currentVersion string, skipBrewUpdate bool, out, errOut io.Writer) error`; the single call site in `cmd/hop/update.go` passes the flag value; the `proc.ErrNotFound` → `errSilent` mapping is preserved in `RunE`.
- [ ] A-005 Output routing preserved: The `Run` doc comment's documented split (wrapper messages → `out`/`errOut`; subprocess streams → `internal/proc`) is preserved; the guard introduces no new writes to `out`/`errOut` and does not alter `brew info`/`brew upgrade` stream routing.

### Behavioral Correctness

- [ ] A-006 Test seam is minimal and unexported, defaults to real `proc.Run`/`proc.RunForeground`/`isBrewInstalled`; `internal/proc` is NOT refactored; production behavior is byte-identical (Constitution I explicit-arg-slice convention preserved).

### Scenario Coverage

- [ ] A-007 Skip path test: a test asserts `Run("v0.0.1", true, ...)` records "brew upgrade" and NOT "brew update", against a recorder reporting a newer available version, with no brew/network/Cellar dependency.
- [ ] A-008 Default path regression test: a test asserts `Run("v0.0.1", false, ...)` records both "brew update" and "brew upgrade".
- [ ] A-009 Existing non-brew test: `TestRunNonBrewInstall` calls `Run("v0.0.3", false, &stdout, &stderr)`, retains its isBrewInstalled skip guard, and still passes.

### Edge Cases & Error Handling

- [ ] A-010 Verification gate: `go build ./...`, `go test ./internal/update/...`, and `go vet ./internal/update/... ./cmd/...` (from `src/`) all pass with no failures.

### Code Quality

- [ ] A-011 Pattern consistency: New code follows naming and structural patterns of surrounding code (clone.go/sync.go flag wiring, update.go comment/error-handling style).
- [ ] A-012 No unnecessary duplication: Existing utilities reused where applicable; no magic strings/numbers without named constants; no god functions introduced.

### Security

- [ ] A-013 Constitution I (Security First): subprocess args remain explicit slices routed through `internal/proc`; the seam defaults point at `proc.Run`/`proc.RunForeground`; no shell strings introduced.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
