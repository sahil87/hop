# Plan: shll update/version/shell-init Standards Conformance

**Change**: 260719-g94r-shll-update-version-shell-init-conformance
**Intake**: `intake.md`

## Requirements

### Update: Brew-handling safety

#### R1: No deadline on foreground `brew upgrade`
`hop update` MUST NOT impose a deadline on the foreground `brew upgrade` invocation. The context passed to the upgrade call SHALL carry no deadline (`ctx.Deadline()` reports none), so the wrapper can never SIGKILL brew mid-transaction (shll update standard: "MUST NOT send SIGKILL to a package-manager subprocess mid-transaction", "MUST NOT impose a short hard timeout on `brew upgrade`"). The `brewUpgradeTimeout` constant SHALL be removed.

- **GIVEN** a brew-installed hop with a newer tap version available
- **WHEN** `update.Run` reaches the `brew upgrade sahil87/tap/hop` step
- **THEN** the context passed to `brewRunForeground` has no deadline
- **AND** brew's progress streams through inherited stdio, leaving interruption (Ctrl-C/SIGINT) to the user

#### R2: Generous SIGTERM+grace bound on captured `brew update`
The captured `brew update --quiet` call (no visible progress — an unbounded hang would look like a frozen `hop update`) MUST run under a generous 10-minute bound whose cancellation is graceful: SIGTERM first, then a grace period, with kill only as the final escalation. It MUST NOT use the `exec.CommandContext` default SIGKILL-on-cancel (brew update mutates tap git state — the same transaction clause applies).

- **GIVEN** `hop update` without `--skip-brew-update`
- **WHEN** `update.Run` invokes `brew update --quiet`
- **THEN** the context carries a deadline ~10 minutes out
- **AND** the invocation routes through the graceful-cancel runner (SIGTERM + grace, never SIGKILL-first)

#### R3: Graceful-cancel runner lives in `internal/proc`
`internal/proc` SHALL gain a graceful-cancel runner (`RunGraceful`) that sets `cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }` and `cmd.WaitDelay` to a grace period (20s) before Go escalates to kill. It is the ONLY place this may live — no package outside `internal/proc` imports `os/exec` (Constitution I). The read-only `brew info --json=v2` query keeps its bounded 30s call but routes through the same graceful path for consistency (SIGTERM first costs nothing). Unix-only signal use is acceptable — Windows is unsupported (Constitution § Cross-Platform Behavior).

- **GIVEN** a subprocess started via `proc.RunGraceful` and a context that gets cancelled
- **WHEN** the context fires
- **THEN** the subprocess receives SIGTERM (trappable — a trap handler runs) rather than SIGKILL
- **AND** a subprocess that ignores SIGTERM is forcibly ended only after the grace period elapses

### Update: Help contract

#### R4: `--skip-brew-update` literal pinned in `hop update --help`
`hop update --help` output MUST contain the exact literal `--skip-brew-update` (shll discovers the flag via `strings.Contains` — a frozen textual contract). Behavior already conforms (cobra renders the flag name); a test SHALL pin the substring.

- **GIVEN** the hop root command
- **WHEN** `hop update --help` runs
- **THEN** stdout contains the literal substring `--skip-brew-update`

### Version: Integration contract

#### R5: `--version` exit 0, stdout, first-line token shape
The `--version` integration test MUST pin the shll version standard's verify checklist: exit code 0; version written to stdout (captured separately from stderr, not `CombinedOutput`); the first non-empty stdout line has the `hop version <nonempty>` prefix shape; and when the trailing field looks like a version (leading digit or `v`+digit), it matches the token regex `v?\d+(\.\d+)*([.-][\w.+-]+)?` — so the test passes for both `hop version dev` (unstamped test build) and `hop version vX.Y.Z` (tagged build). No runtime behavior changes — the audit found `--version` conformant.

- **GIVEN** a built hop binary (version `dev` or a tagged `vX.Y.Z`)
- **WHEN** `hop --version` (and `hop -v`) runs
- **THEN** exit code is 0, stdout's first non-empty line is `hop version <token>`, and a version-looking token matches shll's parse regex

### Shell-init: Usage errors and eval safety

#### R6: Usage errors keep stdout empty
The shell-init usage-error tests (missing shell, unsupported shell) MUST additionally assert that stdout stays empty — the shll shell-init standard requires "missing/unsupported shell → exit 2, usage on stderr, stdout empty". Behavior already conforms (RunE returns before any stdout write); the tests pin the third leg.

- **GIVEN** `hop shell-init` with no shell argument, or `hop shell-init fish`
- **WHEN** the command runs
- **THEN** it exits 2 with the usage message on stderr AND the captured stdout buffer is empty

#### R7: zsh eval-the-shim integration test
An integration test SHALL eval the output of `hop shell-init zsh` in a real zsh subshell (`zsh -f -c`) and exercise one dispatch round-trip (`hop <name> where`), asserting clean exit and correct resolution — mirroring the existing bash sourceable test. The test MUST `t.Skip` when `zsh` is absent from PATH (CI portability — the standard asks for the test, not a hard CI dependency). The existing bash test already covers bash; no duplicate coverage is added.

- **GIVEN** zsh on PATH and a built hop binary
- **WHEN** a `zsh -f` subshell evals `$(hop shell-init zsh)` and runs `hop probe where`
- **THEN** the subshell exits 0 and prints the resolved repo path
- **AND** on a machine without zsh, the test skips rather than fails

### Non-Goals

- No changes to `--version` or `shell-init` runtime behavior — the intake audit found them conformant.
- No CLI surface, help-text, README, or docs/site changes; no dependency changes.
- No changes to shll's consumer side (`shll update`/`version`/`shell-init` delegation) — this change tightens hop's producer side only.

### Design Decisions

#### Graceful cancel as a new `proc.RunGraceful` function
**Decision**: Add `RunGraceful(ctx, name, args...)` as a sixth runner alongside Run/RunCapture/RunCaptureBoth/RunForeground/RunInteractive, rather than an option parameter on `Run`.
**Why**: proc's existing surface is one function per execution mode with a fixed positional signature; an options struct or variadic flag on `Run` would be the package's first and only such parameter, diverging from the established shape for one caller.
**Rejected**: `proc.Run(ctx, name, args..., WithGracefulCancel())` — functional options on a variadic-args signature are awkward in Go and inconsistent with the package's five existing runners.
*Introduced by*: 260719-g94r-shll-update-version-shell-init-conformance

#### Grace period as unexported package var
**Decision**: `var gracefulWaitDelay = 20 * time.Second` in `proc.go` — an unexported package-level var, not a const or parameter.
**Why**: The escalation-after-grace test needs to shorten the grace to run in milliseconds; a swappable var is the codebase's established test-seam pattern (`fzf.runInteractive`, `cmd/hop.listWorktrees`, `update.brewRun`).
**Rejected**: A `grace time.Duration` parameter on `RunGraceful` — pushes a policy knob to every call site when both production callers want the same value.
*Introduced by*: 260719-g94r-shll-update-version-shell-init-conformance

## Tasks

### Phase 2: Core Implementation

- [x] T001 Add `RunGraceful(ctx, name, args...)` to `src/internal/proc/proc.go`: `cmd.Cancel` = SIGTERM signal func, `cmd.WaitDelay` = `gracefulWaitDelay` (new unexported var, 20s); stdout captured, stderr passthrough, `ErrNotFound` mapping — mirroring `Run` <!-- R3 -->
- [x] T002 Add `RunGraceful` tests to `src/internal/proc/proc_test.go`: success, not-found, SIGTERM-delivered-on-cancel (subprocess traps TERM → exit 42, asserted via `ExitCode`), escalates-after-grace (subprocess ignores TERM, `gracefulWaitDelay` swapped short, bounded elapsed time) <!-- R3 -->
- [x] T003 Rework `src/internal/update/update.go` timeout policy: remove `brewUpgradeTimeout` and pass `context.Background()` to the `brew upgrade` foreground call; raise `brewUpdateTimeout` to 10 minutes; point the `brewRun` seam default at `proc.RunGraceful` (covers both `brew update` and `brew info`); update the policy comments citing the shll update standard <!-- R1 -->
- [x] T004 Add `TestRunBrewTimeoutPolicy` to `src/internal/update/update_test.go`: capture contexts at the `brewRun`/`brewRunForeground` seams; assert the upgrade context has NO deadline, the update context has a generous (~10 min) deadline, and the info context has a deadline <!-- R1 -->

### Phase 3: Test Pins (independent)

- [x] T005 [P] Add `TestUpdateHelpAdvertisesSkipBrewUpdate` to `src/cmd/hop/update_test.go` pinning the literal `--skip-brew-update` substring in `hop update --help` stdout <!-- R4 -->
- [x] T006 [P] Strengthen `TestIntegrationVersion` in `src/cmd/hop/integration_test.go`: separate stdout/stderr capture, exit 0, first-non-empty-line `hop version <token>` prefix shape, token regex when the field looks like a version — for both `--version` and `-v` <!-- R5 -->
- [x] T007 [P] Extend `TestShellInitMissingShell` and `TestShellInitUnsupportedShell` in `src/cmd/hop/shell_init_test.go` to assert the captured stdout buffer is empty <!-- R6 -->
- [x] T008 [P] Add `TestIntegrationShellInitZshSourceable` to `src/cmd/hop/integration_test.go`: `zsh -f -c` evals `$(hop shell-init zsh)` and round-trips `hop probe where`; `t.Skip` when zsh is not on PATH <!-- R7 -->

### Phase 4: Verification

- [x] T009 Run the full suite (`cd src && go test ./...`), plus `gofmt -l` and `go vet ./...` over the touched packages; fix any failures <!-- R1 -->

## Execution Order

- T001 blocks T002 and T003 (both use `RunGraceful`); T003 blocks T004
- T005–T008 are independent of each other and of Phase 2 (test-only pins of already-conformant behavior)
- T009 runs last

## Acceptance

### Functional Completeness

- [x] A-001 R1: The `brew upgrade` call receives a context with no deadline; `brewUpgradeTimeout` no longer exists; a seam-level test asserts `ctx.Deadline()` reports none
- [x] A-002 R2: `brew update --quiet` runs under a 10-minute bound routed through the graceful-cancel runner; a test asserts the deadline is present and generous
- [x] A-003 R3: `proc.RunGraceful` exists in `src/internal/proc/proc.go` with `cmd.Cancel` = SIGTERM and `cmd.WaitDelay` = grace; `brew info` stays bounded (30s) and routes through it
- [x] A-004 R4: A test pins the literal `--skip-brew-update` in `hop update --help` stdout
- [x] A-005 R5: `TestIntegrationVersion` asserts exit 0, stdout placement, and the first-non-empty-line shape, passing for both `dev` and tagged builds
- [x] A-006 R6: The two shell-init usage-error tests assert stdout is empty alongside the existing exit-2 + stderr assertions
- [x] A-007 R7: A zsh eval integration test exists, exercises a `where` round-trip, and `t.Skip`s when zsh is absent from PATH

### Behavioral Correctness

- [x] A-008 R1: SIGTERM delivery is proven at the proc level — a trapping subprocess exits through its TERM handler on context cancel (SIGKILL would be untrappable)
- [x] A-009 R2: A SIGTERM-ignoring subprocess is forcibly ended only after the grace period, bounded by the escalation test

### Scenario Coverage

- [x] A-010 R1: `TestRunSkipBrewUpdate` (both directions) and `TestRunNonBrewInstall` still pass unchanged — skip-flag semantics and the non-brew degrade are unaffected by the timeout rework

### Edge Cases & Error Handling

- [x] A-011 R3: `RunGraceful` maps a missing binary to `proc.ErrNotFound` exactly like the sibling runners

### Code Quality

- [x] A-012 Pattern consistency: New code follows naming and structural patterns of surrounding code (per-mode proc runners, seam-var test pattern, comment density)
- [x] A-013 No unnecessary duplication: Existing seams (`brewRun`, `brewRunForeground`) and helpers (`ExitCode`, `writeHopYamlHome`, `buildBinary`) are reused; no parallel test scaffolding introduced
- [x] A-014 No magic numbers: Timeout/grace values are named constants or package vars with rationale comments

### Security

- [x] A-015 R3: All subprocess execution stays inside `internal/proc` with explicit argument slices (`exec.CommandContext`, no shell strings) — Constitution I audit still passes (`os/exec` imports confined to `internal/proc` in non-test code)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Graceful runner is a new `proc.RunGraceful` function, not an option on `Run` | Intake left the shape open ("pick whichever reads best"); proc's surface is one function per execution mode, so a sixth runner reads best | S:70 R:85 A:80 D:65 |
| 2 | Confident | Grace period is 20s via an unexported `gracefulWaitDelay` package var (test seam) | Intake names "e.g. 20s"; the var-seam pattern matches `fzf.runInteractive`/`listWorktrees`/`brewRun` and lets the escalation test run in milliseconds | S:75 R:85 A:80 D:70 |
| 3 | Confident | Graceful-cancel mechanics (SIGTERM delivery, grace escalation) are pinned at the proc level; update-level tests pin only deadline presence/absence at the brew seams | Intake says "pin at whichever seam is testable without invoking real brew"; the seams take a ctx (deadline observable) but Cancel/WaitDelay live on the cmd inside proc | S:65 R:85 A:80 D:65 |
| 4 | Confident | Version first-line assertion: always require the `hop version <nonempty>` prefix shape; apply the token regex only when the trailing field starts with a digit or `v`+digit | Matches intake §3 exactly; `dev` fails the token regex by design, so gating the regex on version-looking tokens is what makes the test pass for both build flavors | S:70 R:85 A:85 D:70 |
| 5 | Certain | zsh eval test mirrors the bash sourceable test verbatim (`zsh -f -c`, eval with stderr suppressed — bare zsh lacks `compdef` — then a `hop probe where` round-trip) | Intake §4 names this exact shape including the round-trip option and the `t.Skip` guard; the bash precedent is in-repo | S:80 R:90 A:90 D:80 |

5 assumptions (1 certain, 4 confident, 0 tentative).
