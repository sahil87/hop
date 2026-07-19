# Plan: Upgrade fab kit to 2.16.4

**Change**: 260719-obnb-upgrade-fab-kit
**Intake**: `intake.md`

## Requirements

> `fab upgrade-repo` (kit 2.16.0 → 2.16.4) was ALREADY executed in this working
> tree before this plan was generated. The upgrade is a scaffolding-only change:
> three tracked one-line edits under `fab/`, plus `fab sync` repairs to deployed
> `.claude/skills/` files (untracked deployment artifacts). No hop source code
> (`cmd/`, `internal/`) is touched. These requirements are therefore
> **verification** requirements — the apply stage confirms the applied diff
> matches the intake exactly and that the upgraded scaffolding breaks no tests.

### Fab Kit Upgrade: Diff Conformance

#### R1: Version marker files reflect the 2.16.4 upgrade
The working-tree diff for `fab/.fab-version` and `fab/.kit-migration-version` SHALL be exactly the version bumps recorded in the intake — no other lines changed, no other files under `fab/` staged as version metadata.

- **GIVEN** `fab upgrade-repo` ran (2.16.0 → 2.16.4) and its diff is present in the working tree
- **WHEN** the diff for `fab/.fab-version` and `fab/.kit-migration-version` is inspected
- **THEN** `fab/.fab-version` is a single-line change `2.16.0` → `2.16.4`
- **AND** `fab/.kit-migration-version` is a single-line change `2.15.8` → `2.16.4`

#### R2: config.yaml change is the reference-fence header only
The working-tree diff for `fab/project/config.yaml` SHALL be limited to the regenerated reference-fence header comment (`kit 2.16.0` → `kit 2.16.4`); no overridden field above the fence is modified.

- **GIVEN** the upgrade regenerated the `# >>> fab reference (kit …) >>>` fence in `config.yaml`
- **WHEN** the `config.yaml` diff is inspected
- **THEN** the only changed line is the fence header comment `kit 2.16.0` → `kit 2.16.4`
- **AND** no user-overridden field (identity, `source_paths`, `test_paths`, `true_impact_exclude`) is altered

#### R3: No source or unexpected files changed
The change SHALL introduce no edits to hop source code (`cmd/`, `internal/`) or any tracked file other than the three `fab/` metadata files above; the only additional tracked artifact is the change folder itself.

- **GIVEN** the intake scopes the change to three `fab/` metadata files plus the change folder
- **WHEN** `git status` / `git diff --stat` is inspected
- **THEN** the only modified tracked files are `fab/.fab-version`, `fab/.kit-migration-version`, `fab/project/config.yaml`
- **AND** the only untracked addition is `fab/changes/260719-obnb-upgrade-fab-kit/`

### Fab Kit Upgrade: Behavior Preservation

#### R4: The Go test suite passes after the upgrade
The full Go test suite SHALL pass, confirming the upgraded kit scaffolding breaks no hop behavior.

- **GIVEN** the kit upgrade is applied to the working tree
- **WHEN** `go test ./...` is run from the repo root
- **THEN** all packages report `ok` (or `no test files`) with a zero exit status

### Non-Goals

- Re-running `fab upgrade-repo` — the upgrade already ran; re-running would be a no-op at best and is explicitly out of scope.
- Editing any hop source, config override, or version file by hand — hand-editing version files is not a supported upgrade path (intake § Why).
- Committing or opening a PR — those belong to later pipeline stages, not apply.

## Tasks

### Phase 1: Diff Verification

- [x] T001 [P] Verify `fab/.fab-version` diff is the single-line bump `2.16.0` → `2.16.4` via `git diff -- fab/.fab-version` <!-- R1 -->
- [x] T002 [P] Verify `fab/.kit-migration-version` diff is the single-line bump `2.15.8` → `2.16.4` via `git diff -- fab/.kit-migration-version` <!-- R1 -->
- [x] T003 [P] Verify `fab/project/config.yaml` diff is the fence-header comment only (`kit 2.16.0` → `kit 2.16.4`), no overridden field touched, via `git diff -- fab/project/config.yaml` <!-- R2 -->
- [x] T004 Verify no other tracked file is modified and the only untracked addition is the change folder, via `git status --short` and `git diff --stat` <!-- R3 -->

### Phase 2: Behavior Verification

- [x] T005 Run `go test ./...` from the module root (`src/`) and confirm every package passes with a zero exit status <!-- R4 -->

## Execution Order

- T001–T004 are independent diff inspections (all `[P]`, though T004 aggregates the whole tree) and can run in any order.
- T005 (test suite) is independent of the diff checks and may run alongside them.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `fab/.fab-version` and `fab/.kit-migration-version` diffs are exactly the recorded single-line version bumps (`2.16.0`→`2.16.4`, `2.15.8`→`2.16.4`)
- [x] A-002 R2: `fab/project/config.yaml` diff is limited to the reference-fence header comment (`kit 2.16.0`→`kit 2.16.4`) with no overridden field changed
- [x] A-003 R3: No hop source (`cmd/`, `internal/`) or other tracked file is modified; the only untracked addition is the change folder

### Scenario Coverage

- [x] A-004 R4: `go test ./...` passes for all packages with a zero exit status

### Code Quality

- [x] A-005 Pattern consistency: No source or config-override edits were introduced; the change follows the version-bump-only shape of the prior kit-upgrade precedent (PR #55)
- [x] A-006 No unnecessary duplication: No files re-generated or duplicated by hand; the upgrade output is taken verbatim from `fab upgrade-repo`

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- This is a verification-only apply — it introduces no *additional* working-tree changes beyond the already-applied kit upgrade (no source/config edits, no re-run of `fab upgrade-repo`, no commit). The pipeline still generates the change-folder artifacts (this plan, status files) as usual.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Apply verifies the existing working-tree diff rather than re-running `fab upgrade-repo` | Upgrade already ran; diff present and captured verbatim in the intake | S:90 R:90 A:95 D:90 |
| 2 | Certain | Scope is exactly the three `fab/` metadata files; no source edits | Diff inspected — version bumps and a regenerated fence comment only | S:90 R:90 A:95 D:90 |
| 3 | Confident | `go test ./...` green is the acceptance bar for "upgrade broke nothing" | Standard verification for a scaffolding-only change; `test_paths` configured in config.yaml | S:70 R:90 A:90 D:85 |

3 assumptions (2 certain, 1 confident, 0 tentative).
