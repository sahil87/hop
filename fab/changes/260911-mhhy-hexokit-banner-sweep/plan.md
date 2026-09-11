# Plan: HexoKit banner sweep (hop)

**Change**: 260911-mhhy-hexokit-banner-sweep
**Intake**: `intake.md`

## Requirements

### Docs: README toolkit banner

#### R1: Revised canonical blockquote
`README.md` line 3 MUST be the byte-exact revised canonical toolkit blockquote from the C1 branch of the `readme-extraction` standard (`sahil87/shll` PR #98, `docs/site/standards/readme-extraction.md` rule 1), including the em-dash:

```markdown
> Part of [HexoKit](https://hexokit.com) — see all projects there.
```

- **GIVEN** `README.md` currently carries `> Part of the [shll toolkit](https://shll.ai) — see all projects there.` on line 3
- **WHEN** the change is applied
- **THEN** line 3 reads exactly `> Part of [HexoKit](https://hexokit.com) — see all projects there.`
- **AND** the old line no longer appears anywhere in `README.md`

#### R2: README head order and diff scope preserved
The README head MUST keep the standard's rule-1 order — `# hop` H1, blank line, the blockquote, blank line, the single badge line, blank line, the tagline paragraph — and no other line of `README.md` SHALL change. No other tracked file outside `fab/changes/260911-mhhy-hexokit-banner-sweep/` SHALL change.

- **GIVEN** the applied change
- **WHEN** `git diff origin/main --stat -- . ':!fab/changes'` is run
- **THEN** it lists only `README.md` with 1 insertion and 1 deletion

### Docs: run-kit product-mention sweep

#### R3: No present-tense "run-kit" product prose on live surfaces
Live surfaces (`README.md`, `docs/site/**`, `src/`, `scripts/`, `.github/`, `justfile`) MUST contain zero occurrences of `run-kit`/`runkit` (case-insensitive). Occurrences in `docs/specs/**` and `docs/memory/**` MUST each be a reference to the `sahil87/run-kit` repository or its release-workflow files (repo links, kept until plan row R2; memory prose is the historical tier, D11) and MUST be left unchanged.

- **GIVEN** the applied change
- **WHEN** `grep -rniE "run-kit|runkit" README.md docs/site src scripts .github justfile LICENSE` is run
- **THEN** it returns no matches
- **AND** `git diff origin/main --stat -- docs/specs docs/memory` is empty

### Non-Goals
- "shll toolkit" phrases and `shll.ai` URLs outside the blockquote — owned by plan rows X4 and X2 (D14, D4).
- Badges, `sahil87/hop` links, code, help text, `hop skill` bundle — untouched.
- Substrate identifiers (`rk`, `Formula/rk.rb`) — never renamed under the plan.
- The plan doc's C7 row in the run-kit repo — cross-repo operator edit, not part of this diff.

## Tasks

### Phase 2: Core Implementation

- [x] T001 Replace line 3 of `README.md` with `> Part of [HexoKit](https://hexokit.com) — see all projects there.` (byte-exact, em-dash) <!-- R1 -->
- [x] T002 Verify the README head order (H1 → blockquote → badge line → tagline) and that `git diff origin/main --stat -- . ':!fab/changes'` lists only `README.md` (+1/−1) <!-- R2 -->
- [x] T003 [P] Run the sweep grep over live surfaces and confirm zero hits; confirm `docs/specs` and `docs/memory` are absent from the diff <!-- R3 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `README.md` line 3 is exactly `> Part of [HexoKit](https://hexokit.com) — see all projects there.`
- [x] A-002 R2: README head order is H1 → blockquote → badge line → tagline, with no other README line changed
- [x] A-003 R3: No `run-kit`/`runkit` match on any live surface (README, docs/site, src, scripts, .github, justfile, LICENSE)

### Behavioral Correctness

- [x] A-004 R1: The old line `> Part of the [shll toolkit](https://shll.ai) — see all projects there.` appears nowhere in `README.md`

### Scenario Coverage

- [x] A-005 R2: `git diff origin/main --stat -- . ':!fab/changes'` shows only `README.md`, 1 insertion, 1 deletion

### Edge Cases & Error Handling

- [x] A-006 R3: `docs/specs/build-and-release.md`, `docs/memory/build/release-pipeline.md`, and `docs/memory/build/ci-pipeline.md` are unchanged (their `run-kit` repo references intentionally remain)

### Code Quality

- [x] A-007 Pattern consistency: the blockquote matches the standard's mandated line byte-for-byte (em-dash, link text, URL, trailing period); no ASCII `--` substitution
- [x] A-008 No unnecessary duplication: no second banner, comment, or note about the rebrand is added to the README

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Blockquote uses the em-dash form from the C1 branch, not the ASCII `--` in the invocation | Standard mandates "this exact line"; intake assumption 1 | S:90 R:95 A:95 D:90 |
| 2 | Certain | Sweep is a verified-negative: docs/specs + docs/memory `run-kit` hits are repo/workflow references and stay | Intake assumption 3; user instruction "repo links stay until R2"; D11 | S:85 R:90 A:90 D:85 |
| 3 | Certain | No tests to add or run — README-only prose change; CI `go test` unaffected | Change touches no Go source; `test-alongside` has nothing to pair with | S:85 R:100 A:95 D:95 |

3 assumptions (3 certain, 0 confident, 0 tentative).
