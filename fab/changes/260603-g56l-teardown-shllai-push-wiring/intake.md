# Intake: Tear down shll.ai help-dump push wiring

**Change**: 260603-g56l-teardown-shllai-push-wiring
**Created**: 2026-06-03
**Status**: Draft

## Origin

Initiated conversationally from a `/fab-discuss` session. The user pointed at an updated shll.ai contract spec — `sahil87/shll.ai` `docs/specs/help-dump-contract.md`, the "Pull Model (Transport Inversion, 2026-06-03)" / teardown-directive section — and asked to understand and act on it.

> There's an update in the way we integrate with shll.ai. [...] Tear down hop's shll.ai help-dump push wiring now that shll.ai pulls help via its own scheduled job (transport inversion, per shll.ai's help-dump-contract.md). Remove the "Dump help tree and PR to shll.ai" step in .github/workflows/release.yml (lines 138-191) including all SHLLAI_TOKEN usage. Remove the local scripts/help-dump.sh and the help-dump recipe in justfile (it should no longer be a CI/CD-style step). PRESERVE the help-dump command itself (src/cmd/hop/help_dump.go, its wiring in root.go) and its test coverage (src/cmd/hop/help_dump_test.go) — the command remains the contract surface and the tests keep it verified.

**Mode**: Conversational. The shll.ai spec was fetched and read; three decisions were settled with the user before this intake:

1. **Puller is live** — the user confirmed shll.ai's scheduled pull job is deployed and confirmed pulling hop. This satisfies the spec's hard guardrail ("Tool repos can delete [push wiring] … but only after the puller is live and proven"). Removal is therefore safe now.
2. **Local recipe** — remove the `scripts/help-dump.sh` + `justfile` `help-dump` recipe; it "shouldn't be a ci/cd step." The user's exact qualifier: *"Remove it - at max a test case that tests it. It shouldn't be ci/cd step."* The contract must stay **verified**, just not via a runnable script/recipe — and it already is, by the Go tests in `src/cmd/hop/help_dump_test.go`. So removing the script loses no coverage.
3. **Manual follow-up** — after merge, the maintainer deletes the `SHLLAI_TOKEN` GitHub repository secret (after confirming no other usage). This is a GitHub-UI action the agent cannot perform.

This change is the inverse of the `260602-jr5f-help-dump-shllai-json` change, which *added* the push wiring one day earlier. The `help-dump` producer command it introduced stays; only the push transport is removed.

## Why

**Problem.** shll.ai inverted its help-collection transport (dated 2026-06-03). It used to be **push**-based: each tool repo's CI ran `help-dump`, wrote `help/<tool>.json`, and opened an auto-merged PR into `sahil87/shll.ai`. shll.ai now **pulls** — its own scheduled job installs each tool and runs `help-dump` itself. With the puller live, hop's push wiring is now dead weight that is actively harmful:

1. **Cross-repo credential** — the push step needs `SHLLAI_TOKEN`, a `sahil87` PAT with `contents:write` + `pull-requests:write` on `sahil87/shll.ai`. A standing cross-repo write credential on the hop repo is attack surface with no remaining purpose. Removing it shrinks the blast radius (aligns with Constitution Principle I — Security First).
2. **Redundant work + PR noise** — with the puller live, the push step now races the pull: both produce `help/hop.json` on shll.ai. Every hop release would still open a downstream PR that the puller has made unnecessary.
3. **Coupling** — the push step couples hop's release pipeline to shll.ai's internals (`TRUSTED_AUTHOR` actor guard, `help-automerge.yml`, `validate-help.mjs`). The pull model deliberately eliminates this coupling; leaving the push wiring in place re-introduces what the inversion was designed to remove.

**Consequence of not acting.** The token lingers as unnecessary attack surface; releases keep opening redundant downstream PRs; hop stays coupled to shll.ai internals the contract no longer expects it to know.

**Why this approach.** The spec prescribes exactly this teardown and explicitly preserves the `help-dump` command as the singular contract surface. We follow the directive precisely: remove the transport, keep the producer. The only judgment call beyond the directive — what to do with the *local* helper script — was resolved with the user (remove it; tests retain verification).

## What Changes

This is a removal-only change to CI and local build ergonomics. **No Go source under `src/cmd/hop/` is modified** except — if needed — confirming the `help-dump` command and its tests are untouched. Four edit areas:

### A. Remove the push step from `.github/workflows/release.yml`

Delete the entire final step of the `release` job:

```yaml
      - name: Dump help tree and PR to shll.ai
        # Best-effort: this publishes the command reference downstream and must
        # never flip the release job red after the GitHub Release + Homebrew tap
        # steps have already succeeded. Merging is owned by shll.ai's
        # help-automerge.yml (gated on actor/content/schema) — we only open the PR.
        continue-on-error: true
        env:
          GH_TOKEN: ${{ secrets.SHLLAI_TOKEN }}   # sahil87 PAT: contents + pull-requests write on shll.ai
        run: |
          ... (dump → jq inject captured_at → jq -e validate → clone shll.ai →
               no-op guard → branch help-dump/hop-${version} → commit → push → gh pr create)
```

This is the only active `SHLLAI_TOKEN` reference in the workflow (confirmed by repo-wide grep: the only non-doc, non-`fab/` hit is `release.yml:145`). Removing this step removes all live token usage.

After removal, the `release` job drops from **8 steps to 7**: Checkout, Setup Go, Extract version, Cross-compile, Determine release-notes base tag, Create GitHub Release, Update Homebrew tap. The `permissions: contents: write` block stays unchanged (it was always scoped for the GitHub Release, not the shll.ai push — the push used the PAT, not `GITHUB_TOKEN`).

**Constraint:** do not touch any other step. The Homebrew tap step (`HOMEBREW_TAP_TOKEN`) and the GitHub Release step are unrelated and must remain byte-identical.

### B. Remove the local `help-dump` recipe and script

1. Delete the `justfile` recipe (and its preceding comment):

   ```just
   # Build hop and pretty-print its CLI help tree as JSON (the help/hop.json contract).
   help-dump:
       ./scripts/help-dump.sh
   ```

2. Delete `scripts/help-dump.sh` entirely.

`scripts/build.sh` is shared by the `build`/`local-install` recipes and **must stay**. Only the `help-dump` recipe and its dedicated script are removed. This is consistent with Constitution Principle V (thin justfile delegating to `scripts/`): we remove a recipe + its script together, not orphan one.

The contract remains exercised by `src/cmd/hop/help_dump_test.go` — the user's "at max a test case that tests it" condition is already met by existing tests; no new test is required, and none is added.

### C. Preserve the producer command and its tests (no-op, stated for the apply agent)

These files are the **contract surface** and MUST NOT be modified by this change:

- `src/cmd/hop/help_dump.go` — the hidden `hop help-dump` command, the `Doc`/`Node` envelope (`tool`, `version`, `captured_at` left empty by producer, `schema_version: 1`, `root`), the recursive cobra walk, and the `completion`/`help`/`Hidden` filters.
- `src/cmd/hop/root.go:132` — `newHelpDumpCmd()` wiring into the root command.
- `src/cmd/hop/help_dump_test.go` — all `TestHelpDump*` tests (envelope fields, version-not-hardcoded, child filtering, leaf serialization, hidden-from-`--help`).

The apply agent's job here is to *confirm* these are untouched and that `cd src && go test ./...` still passes after the removals (it should — none of the removed artifacts are imported by Go code).

### D. Update memory: `docs/memory/build/release-pipeline.md`

This is the only memory file with substantive push-wiring content. Required edits:

1. **Workflow steps list** — change "eight steps" to "seven steps"; delete step 8 ("Dump help tree and PR to shll.ai").
2. **Delete the entire "Help reference publish step (shll.ai)" section** — the multi-paragraph block describing the dump/inject/validate/PR flow, the ordering-is-load-bearing note, the `SHLLAI_TOKEN` secret paragraph, and the local-equivalent line.
3. **Setup checklist** — delete item 2 ("Provision `SHLLAI_TOKEN`"); renumber.
4. **Release-day runbook** — delete item 6 (verify the `help-dump/hop-<version>` PR on shll.ai); renumber.
5. **Cross-references** — drop any cross-ref that exists solely to point at the removed step.
6. **Add a short "Help reference (shll.ai)" note** recording the transport inversion: hop's CLI help is now **pulled** by shll.ai's scheduled job (which runs the published binary's `hop help-dump`); hop no longer pushes. This preserves the institutional memory of *why* the wiring is gone, citing the 2026-06-03 inversion and that the `help-dump` command remains the contract surface.

`docs/memory/build/local.md` — review only; its illustrative justfile snippet does not currently list the `help-dump` recipe, so likely **no change** (confirm during hydrate).
`docs/memory/cli/subcommands.md` — the `hop help-dump` contract documentation **stays** (the command stays). Its mention that "CI generates the canonical artifact" should be softened to reflect the pull model during hydrate.

## Affected Memory

- `build/release-pipeline`: (modify) Remove the entire shll.ai push step from the workflow-steps list (8→7 steps), the "Help reference publish step (shll.ai)" section, the `SHLLAI_TOKEN` setup-checklist item, and the runbook PR-verification item. Add a brief note recording the 2026-06-03 transport inversion (shll.ai now pulls; hop no longer pushes; the `help-dump` command remains the contract surface).
- `cli/subcommands`: (modify) The `hop help-dump` contract section stays, but soften any "CI publishes/generates the canonical artifact" phrasing to reflect that shll.ai now pulls via the command rather than hop pushing.
- `build/local`: (modify, likely no-op) Confirm the local-build doc does not describe the removed `help-dump` recipe; adjust only if it does.

## Impact

**Code areas:**
- `.github/workflows/release.yml` — remove the final `Dump help tree and PR to shll.ai` step (the sole live `SHLLAI_TOKEN` consumer).
- `justfile` — remove the `help-dump` recipe.
- `scripts/help-dump.sh` — delete the file.

**Preserved (must not change):** `src/cmd/hop/help_dump.go`, `src/cmd/hop/root.go`, `src/cmd/hop/help_dump_test.go`.

**Secrets:** `SHLLAI_TOKEN` GitHub repository secret on `sahil87/hop` becomes unused after this merge. Deleting it is a **manual maintainer action** (GitHub UI) the agent cannot perform — flagged in the PR description. The repo-wide grep confirms no other active (non-doc, non-`fab/`) usage. `HOMEBREW_TAP_TOKEN` is unrelated and untouched.

**External systems:** `sahil87/shll.ai`'s pull job is now the source of hop's command reference. This change assumes that job is live (user-confirmed). No code in `shll.ai` is touched by this change.

**Dependencies:** none added or removed. No Go imports change.

**Constitution alignment:** Principle I (Security First — removes a standing cross-repo write credential), Principle V (Thin Justfile — removes a recipe + its script together, leaving `scripts/build.sh` intact), Principle VI (Minimal Surface Area — removes wiring, adds none).

## Open Questions

None blocking. All three judgment calls were resolved with the user during the discussion that preceded this intake (puller-live confirmation, local-recipe removal, manual secret deletion). The `build/local.md` review is a hydrate-time confirmation, not a blocking question.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | shll.ai's scheduled pull job is live and proven for hop, so removing the push wiring now satisfies the spec's "only after the puller is live and proven" guardrail | User explicitly confirmed "Yes, puller is live" in the preceding discussion | S:98 R:40 A:95 D:98 |
| 2 | Certain | Remove the local `scripts/help-dump.sh` and the `justfile` `help-dump` recipe; the contract stays verified by existing `help_dump_test.go` tests (no new test added) | User: "Remove it - at max a test case that tests it. It shouldn't be ci/cd step." Existing tests already satisfy the "at max a test case" condition | S:95 R:80 A:90 D:90 |
| 3 | Certain | Preserve `help_dump.go`, its `root.go` wiring, and `help_dump_test.go` unchanged — the command is the contract surface | Spec is explicit the command is preserved; user reiterated PRESERVE in the request | S:98 R:90 A:98 D:98 |
| 4 | Certain | Delete only the final release.yml step (the sole live `SHLLAI_TOKEN` consumer); leave Homebrew + GitHub Release steps byte-identical | Repo-wide grep shows `release.yml:145` is the only active token reference; user named the exact step/lines | S:95 R:75 A:95 D:95 |
| 5 | Confident | Update `docs/memory/build/release-pipeline.md` to reflect the removal (8→7 steps, drop the publish section/secret/runbook items) and record the transport inversion | Memory must track reality; this file is the only one with substantive push content. Standard hydrate scope | S:85 R:80 A:85 D:80 |
| 6 | Confident | The `SHLLAI_TOKEN` secret is deleted manually by the maintainer post-merge, flagged in the PR body | Agent cannot perform GitHub-UI secret deletion; user agreed to the manual follow-up | S:90 R:70 A:80 D:90 |
| 7 | Confident | Soften `cli/subcommands.md` "CI publishes the canonical artifact" phrasing to reflect the pull model, but keep the `help-dump` contract docs | The command stays; only the transport story changes. Low-risk doc accuracy edit at hydrate | S:80 R:85 A:80 D:75 |
| 8 | Tentative | `docs/memory/build/local.md` needs no change (its justfile snippet omits the `help-dump` recipe) | Inspection suggests no current reference, but confirm at hydrate; trivially reversible | S:70 R:90 A:75 D:70 |

8 assumptions (4 certain, 3 confident, 1 tentative, 0 unresolved).
