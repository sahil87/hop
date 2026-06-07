# Plan: Fix relative-dir handling in `hop add` / `hop config scan`

**Change**: 260605-c92v-fix-relative-dir-args
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### CLI: Directory-argument resolution

#### R1: `validateConfigDir` resolves to an absolute path
The shared `validateConfigDir` helper (in `src/cmd/hop/config_scan.go`, used by both `hop add <dir>` and `hop config scan <dir>`) SHALL resolve its `userArg` to an absolute, symlink-evaluated path before returning. It MUST call `filepath.Abs(userArg)` first, then `filepath.EvalSymlinks`, then `os.Stat` (directory check). The redundant `filepath.Clean` call is dropped (`filepath.Abs` already cleans). The not-a-directory error wording (`'%s' is not a directory.`) and verbatim `userArg` echo MUST be preserved.

- **GIVEN** the process CWD is `~/code/sahil87` and a convention-layout repo `hop` exists there
- **WHEN** the user runs `hop add hop` (a bare relative name)
- **THEN** `validateConfigDir` returns the absolute canonical path `/home/.../code/sahil87/hop`
- **AND** the returned `Found.Path` is absolute, so `filepath.Dir`/`filepath.Base` yield a real parent and convention matching works
- **AND** no `cannot derive group name from parent dir '.'` line is emitted

#### R2: Relative-arg add for a convention repo lands in `default`
Given a convention-layout repo (`<code_root>/<org>/<name>`) NOT yet in config, `hop add <relative-name>` invoked from the parent directory SHALL register the repo's URL into the `default` group. It MUST NOT emit a false "already registered" message and MUST NOT emit the slugify-failure skip line.

- **GIVEN** `code_root: ~/code`, CWD is `~/code/sahil87`, repo `hop` (remote `git@github.com:sahil87/hop.git`) is on disk and absent from config
- **WHEN** the user runs `hop add hop`
- **THEN** the URL is merged under the `default` group, `added:` and `wrote:` lines are printed, exit 0

#### R3: Absolute-arg behavior is unchanged (regression guard)
The absolutization change SHALL be a no-op for absolute inputs: `filepath.Abs` on an already-absolute path returns it unchanged (after Clean), and `EvalSymlinks` already returned absolute for absolute inputs. Existing absolute-arg add/scan behavior MUST remain identical.

- **GIVEN** a convention repo on disk and absent from config
- **WHEN** the user runs `hop add /abs/path/to/repo`
- **THEN** the URL lands in `default` with `added:`/`wrote:` exactly as before the change

### CLI: "already registered" reporting

#### R4: `runAdd` reports "already registered" only on a genuine duplicate
`runAdd` (in `src/cmd/hop/config_add.go`) SHALL print `"%s: %s already registered in %s. Nothing to add."` ONLY when the classified repo's URL already exists in the loaded config. This determination MUST be made by a helper `urlAlreadyRegistered(cfg, url)` that exact-matches against `cfg.Groups[*].URLs` (no normalization, mirroring `buildScanPlan`'s `existingURLs`), checked BEFORE building the scan plan.

- **GIVEN** a repo whose URL is already present in `hop.yaml`
- **WHEN** the user runs `hop add <dir>` for it
- **THEN** the "already registered" message is printed and the file is unchanged, exit 0

#### R5: A skipped (non-dedup) candidate is not misreported as "already registered"
When the candidate's URL is NOT a duplicate but the built scan plan is still empty (the candidate was skipped for a non-dedup reason — e.g. a genuine slugify failure, where `buildScanPlan` already emitted a `skip:` line), `runAdd` SHALL print the fallback `"%s: '%s' could not be registered (see skip above). Nothing to add."` (with `cmdName` and `userArg`) and return nil. It MUST NOT claim the repo was already registered.

- **GIVEN** a repo whose URL is absent from config but whose candidate is skipped during plan building
- **WHEN** the user runs `hop add <dir>` for it
- **THEN** the fallback "could not be registered" message is printed, NOT "already registered", exit 0

### Design Decisions

1. **Absolutize in the shared helper**: Fix `validateConfigDir` once so both `add` and `scan` are corrected — *Why*: Constitution IV (wrap once, don't duplicate); both commands call this exact helper — *Rejected*: absolutizing separately in each caller (duplication, drift risk).
2. **Pre-plan dedup check for Bug B**: Distinguish "deduped" from "skipped" via a pre-plan `urlAlreadyRegistered` check plus a corrected fallback message — *Why*: minimal surface; the user-visible defect is resolved without changing the scan-plan return contract — *Rejected*: threading a structured skip-reason out of `buildScanPlan` back into `runAdd` (larger refactor of the return contract, not needed).
3. **Exact-match dedup (no normalization)**: `urlAlreadyRegistered` compares URLs verbatim — *Why*: mirrors `buildScanPlan`'s existing `existingURLs` semantics so `add` and `scan` stay consistent — *Rejected*: URL normalization (would diverge from scan's dedup behavior).

### Non-Goals

- The `fzf` shim failure (stale `_hop_dispatch` in the user's shell) — an environment issue, not a code bug.
- The stale `$HOP_CONFIG` header comment in generated `hop.yaml` — an unrelated doc nit from #36.
- Any change to `internal/scan`, `internal/config`, or `internal/yamled` — the bug is entirely in the CLI layer.
- New flags, env vars, subcommands, schema, or YAML-format changes.

## Tasks

### Phase 2: Core Implementation

- [x] T001 In `src/cmd/hop/config_scan.go`, change `validateConfigDir` to call `filepath.Abs(userArg)` first (error → not-a-directory message), then `filepath.EvalSymlinks(abs)`, then `os.Stat`; drop the `filepath.Clean` call. Update the doc comment to read "filepath.Abs → filepath.EvalSymlinks → os.Stat". <!-- R1 -->
- [x] T002 In `src/cmd/hop/config_add.go`, add a `urlAlreadyRegistered(cfg *config.Config, url string) bool` helper that exact-matches `url` against `cfg.Groups[*].URLs`. <!-- R4 -->
- [x] T003 In `src/cmd/hop/config_add.go` `runAdd`, after `ClassifyOne`/`isRepo`, call `urlAlreadyRegistered(cfg, found.URL)` BEFORE `buildScanPlan`; print "already registered" only when true. Then build the plan; if `planIsEmpty(plan)` still holds, print the fallback `"%s: '%s' could not be registered (see skip above). Nothing to add."` and return nil. <!-- R4, R5 -->

### Phase 3: Tests

- [x] T004 [P] Add `TestTopLevelAddRelativeArgConventionLandsInDefault` to `src/cmd/hop/config_add_test.go`: Chdir into `~/code/sahil87`, `hop add hop` (relative), assert URL in `default`, no "cannot derive group name", no false "already registered". <!-- R2 -->
- [x] T005 [P] Add `TestAddSkippedCandidateNotReportedAsRegistered` to `src/cmd/hop/config_add_test.go`: an unregistered repo whose candidate skips to an empty plan does NOT print "already registered" (prints the fallback instead); pair with assertion that a genuinely-registered repo DOES print "already registered". <!-- R5, R4 -->
- [x] T006 [P] Add `TestAddAbsoluteArgParityUnchanged` to `src/cmd/hop/config_add_test.go` (or rely on existing absolute-arg tests) confirming absolute-path add still lands in `default` with `added:`/`wrote:`. <!-- R3 -->

## Execution Order

- T001 blocks T004 and T006 (relative/parity tests depend on the absolute-path fix).
- T002 blocks T003 (helper used by runAdd).
- T003 blocks T005 (skip-vs-dup test depends on the corrected reporting).
- T004, T005, T006 are independent of each other once their blockers land.

## Acceptance

### Functional Completeness

- [ ] A-001 R1: `validateConfigDir` returns an absolute, symlink-evaluated path; uses `filepath.Abs` → `EvalSymlinks` → `os.Stat`; `filepath.Clean` removed; doc comment updated.
- [ ] A-002 R2: relative-arg add of a convention repo not in config writes the URL under `default` (regression test passes).
- [ ] A-003 R4: `runAdd` has a `urlAlreadyRegistered` helper exact-matching `cfg.Groups[*].URLs`, checked before plan building.
- [ ] A-004 R5: `runAdd` prints the `could not be registered (see skip above)` fallback when the candidate is skipped (non-dedup) to an empty plan.

### Behavioral Correctness

- [ ] A-005 R2: no `cannot derive group name from parent dir '.'` line and no false "already registered" for the relative convention-repo case.
- [ ] A-006 R4: "already registered" is printed for a genuinely-registered repo and the file is unchanged.
- [ ] A-007 R5: "already registered" is NOT printed for an unregistered repo that fails to register.

### Scenario Coverage

- [ ] A-008 R2: `TestTopLevelAddRelativeArgConventionLandsInDefault` exercises the relative-arg path with a fake gitRunner and temp config.
- [ ] A-009 R5/R4: `TestAddSkippedCandidateNotReportedAsRegistered` exercises both the skip and dup branches.
- [ ] A-010 R3: absolute-arg parity is verified (new or existing test) — behavior unchanged.

### Edge Cases & Error Handling

- [ ] A-011 R1: a non-existent relative arg still yields the `'%s' is not a directory.` message with `userArg` echoed verbatim, exit 2.

### Code Quality

- [ ] A-012 Pattern consistency: new code matches `cmd/hop/` naming, comment density, and error-handling style of neighboring files.
- [ ] A-013 No unnecessary duplication: `urlAlreadyRegistered` mirrors `buildScanPlan`'s `existingURLs` rather than introducing divergent dedup logic; the absolute-path fix lives once in the shared helper (Anti-Pattern: duplicating existing utilities).
- [ ] A-014 Composition / scope: changes are CLI-layer only; `internal/scan`, `internal/config`, `internal/yamled` untouched.

### Security

- [ ] A-015 R1: no shell strings introduced; process execution continues to use the injectable `gitRunner` (`exec.CommandContext` under the hood); user-provided path is validated before subprocess use (Constitution I).

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Bug A fix is `filepath.Abs` before `EvalSymlinks` in shared `validateConfigDir`; drop `filepath.Clean` | Verbatim from intake "What Changes" §1 with code block; Constitution IV. | S:95 R:80 A:95 D:90 |
| 2 | Certain | Bug B fix is a pre-plan `urlAlreadyRegistered` exact-match check + corrected fallback message | Verbatim from intake "What Changes" §2 with code block; user-confirmed lighter approach (intake Assumption 7). | S:95 R:55 A:65 D:50 |
| 3 | Certain | Fallback wording: `'<dir>' could not be registered (see skip above). Nothing to add.` | Intake Open Questions / Assumption 8 — user confirmed exact wording. | S:95 R:90 A:55 D:55 |
| 4 | Certain | `urlAlreadyRegistered` uses exact URL match (no normalization) | Intake Assumption 5 — mirrors `buildScanPlan`'s `existingURLs`. | S:95 R:70 A:85 D:75 |
| 5 | Confident | Relative-arg test drives the fix by `os.Chdir` into the parent dir then passing the bare repo name; fake gitRunner keys on the canonical absolute dir | Intake §3 mandates relative-arg regression test "from a known CWD"; `open_test.go` establishes the `os.Chdir`+cleanup pattern; the fix produces the canonical absolute path the fake keys on. | S:80 R:75 A:80 D:70 |

5 assumptions (4 certain, 1 confident, 0 tentative, 0 unresolved).
