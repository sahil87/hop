# Plan: `hop rm <name>` Consent Gate

**Change**: 260717-clc4-rm-consent-gate
**Intake**: `intake.md`

## Requirements

### CLI: `hop rm <name>` consent gate

#### R1: TTY confirmation prompt on the direct `hop rm <name>` path
On a controlling TTY, before any write, `hop rm <name>` SHALL display the resolved match (repo name and URL) and prompt `Proceed? [y/N]` on stderr. `y`/`yes` (case-insensitive, surrounding whitespace trimmed) SHALL proceed with the removal; any other input (including bare Enter — the `[y/N]` default is No) SHALL abort with no write. The consent step sits between `resolveOne` and `removeRepo`, on the positional (`name != ""`) path only.

- **GIVEN** `hop.yaml` registers `hop` and `wt`, stdin is a TTY, `--yes` absent, `--dry-run` absent
- **WHEN** I run `hop rm wt` and answer `y`
- **THEN** stderr shows `remove: wt  (git@github.com:sahil87/wt.git)` then the `Proceed? [y/N]` prompt
- **AND** the entry is removed (`removed:` + `wrote:` on stderr), exit 0
- **AND** stdout is empty

#### R2: Declined prompt aborts as a benign no-op (exit 0)
When the prompt is declined (bare Enter, `n`, or any non-affirmative input), `hop rm <name>` SHALL write nothing, print `aborted: no changes written` on stderr, and exit 0 (matching hop's forgiving exit-0 convention for "nothing to remove"; an answered "no" is not an fzf-style cancellation, so 130 is NOT used).

- **GIVEN** the same setup as R1
- **WHEN** I run `hop rm wt` and press Enter (or type `n`, or garbage)
- **THEN** stderr shows `aborted: no changes written`
- **AND** `hop.yaml` is byte-for-byte unchanged
- **AND** exit code is 0

#### R3: `--yes`/`-y` flag skips the prompt
`newRmCmd()` SHALL register a `--yes`/`-y` bool flag. When set, the prompt SHALL be skipped entirely and the removal SHALL proceed (this is principle №1's flag-based consent). It composes trivially with `--dry-run` (which never consults consent at all). On the picker shape (no positional), `--yes` is accepted and ignored — the fzf pick is itself the consent — and does NOT produce a usage error.

- **GIVEN** `hop.yaml` registers `wt`
- **WHEN** I run `hop rm wt --yes` (TTY or no TTY)
- **THEN** no prompt is shown and the entry is removed, exit 0
- **AND** running `hop rm --yes` (picker shape) removes the picked entry with no post-pick prompt and no usage error

#### R4: No-TTY refusal naming the flag (exit 3)
When `name != ""`, `--yes` absent, `--dry-run` absent, and `isTTY()` is false, `hop rm <name>` SHALL refuse fast with no write, printing `hop rm: consent required for removal — re-run with --yes (or preview with --dry-run)` on stderr and exiting 3. This reuses hop's documented "a TTY was required and none is present" convention (exit 3) but with a consent-specific message naming `--yes` — NOT the generic `noTTYHint`. The message is prefixed with the command's `cmdName` (`hop rm` for the canonical command).

- **GIVEN** `hop.yaml` registers `wt`, stdin is not a TTY, `--yes` absent, `--dry-run` absent
- **WHEN** I run `hop rm wt`
- **THEN** stderr shows `hop rm: consent required for removal — re-run with --yes (or preview with --dry-run)`
- **AND** no write occurs (`hop.yaml` unchanged)
- **AND** exit code is 3 (distinct from 130 fzf-cancel and 1 application error)

#### R5: `--dry-run` requires no consent (checked before the gate)
`--dry-run` SHALL be evaluated before the consent gate on the positional path, so a dry-run behaves exactly as today (preview `would remove:` + `dry-run: no changes written`, exit 0) with or without a TTY and with or without `--yes`. Dry-run is never gated by the prompt or the no-TTY refusal.

- **GIVEN** `hop.yaml` registers `wt`, stdin is not a TTY, `--yes` absent
- **WHEN** I run `hop rm wt --dry-run`
- **THEN** stderr shows `would remove: git@github.com:sahil87/wt.git` + `dry-run: no changes written`
- **AND** exit code is 0, `hop.yaml` unchanged (no consent refusal, no prompt)

#### R6: Picker path is ungated (regression guard)
The picker shapes (`hop rm`, `hop rm --stale`, and the hidden `hop config rm`) SHALL NOT gain a post-pick confirmation prompt — the fzf pick is the consent. Their existing exit-code and status-line behavior is unchanged.

- **GIVEN** `hop.yaml` registers `hop` and `wt`, TTY present
- **WHEN** I run `hop rm` and pick `wt`
- **THEN** the entry is removed with no post-pick prompt, exit 0

#### R7: Hidden `hop config rm` alias gains no `--yes` flag
The hidden `hop config rm` alias SHALL NOT register a `--yes`/`-y` flag — it is picker-only (`cobra.NoArgs`), so it has no consent point (its consent is the pick). Adding a meaningless flag would be surface bloat (Constitution VI).

- **GIVEN** the hidden `config rm` command
- **WHEN** its flag set is inspected
- **THEN** no `yes` flag is registered (only `stale` and `dry-run`)

### Docs & Spec

#### R8: Help text, spec, and exit-code doc updates
`rmLong` SHALL document the prompt, `--yes`, and the no-TTY refusal, with an updated examples block. `docs/specs/cli-surface.md` SHALL update the `hop rm` inventory row (behavior + exit codes: exit 3 now also covers consent refusal), add behavioral scenarios (prompt-accept, prompt-decline, no-TTY refusal, `--yes`, `--dry-run` unaffected), and extend the exit-3 meaning. `main.go::translateExit`'s doc comment SHALL note exit 3 now also covers consent refusal. README SHALL be updated only if it documents `hop rm`'s immediacy (checked at apply time).

- **GIVEN** the help/spec surfaces
- **WHEN** the change lands
- **THEN** `rmLong`, `cli-surface.md`, and the `translateExit` doc comment reflect the consent gate; a documented immediacy claim in README (if any) is corrected

### Non-Goals

- No shim / `HOP_WRAPPER` changes — `rm` is `PASSTHROUGH`, stdio is inherited, prompt + exit codes flow through the shim unchanged (intake Assumption 8, verified).
- No changes to `internal/yamled` (`RemoveURL`/`WouldRemoveURL` untouched), the `--shim-plan` classifier, or match resolution.
- Memory updates are deferred to hydrate (see intake Affected Memory).

### Design Decisions

1. **Consent-refusal sentinel**: Introduce a dedicated `errConsentRequired` sentinel that `translateExit` (and, for completeness with the existing pattern, `shimResolveErr`) maps to exit 3 with the consent-specific message — rather than reusing `errNoTTY` (whose `noTTYHint` says "pass a repo name", wrong advice when a name was already passed). *Why*: keeps "3 = needed a terminal / interactive consent" coherent while emitting correct next-step advice. *Rejected*: reusing `errNoTTY` (wrong message); a brand-new exit code 4 (fragments the "no terminal" family for agents branching on codes — intake Assumption 5).
2. **Prompt input read via a `cmd.InOrStdin()`-backed reader**: the confirm helper reads a line from `cmd.InOrStdin()` (cobra's injectable stdin), so seam-injected tests set the command's stdin buffer — no PTY, no new package, mirroring the existing `runArgs` test harness. *Why*: matches hop's seam-injection test idiom and keeps the y/N read local (no subprocess → no injection surface, Constitution I). *Rejected*: a package-level function seam like `pickOne` (heavier than needed; `cmd.InOrStdin()` is already injectable).

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add the `errConsentRequired` sentinel + `consentRefusalMsg` constant (the message names `--yes`) in `src/cmd/hop/config_rm.go` (co-located with `dryRunNoChanges` / `rmLong`); wire `errConsentRequired → exit 3` into `main.go::translateExit` (alongside the `errNoTTY` arm) and into `shim_plan.go::shimResolveErr` if that function branches per-sentinel. <!-- R4 -->
- [x] T002 Add a `confirmRemoval(cmd, stderr, repo)` helper in `src/cmd/hop/config_rm.go` that prints `remove: <name>  (<url>)` and `Proceed? [y/N] ` to stderr, reads one line from `cmd.InOrStdin()` (bufio), and returns true only for a trimmed case-insensitive `y`/`yes`. <!-- R1 -->
- [x] T003 Register the `--yes`/`-y` bool flag on `newRmCmd()` only (NOT `newConfigRmCmd()`), and thread a `yes bool` parameter through `runRm(cmd, cmdName, stale, dryRun, yes, name)`; the hidden alias factory passes `false`. <!-- R3 R7 -->
- [x] T004 In `runRm`, on the positional (`name != ""`) path, insert the consent gate AFTER `resolveOne` and BEFORE `removeRepo`, ordered: (a) if `dryRun` → straight to `removeRepo` (dry-run path, ungated); (b) else if `yes` → straight to `removeRepo`; (c) else if `!isTTY()` → return `errConsentRequired`; (d) else prompt via `confirmRemoval` — on decline print `aborted: no changes written` to stderr and return nil (exit 0), on accept fall through to `removeRepo`. The picker path (`name == ""`) is untouched — `yes` is ignored there. <!-- R1 R2 R4 R5 R6 -->

### Phase 2: Docs & Spec

- [x] T005 Update `rmLong` in `src/cmd/hop/config_rm.go` to document the TTY prompt, `--yes`/`-y`, and the no-TTY refusal; extend the Examples block (add `hop rm widget --yes`). <!-- R8 -->
- [x] T006 Update `main.go::translateExit` doc comment so the exit-3 line covers consent refusal (not only interactive selection). <!-- R8 -->
- [x] T007 Update `docs/specs/cli-surface.md`: the `hop rm [<name>]` inventory row (behavior + exit codes — exit 3 now also covers consent refusal), the two exit-code convention tables (add the consent-refusal trigger to code 3), and add GIVEN/WHEN/THEN behavioral scenarios (prompt-accept, prompt-decline exit 0, no-TTY refusal exit 3, `--yes`, `--dry-run` unaffected). <!-- R8 -->
- [x] T008 Check README for any `hop rm` immediacy/"no confirmation" claim; update it to reflect the consent gate if present, otherwise no-op. <!-- R8 -->

### Phase 3: Tests

- [x] T009 Extend `src/cmd/hop/config_rm_test.go` (seam-injected, following existing patterns) with: TTY + `y` → removal proceeds (stub `isTTY` true, feed `y\n` via cmd stdin); TTY + Enter/`n`/garbage → abort, exit 0, `hop.yaml` unchanged, `aborted:` line; no TTY + no `--yes` + no `--dry-run` → `errConsentRequired`, `translateExit == 3`, no write, message names `--yes`; `--yes` (TTY and no-TTY) → no prompt, removal proceeds (assert stdin never read / no prompt line); `--dry-run` without `--yes`, no TTY → unchanged preview behavior exit 0 (regression guard); picker path → no post-pick prompt (regression guard); hidden `config rm` → no `yes` flag registered. <!-- R1 R2 R3 R4 R5 R6 R7 -->

## Execution Order

- T001–T003 are prerequisites for T004 (the gate uses the sentinel, the helper, and the `yes` param).
- T005–T008 (docs) depend only on the behavior being settled (T004).
- T009 (tests) depends on T001–T004.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `hop rm <name>` on a TTY with neither flag prints the `remove: <name>  (<url>)` match-preview and `Proceed? [y/N]` prompt to stderr before any write, and `y`/`yes` proceeds to removal (exit 0, stdout empty).
- [x] A-002 R3: `newRmCmd()` registers a `--yes`/`-y` bool flag threaded through `runRm`; when set the prompt is skipped and removal proceeds (TTY and no-TTY).
- [x] A-003 R4: the no-TTY + no-`--yes` + no-`--dry-run` positional path returns `errConsentRequired` → exit 3 with the message `hop rm: consent required for removal — re-run with --yes (or preview with --dry-run)`, writing nothing.
- [x] A-004 R7: the hidden `hop config rm` command registers no `yes` flag (only `stale` and `dry-run`).

### Behavioral Correctness

- [x] A-005 R2: a declined prompt (Enter / `n` / garbage) aborts with `aborted: no changes written` on stderr, exit 0, `hop.yaml` byte-for-byte unchanged.
- [x] A-006 R5: `hop rm <name> --dry-run` (no `--yes`, no TTY) previews `would remove:` + `dry-run: no changes written` and exits 0 — the dry-run path is checked before the consent gate and is never refused or prompted.
- [x] A-007 R3: `hop rm --yes` (picker shape) removes the picked entry with no post-pick prompt and no usage-error rejection (`--yes` is accepted-and-ignored on the picker).

### Scenario Coverage

- [x] A-008 R1: a test exercises TTY-accept via `isTTY` stubbed true and `y\n` fed through the command's injected stdin, asserting removal + match-preview line.
- [x] A-009 R2: a test exercises each decline input (Enter, `n`, garbage) asserting the `aborted:` line, exit 0, and unchanged file.
- [x] A-010 R4: a test asserts `errConsentRequired` and `translateExit(err) == 3` with no write on the no-TTY positional path.
- [x] A-011 R6: a regression test asserts the picker path emits no `Proceed?`/`aborted:` prompt line after selection.

### Edge Cases & Error Handling

- [x] A-012 R5: `--yes` composes with `--dry-run` — a `hop rm <name> --yes --dry-run` still takes the dry-run (no-write) path (dry-run precedence over the gate), exit 0.
- [x] A-013 R2: bare Enter (empty line) is treated as No (the `[y/N]` default), consistent with `shll uninstall`.

### Code Quality

- [x] A-014 Pattern consistency: the consent gate reuses the existing `isTTY` seam, `errExitCode`/sentinel + `translateExit` mapping idiom, and `cmdName`-prefixed stderr voice; new code follows the surrounding structure of `config_rm.go`.
- [x] A-015 No unnecessary duplication: the confirm read uses `cmd.InOrStdin()` (already injectable) rather than a new package-level seam; `internal/yamled` and match resolution are untouched.

### Security

- [x] A-016 R1: prompt input is read locally and compared against a fixed accept set — no subprocess is spawned and no user input reaches a shell, so Constitution I (no shell-injection surface) is unaffected.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. (The plan's Design Decision 1 mention of wiring `errConsentRequired` into `shim_plan.go::shimResolveErr` "for completeness" was correctly NOT implemented: `shimResolveErr` only maps `resolveByName` errors on the `--shim-plan` classification path, which `rm` never takes (it classifies `PASSTHROUGH`), so that arm would have been an unreachable zero-call-site branch.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Full consent gate on `hop rm <name>` (TTY prompt + `--yes`/`-y` + no-TTY refusal), gate placed between `resolveOne` and `removeRepo` on the positional path | Intake Assumption 1 — user chose "Full consent gate"; the code seam (`resolveOne` → `removeRepo`) is named in the intake's What Changes §1 | S:100 R:90 A:100 D:100 |
| 2 | Certain | Prompt shape: stderr `remove: <name>  (<url>)` line + `Proceed? [y/N]` with No as default; accept set is trimmed case-insensitive `y`/`yes` | Intake Assumption 2 + the verbatim UX preview in What Changes §1 | S:90 R:85 A:95 D:90 |
| 3 | Certain | `--dry-run` checked before the gate (no consent for dry-run); ordering in `runRm`: dry-run → yes → no-TTY refusal → prompt | Intake Assumption 3 (principle №5 reference: "`--dry-run` requiring no consent") | S:90 R:90 A:100 D:95 |
| 4 | Certain | Picker paths ungated; `--yes` accepted-and-ignored on the picker shape; hidden `config rm` gains no `--yes` flag | Intake Assumptions 4 and 7 | S:85 R:85 A:90 D:90 |
| 5 | Confident | No-TTY consent refusal exits 3 via a dedicated `errConsentRequired` sentinel (parallel to `errNoTTY`, distinct message naming `--yes`) rather than reusing `errNoTTY` or minting a new code | Intake Assumption 5 + the sentinel mechanism the intake left "decided at plan time" (§What Changes 3); `noTTYHint`'s "pass a repo name" advice is wrong once a name was passed, so a distinct sentinel is cleaner than reusing `errNoTTY` | S:75 R:80 A:80 D:75 |
| 6 | Confident | Declined prompt → `aborted: no changes written`, exit 0 (not 130) | Intake Assumption 6 — matches hop's forgiving exit-0 not-found convention; an answered "no" is not an fzf-style cancellation | S:60 R:90 A:75 D:70 |
| 7 | Confident | Refusal wording exactly `hop rm: consent required for removal — re-run with --yes (or preview with --dry-run)` (cmdName-prefixed) | Intake Assumption 9 — follows the what/why/next shape and hop's cmdName stderr voice; trivially reversible | S:55 R:95 A:75 D:65 |
| 8 | Confident | Prompt input read from `cmd.InOrStdin()` via bufio (no new package-level seam); tests inject stdin on the cobra command | Matches hop's seam-injection test idiom and the `runArgs` harness; `cmd.InOrStdin()` is already injectable, so a heavier `pickOne`-style var is unwarranted | S:65 R:85 A:85 D:75 |

8 assumptions (4 certain, 4 confident, 0 tentative).
