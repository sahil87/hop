# Plan: Tear down shll.ai help-dump push wiring

**Change**: 260603-g56l-teardown-shllai-push-wiring
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

<!-- Removal-only change. shll.ai inverted its help-collection transport (push → pull)
     on 2026-06-03; the puller is confirmed live. Tear down hop's now-dead push wiring
     while PRESERVING the `hop help-dump` producer command (the contract surface) and its
     tests. No Go source is modified. -->

### CI: Release workflow push step removal

#### R1: Remove the shll.ai push step from `release.yml`
The `.github/workflows/release.yml` `release` job MUST NOT contain the `Dump help tree and PR to shll.ai` step. That step is the only live `SHLLAI_TOKEN` consumer in the workflow; removing it removes all live token usage. All other steps MUST remain byte-identical.

- **GIVEN** the `release` job currently has 8 steps ending with `Dump help tree and PR to shll.ai`
- **WHEN** the push step is deleted
- **THEN** the `release` job has 7 steps: Checkout, Create tag (manual dispatch), Setup Go, Extract version, Cross-compile, Determine release notes base tag, Create GitHub Release, Update Homebrew tap
- **AND** the `Update Homebrew tap` step and the `Create GitHub Release` step are unchanged (byte-identical)
- **AND** `SHLLAI_TOKEN` no longer appears anywhere in `release.yml`

<!-- assumed: the intake states "8 steps to 7" and lists 7 step names that omit the
     workflow_dispatch-only `Create tag (manual dispatch)` step. That step is gated by
     `if: github.event_name == 'workflow_dispatch'` and is not part of the tag-push path the
     intake's step count describes. The teardown only removes the final push step; the count
     framing is descriptive, not a step to add/remove. -->

#### R2: Preserve `permissions: contents: write`
The workflow's top-level `permissions: contents: write` block MUST remain unchanged. It was scoped for the GitHub Release step (which uses `GITHUB_TOKEN`), not the shll.ai push (which used the `SHLLAI_TOKEN` PAT).

- **GIVEN** `permissions: contents: write` is declared at the workflow top level
- **WHEN** the push step is removed
- **THEN** `permissions: contents: write` is still present and unchanged

### Build: Local help-dump recipe and script removal

#### R3: Remove the `help-dump` recipe from `justfile`
The `justfile` MUST NOT contain the `help-dump` recipe or its preceding comment. Per Constitution Principle V (thin justfile, recipes delegate to `scripts/`), the recipe and its dedicated script are removed together so neither is orphaned.

- **GIVEN** the `justfile` has a `help-dump` recipe preceded by the comment `# Build hop and pretty-print its CLI help tree as JSON (the help/hop.json contract).`
- **WHEN** the recipe and its preceding comment are deleted
- **THEN** the `justfile` no longer references `help-dump` or `scripts/help-dump.sh`
- **AND** the `build`, `local-install`, `test`, `release`, and `default` recipes remain unchanged

#### R4: Delete `scripts/help-dump.sh`
The file `scripts/help-dump.sh` MUST be deleted in its entirety.

- **GIVEN** `scripts/help-dump.sh` exists
- **WHEN** the file is deleted
- **THEN** `scripts/help-dump.sh` does not exist
- **AND** `scripts/build.sh` (shared by `build`/`local-install`) still exists and is unchanged

### Contract surface: Producer command preservation

#### R5: Preserve the `hop help-dump` producer command and its tests
The Go source files `src/cmd/hop/help_dump.go`, `src/cmd/hop/root.go`, and `src/cmd/hop/help_dump_test.go` MUST NOT be modified. The `hop help-dump` command remains the contract surface; the tests keep it verified. The Go test suite MUST pass after the CI/build removals, since none of the removed artifacts are imported by Go code.

- **GIVEN** the producer command and its tests are the verification of the shll.ai contract
- **WHEN** the push step, the justfile recipe, and `scripts/help-dump.sh` are removed
- **THEN** `src/cmd/hop/help_dump.go`, `src/cmd/hop/root.go`, and `src/cmd/hop/help_dump_test.go` are byte-identical to before the change
- **AND** `cd src && go test ./...` passes (the `TestHelpDump*` tests included)

### Non-Goals

- Deleting the `SHLLAI_TOKEN` GitHub repository secret — a manual maintainer action (GitHub UI) the agent cannot perform; flagged for post-merge follow-up.
- Modifying any `docs/memory/` file — memory hydration is the hydrate stage's responsibility, not apply's.
- Touching the `Update Homebrew tap` step or its `HOMEBREW_TAP_TOKEN` — unrelated and out of scope.
- Adding a replacement test — the existing `help_dump_test.go` already satisfies the "at max a test case" condition; no new test is added.

### Design Decisions

1. **Remove the transport, keep the producer**: Delete only the push wiring (CI step + local recipe/script), preserve the `hop help-dump` command. — *Why*: shll.ai's contract preserves the command as the singular contract surface and now pulls via it; the push transport is dead weight. — *Rejected*: deleting the command too (would break the pull model's contract surface and lose test coverage).
2. **Remove recipe + script together**: Delete the `help-dump` justfile recipe and its dedicated `scripts/help-dump.sh` in the same change. — *Why*: Constitution Principle V — a thin justfile recipe must not be orphaned from its script, nor a script left with no caller. — *Rejected*: removing only one (leaves a dangling reference or an orphaned script).

### Deprecated Requirements

#### shll.ai push transport (from change `jr5f`)
**Reason**: shll.ai inverted its transport to pull (2026-06-03); the push step is now redundant (races the puller), adds PR noise, holds a standing cross-repo write credential, and couples hop to shll.ai internals.
**Migration**: shll.ai's scheduled pull job installs hop and runs `hop help-dump` itself. hop no longer pushes. The `hop help-dump` command remains the contract surface.

## Tasks

### Phase 1: CI workflow removal

- [x] T001 Delete the entire `Dump help tree and PR to shll.ai` step (the final step of the `release` job, including its `continue-on-error`, `env.GH_TOKEN: ${{ secrets.SHLLAI_TOKEN }}`, and the full `run:` block) from `.github/workflows/release.yml`. Leave the `Update Homebrew tap` step and all earlier steps byte-identical; leave `permissions: contents: write` unchanged. <!-- R1 -->

### Phase 2: Local build removal

- [x] T002 [P] Remove the `help-dump` recipe and its preceding comment (`# Build hop and pretty-print its CLI help tree as JSON (the help/hop.json contract).`) from `justfile`. Leave `default`, `build`, `local-install`, `test`, and `release` recipes unchanged. <!-- R3 -->
- [x] T003 [P] Delete the file `scripts/help-dump.sh` entirely. Leave `scripts/build.sh` untouched. <!-- R4 -->

### Phase 3: Verification

- [x] T004 Confirm `src/cmd/hop/help_dump.go`, `src/cmd/hop/root.go`, and `src/cmd/hop/help_dump_test.go` are untouched, then run `cd src && go test ./...` and confirm it passes. Sanity-check `.github/workflows/release.yml` parses as valid YAML and that `SHLLAI_TOKEN` no longer appears outside `docs/` and `fab/` (repo-wide grep). <!-- R2, R5 -->

## Execution Order

- T001, T002, T003 are independent edits (different files) and may run in parallel.
- T004 (verification) runs after T001–T003 complete.

## Acceptance

### Functional Completeness

- [x] A-001 R1: The `Dump help tree and PR to shll.ai` step is absent from `.github/workflows/release.yml`; the `release` job ends at the `Update Homebrew tap` step.
- [x] A-002 R3: The `help-dump` recipe and its preceding comment are absent from `justfile`.
- [x] A-003 R4: `scripts/help-dump.sh` no longer exists; `scripts/build.sh` still exists.
- [x] A-004 R5: `src/cmd/hop/help_dump.go`, `src/cmd/hop/root.go`, and `src/cmd/hop/help_dump_test.go` are unchanged by this change.

### Behavioral Correctness

- [x] A-005 R2: `permissions: contents: write` remains present and unchanged in `release.yml`; the `Create GitHub Release` and `Update Homebrew tap` steps are byte-identical to before.

### Removal Verification

- [x] A-006 R1: `SHLLAI_TOKEN` does not appear anywhere outside `docs/` and `fab/` (verified by repo-wide grep excluding those dirs).
- [x] A-007 R4: `justfile` contains no remaining reference to `help-dump` or `scripts/help-dump.sh`.

### Scenario Coverage

- [x] A-008 R5: `cd src && go test ./...` passes, including the `TestHelpDump*` tests — confirming no removed artifact is imported by Go code.

### Edge Cases & Error Handling

- [x] A-009 R1: `.github/workflows/release.yml` is still well-formed YAML after the step removal.

### Code Quality

- [x] A-010 Pattern consistency: The removal leaves the surrounding files following their existing structure (justfile stays thin and one-line-per-recipe; workflow steps remain intact and ordered).
- [x] A-011 No unnecessary duplication: No new code or script is introduced; the shared `scripts/build.sh` is reused by remaining recipes, not duplicated.
- [x] A-012 R3: Constitution Principle V is honored — the `help-dump` recipe and its dedicated script are removed together; no orphaned recipe or orphaned script remains.
- [x] A-013 No magic strings / dead references: No dangling reference to the removed step, recipe, or script remains in the edited files.

### Security

- [x] A-014 R1: The standing cross-repo write credential usage (`SHLLAI_TOKEN`) is removed from the release pipeline (Constitution Principle I — Security First); the only live consumer is gone.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- The `SHLLAI_TOKEN` GitHub repository secret deletion is a manual maintainer action (GitHub UI) flagged for the PR description — out of scope for code.
- `docs/memory/` updates (release-pipeline 8→7 steps, drop the publish section/secret/runbook items, soften subcommands phrasing) are owned by the hydrate stage, not apply.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Delete only the final `Dump help tree and PR to shll.ai` step from `release.yml`; leave Homebrew + GitHub Release steps and `permissions: contents: write` byte-identical | Intake names the exact step; repo-wide grep confirms `release.yml:145` is the only live `SHLLAI_TOKEN` reference | S:95 R:75 A:95 D:95 |
| 2 | Certain | Remove the `help-dump` justfile recipe + its preceding comment and delete `scripts/help-dump.sh` together; keep `scripts/build.sh` | Intake + Constitution Principle V: recipe and dedicated script are removed as a pair, never orphaned; `build.sh` is shared and must stay | S:95 R:80 A:95 D:95 |
| 3 | Certain | Preserve `help_dump.go`, `root.go`, and `help_dump_test.go` unchanged; verify via `cd src && go test ./...` | Spec and user both explicit on PRESERVE; no removed artifact is imported by Go code, so tests pass unchanged | S:98 R:90 A:98 D:98 |
| 4 | Certain | The "8 steps to 7" framing in the intake describes the tag-push step sequence and omits the `workflow_dispatch`-only `Create tag (manual dispatch)` step; the teardown only removes the final push step, not any other step | The count is descriptive of the intake's listed step names; only one step is deleted regardless of the count framing. Verified against the actual workflow | S:90 R:85 A:90 D:90 |

4 assumptions (4 certain, 0 confident, 0 tentative).
</content>
</invoke>
