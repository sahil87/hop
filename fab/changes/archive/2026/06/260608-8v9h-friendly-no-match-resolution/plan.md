# Plan: Friendlier No-Match DX for Repo Resolution

**Change**: 260608-8v9h-friendly-no-match-resolution
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### CLI: Match Resolution Picker Behavior

#### R1: Suppress `--query` prefill on zero matches
When a `<selection>` query matches **zero** repos, `resolveByName` SHALL pass an empty query (`""`) to the fzf picker so fzf opens on the full browsable repo list, rather than prefilling the dead query and showing an empty filtered picker. The prefill SHALL be retained for the 2+-match case, and the existing 1-match short-circuit (no fzf) SHALL be unchanged.

- **GIVEN** a `hop.yaml` with 2+ repos and a query that `Repos.MatchOne` reduces to 0 candidates (e.g. `hop looo`)
- **WHEN** `resolveByName` falls through to the fzf picker
- **THEN** the query argument passed to the picker SHALL be `""` (the full list is browsable, no `--query` flag emitted)
- **AND** GIVEN a query that `MatchOne` reduces to 2+ candidates (e.g. `hop web` over `webapp`/`web-tools`), WHEN it falls through to the picker, THEN the original query SHALL be passed verbatim (prefilled to narrow)
- **AND** GIVEN a query that `MatchOne` reduces to exactly 1 candidate, WHEN `resolveByName` runs, THEN it SHALL return that candidate directly without invoking the picker (unchanged)

#### R2: Fold fzf exit code 1 into quiet cancellation
The `resolveByName` cancellation handler SHALL map fzf exit code **1** (list exhausted, no selectable match) to `errFzfCancelled` alongside exit code **130** (Esc/Ctrl-C), so neither leaks the internal `hop: fzf failed: exit status 1` string. Any other non-zero exit code SHALL still surface as a real `hop: fzf failed: %w` error.

- **GIVEN** the fzf picker returns an error whose `proc.ExitCode` is 1
- **WHEN** `resolveByName` handles that error
- **THEN** it SHALL return `errFzfCancelled` (which `translateExit` maps to exit 130), not a `fzf failed` wrap
- **AND** GIVEN an error whose `proc.ExitCode` is 130, WHEN handled, THEN it SHALL also return `errFzfCancelled` (unchanged)
- **AND** GIVEN an error whose `proc.ExitCode` is some other non-zero value (e.g. 2), WHEN handled, THEN it SHALL surface as `hop: fzf failed: %w`

#### R3: Introduce a swappable picker seam in resolve.go
`resolveByName` SHALL invoke the interactive fzf selection through a package-level `var` seam (defaulting to `fzf.Pick`) rather than calling `fzf.Pick` directly, mirroring the `pickOne = fzf.Pick` precedent in `config_rm.go`. The seam SHALL be swappable in tests to capture the query argument and inject exit-code errors without spawning a real fzf.

- **GIVEN** the production default
- **WHEN** `resolveByName` reaches the picker step
- **THEN** it SHALL call the seam variable (which equals `fzf.Pick`) with `(context.Background(), pickerLines, fzfQuery)`
- **AND** GIVEN a test that reassigns the seam (saving/restoring via `t.Cleanup`/`defer`), WHEN `resolveByName` runs, THEN the test fake SHALL receive the query argument and may return an injected error

### Non-Goals

- `config_rm.go`'s separate fzf picker (`pickOne` / its `pickRepo` function and own `fzf failed` wrap) — out of scope; the reported path is repo resolution (`resolveByName`).
- `internal/fzf/fzf.go` — already omits `--query` when the query is `""` (verified, no change needed).
- `internal/proc` — `ExitCode` already extracts the subprocess exit code (no change needed).
- CLI surface — no new flags, subcommands, or config.

### Design Decisions

1. **Seam variable named `pickResolve`, not `pickRepo`** (deviation from intake's literal snippet): `package main` already declares both `var pickOne = fzf.Pick` and a `func pickRepo(...)` in `config_rm.go`. Declaring `var pickRepo = fzf.Pick` in `resolve.go` would be a duplicate-symbol compile error. *Why `pickResolve`*: preserves the intake's intent (a swappable `= fzf.Pick` seam mirroring `pickOne`) while avoiding the collision and naming it after its call site (`resolveByName`). *Rejected*: `pickRepo` (collides with existing func), `pickOne` (collides with existing var).
2. **Decide `fzfQuery` from match count, suppress only on exactly 0** — *Why*: matches the behavior matrix; bare-`hop` (`query == ""`) already passes `""`, 1-match short-circuits, 2+ keeps the query, and only the 0-match case is changed. *Rejected*: suppressing on `<2` (would wrongly strip the prefill — but 1-match never reaches fzf anyway, so this is moot; explicit `== 0` is clearest).

## Tasks

### Phase 2: Core Implementation

- [x] T001 In `src/cmd/hop/resolve.go`, add a package-level `var pickResolve = fzf.Pick` seam (doc comment mirroring `config_rm.go`'s `pickOne`), placed near the other sentinel/seam declarations. <!-- R3 -->
- [x] T002 In `src/cmd/hop/resolve.go::resolveByName`, compute `fzfQuery := query`; inside the `if query != ""` block, after the 1-match short-circuit, set `fzfQuery = ""` when `len(candidates) == 0`; pass `fzfQuery` (not `query`) to the picker. <!-- R1 -->
- [x] T003 In `src/cmd/hop/resolve.go::resolveByName`, change the picker call from `fzf.Pick(...)` to `pickResolve(context.Background(), pickerLines, fzfQuery)`. <!-- R3 -->
- [x] T004 In `src/cmd/hop/resolve.go::resolveByName`, widen the cancellation guard to `if code, ok := proc.ExitCode(err); ok && (code == 130 || code == 1)` and update the adjacent comment to explain both 130 and 1 mean "no repo chosen". <!-- R2 -->

### Phase 3: Tests

- [x] T005 In `src/cmd/hop/resolve_test.go`, add a `withPickResolve` test helper (mirroring `withPickOne`) that swaps `pickResolve` and restores it via `t.Cleanup`. <!-- R3 -->
- [x] T006 In `src/cmd/hop/resolve_test.go`, add a test asserting a 0-match query (e.g. `looo`) reaches the seam with query `""`. <!-- R1 -->
- [x] T007 In `src/cmd/hop/resolve_test.go`, add a test asserting a 2+-match query reaches the seam with the original query verbatim. <!-- R1 -->
- [x] T008 In `src/cmd/hop/resolve_test.go`, add a test injecting a real `*exec.ExitError` with code 1 via the seam and asserting `resolveByName` returns `errFzfCancelled` (not a `fzf failed` wrap). Also added `TestResolveByNameFzfOtherExitSurfacesError` covering a non-130/1 exit (A-009). <!-- R2 -->
- [x] T009 Confirmed `src/internal/fzf/fzf_test.go::TestBuildArgsEmptyQuery` already asserts `buildArgs("")` omits `--query`; no change needed. <!-- R1 -->

## Execution Order

- T001 → T003 (seam must exist before it is called)
- T002 and T004 are independent edits to the same function body; both precede the tests
- T005 → T006, T007, T008 (helper before the tests that use it)

## Acceptance

### Functional Completeness

- [ ] A-001 R1: `resolveByName` passes `""` to the picker on 0 matches and the original query on 2+ matches; the 1-match path still short-circuits without fzf.
- [ ] A-002 R2: The cancellation handler maps both fzf exit 130 and exit 1 to `errFzfCancelled`.
- [ ] A-003 R3: `resolveByName` calls a package-level `pickResolve` seam (default `fzf.Pick`) instead of `fzf.Pick` directly.

### Behavioral Correctness

- [ ] A-004 R1: A previously dead-end no-match query now opens the full browsable list (`fzfQuery == ""`) rather than an empty `0/N` picker.
- [ ] A-005 R2: A dismissed/no-match picker no longer leaks `hop: fzf failed: exit status 1`; it exits quietly via `errFzfCancelled` (exit 130). A non-130/non-1 exit still surfaces as `hop: fzf failed`.

### Scenario Coverage

- [ ] A-006 R1: `resolve_test.go` has a test capturing the seam query arg = `""` for a 0-match query and = the input for a 2+-match query.
- [ ] A-007 R2: `resolve_test.go` has a test injecting a code-1 `*exec.ExitError` and asserting `errFzfCancelled`.
- [ ] A-008 R1: `fzf_test.go::TestBuildArgsEmptyQuery` asserts `buildArgs("")` omits `--query`.

### Edge Cases & Error Handling

- [ ] A-009 R2: An fzf exit code other than 130/1 (e.g. via `proc.ExitCode`) still returns a `hop: fzf failed: %w` error, not a silent cancellation.

### Code Quality

- [ ] A-010 Pattern consistency: The `pickResolve` seam and its `withPickResolve` test helper follow the existing `pickOne`/`withPickOne` idiom (same shape, doc comment, cleanup-based restore).
- [ ] A-011 No unnecessary duplication: Tests reuse existing helpers (`writeReposFixture`, the `exit*Error` ExitError-construction pattern) rather than reimplementing them.
- [ ] A-012 Readability over cleverness (code-quality principle): the `fzfQuery` decision is a plain explicit `len(candidates) == 0` branch, not a clever conditional.
- [ ] A-013 No magic numbers without intent (anti-pattern): exit codes 130 and 1 carry an explanatory comment describing what each means.

### Security

- [ ] A-014 R3: The seam still routes through `internal/fzf` → `internal/proc` (`exec.CommandContext` + arg slices); no raw `exec.Command` or shell string is introduced in production code (Constitution Principle I & IV).

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Tentative | Name the resolve.go picker seam `pickResolve` instead of the intake's literal `pickRepo` | `package main` already has both `var pickOne` and `func pickRepo` in `config_rm.go`; `var pickRepo = fzf.Pick` would be a duplicate-symbol compile error. `pickResolve` preserves the seam intent (mirrors `pickOne`) without collision. Reversible (rename) and codebase-determined, but it is a deviation from the intake's exact snippet, so graded Tentative for visibility. | S:80 R:85 A:90 D:70 |
| 2 | Certain | Construct the code-1 fake via a real `exec.Command("sh","-c","exit 1").Run()` `*exec.ExitError` | Mirrors the existing `exit130Error` helper in `config_rm_test.go`; `proc.ExitCode` classifies it identically to a real fzf exit. Test files already import `os/exec`. | S:95 R:80 A:90 D:90 |
| 3 | Certain | 2+-match fixture uses two repos whose names share a common substring so `MatchOne` returns 2 candidates and the flow falls through to fzf with the query retained | Direct consequence of the `MatchOne` case-insensitive-substring contract; the only way to exercise the 2+ branch deterministically. | S:90 R:85 A:90 D:85 |

3 assumptions (2 certain, 0 confident, 1 tentative).
