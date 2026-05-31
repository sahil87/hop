# Spec: Add --skip-brew-update flag to update command

**Change**: 260531-dx50-skip-brew-update-flag
**Created**: 2026-05-31
**Affected memory**: `docs/memory/cli/subcommands.md`

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

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Flag name EXACTLY `--skip-brew-update`, boolean, default `false`, `BoolVar`, `NoArgs` retained | Fixed by cross-toolkit contract; repo convention (clone.go/sync.go). Confirmed from intake #1/#5 | S:100 R:90 A:100 D:100 |
| 2 | Certain | Skip gates ONLY `brew update --quiet` via `if !skipBrewUpdate`; info check, short-circuit, upgrade unchanged | Explicit in contract; single `brew update` call at update.go ~L64. Confirmed from intake #2 | S:100 R:85 A:95 D:100 |
| 3 | Certain | Default (absent) preserves current behavior byte-for-byte | Guard wraps the existing block unchanged. Confirmed from intake #3 | S:100 R:90 A:100 D:100 |
| 4 | Certain | Signature `Run(currentVersion string, skipBrewUpdate bool, out, errOut io.Writer) error`; single call site updated; ErrNotFound→errSilent preserved | Contract names Run + threading; verified single caller. Confirmed from intake #4 | S:95 R:85 A:100 D:95 |
| 5 | Certain | Preserve output routing; no new out/errOut writes from the guard; do NOT refactor | Contract: "Preserve the intentional brew output routing" + "do NOT refactor". Confirmed from intake #6 | S:95 R:80 A:95 D:90 |
| 6 | Confident | Test seam = minimal unexported package-local function-var indirection (brewRun/brewRunForeground, optionally isBrewInstalled override), defaults to real internal/proc, recorder in test | No seam today; refactor forbidden; least-invasive reversible option. Upgraded from intake Tentative #9 after spec-level analysis confirmed it preserves Constitution I and production behavior | S:75 R:70 A:75 D:70 |
| 7 | Confident | Test asserts both directions (skip omits update+runs upgrade; default runs both) without brew/network/Cellar, testing-only, restores vars via Cleanup | Contract requires the assertion; repo test pattern is testing-only with bytes.Buffer. Confirmed from intake (test requirement) | S:90 R:80 A:90 D:85 |
| 8 | Confident | PR title EXACTLY `feat: add --skip-brew-update flag to update command`; OPEN PR, no merge, no Claude/Co-Authored-By footer | Stated verbatim in contract. Confirmed from intake #7 | S:100 R:95 A:90 D:95 |
| 9 | Confident | `docs/memory/cli/subcommands.md` updated at hydrate to document the flag | CLI surface change; memory index lists cli/subcommands as surface doc. Confirmed from intake #8 | S:80 R:80 A:85 D:80 |

9 assumptions (5 certain, 4 confident, 0 tentative, 0 unresolved).
