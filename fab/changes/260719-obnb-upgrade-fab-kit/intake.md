# Intake: Upgrade fab kit to 2.16.4

**Change**: 260719-obnb-upgrade-fab-kit
**Created**: 2026-07-19

## Origin

> fab upgrade-repo, then drive the resulting change through the full pipeline with /fab-fff. If fab upgrade-repo produced no diff, stop — do not run /fab-fff and do not run /git-pr.

One-shot invocation. `fab upgrade-repo` was executed **before** this intake was created, and it produced a diff — so per the user's instruction the change proceeds through `/fab-fff`. The upgrade command's output:

```
Current version: 2.16.0
Target version: 2.16.4
Upgrading to 2.16.4...
Running sync...
Resolving kit v2.16.4 from cache...
fab/.kit-migration-version: OK (2.15.8)
.claude/settings.local.json: OK
.envrc: OK
.gitignore: OK
Claude Code: 34/34 (created 0, repaired 6, already valid 28)
Skipping OpenCode: opencode not found in PATH
Skipping Codex: codex not found in PATH
Skipping Gemini: gemini not found in PATH
Done.
Upgraded /home/sahil/code/sahil87/hop.worktrees/feisty-alpaca/fab/project/config.yaml (kit 2.16.4)

Updated: 2.16.0 -> 2.16.4
```

## Why

1. **Problem**: the repo's fab kit was pinned at 2.16.0 while the installed kit is 2.16.4. Skills, templates, and migrations drift from the kit the tooling actually runs, which is exactly the shim-staleness class of bug this toolkit tries to avoid.
2. **If not fixed**: future fab commands run against stale deployed skills/config fences; migrations accumulate and a later jump gets riskier.
3. **Approach**: `fab upgrade-repo` is the kit's own supported upgrade path (same as the previous upgrade, PR #55, which took the repo 2.13.4 → 2.16.0). No alternative was considered — hand-editing version files is explicitly not supported.

## What Changes

The upgrade has **already been applied to the working tree** by `fab upgrade-repo`. The resulting diff is version-bump-only — three files, one line each:

### fab/.fab-version

```diff
-2.16.0
+2.16.4
```

### fab/.kit-migration-version

```diff
-2.15.8
+2.16.4
```

Migrations were applied through 2.16.4 (previous marker was 2.15.8; the upgrade reported `fab/.kit-migration-version: OK (2.15.8)` before bumping).

### fab/project/config.yaml

Only the regenerated reference-fence header comment changes — no overridden fields were touched:

```diff
-# >>> fab reference (kit 2.16.0) >>> ---------------------------------------
+# >>> fab reference (kit 2.16.4) >>> ---------------------------------------
```

### Not in the git diff

`fab sync` also repaired 6 deployed Claude Code skill files under `.claude/skills/` (34/34 valid after sync); these are deployment artifacts, not tracked source, so they do not appear in the diff.

**No hop source code changes** (`cmd/`, `internal/`) are part of this change. The apply stage therefore **verifies rather than implements**: confirm the three-file diff is exactly as described above, and run the Go test suite (`go test ./...`) to confirm the upgraded kit scaffolding breaks nothing.

## Affected Memory

None — `fab/` is pipeline scaffolding (listed in `true_impact_exclude`), and no hop spec-level behavior changes. No memory files created, modified, or removed.

## Impact

- `fab/.fab-version`, `fab/.kit-migration-version`, `fab/project/config.yaml` — version metadata only
- No API, CLI-surface, dependency, or runtime behavior changes to hop
- Precedent: PR #55 (`chore: upgrade fab kit to 2.16.0`) shipped the identical shape of change

## Open Questions

None.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Apply stage verifies the existing working-tree diff instead of re-running the upgrade | `fab upgrade-repo` already ran in this session; its diff is present and captured verbatim above | S:90 R:90 A:95 D:90 |
| 2 | Certain | Scope is exactly the three files the upgrade touched; no source edits | Diff inspected — version bumps and a regenerated comment fence only | S:90 R:90 A:95 D:90 |
| 3 | Confident | No memory hydration needed (Affected Memory: none) | `fab/` is excluded scaffolding; no hop behavior changes; prior kit-upgrade PR #55 touched no memory | S:75 R:85 A:85 D:80 |
| 4 | Confident | `go test ./...` green is the acceptance bar for "upgrade broke nothing" | Standard verification for a scaffolding-only change; test_paths configured in config.yaml | S:70 R:90 A:90 D:85 |

4 assumptions (2 certain, 2 confident, 0 tentative, 0 unresolved).
