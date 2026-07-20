# Plan: Centralize Install Docs (Policy B)

**Change**: 260720-of6z-centralize-install-docs
**Intake**: `intake.md`

## Requirements

### Docs: install.md — shll.ai-pointed install section

#### R1: Replace the Homebrew install section with the shll.ai pointer
`docs/site/install.md` §1's "### Homebrew (macOS and Linux)" section MUST NOT carry the per-formula `brew install sahil87/tap/hop` instruction. It SHALL instead present the shll.ai curl bootstrap (`curl -fsSL https://shll.ai/install | sh -s -- hop`), note that it installs via Homebrew under the hood handling tap trust automatically (mirroring README's conformant wording), mention the full-toolkit variant (`curl -fsSL https://shll.ai/install | sh`), and point at https://shll.ai for the complete install story. The heading MUST no longer be brew-specific.

- **GIVEN** a user reading `docs/site/install.md` §1 "Install the binary"
- **WHEN** they follow the primary install path
- **THEN** they are given the shll.ai curl bootstrap, not a `brew install sahil87/tap/hop` line
- **AND** the section heading does not name Homebrew as the install method

#### R2: Fix the false wt-dependency claim in install.md §1
The sentence "The formula pulls in `wt` as a dependency … brew installs it for you" MUST be rewritten to state current truth: `wt` is a sibling tool that is NOT installed automatically (the `depends_on "wt"` formula edge was removed in PR #62); hop shells out to `wt list --json` for the `<name>/<wt>` suffix, so users who want worktree navigation install wt via `shll install wt` (or the full-toolkit bootstrap); bare `hop <name>` never touches wt. The usage content (what wt enables) SHALL be preserved.

- **GIVEN** the rewritten install section
- **WHEN** a user reads how hop relates to `wt`
- **THEN** no text claims the formula pulls wt in as a dependency
- **AND** the wt install pointer is `shll install wt` / the shll.ai bootstrap, not a per-formula brew line

#### R3: Repoint install.md's from-source wt line
`docs/site/install.md` §1 "From source" (line 27) MUST replace "install it separately: `brew install sahil87/tap/wt`, or build it from source the same way" with a shll.ai pointer ("install it separately via [shll.ai](https://shll.ai) (`shll install wt`), or build it from source the same way"). The from-source build instructions themselves (git clone / just install) SHALL stay intact.

- **GIVEN** the "From source" section
- **WHEN** a user needs wt after a from-source hop install
- **THEN** they are pointed at shll.ai / `shll install wt`, not `brew install sahil87/tap/wt`
- **AND** the git-clone/just-install steps are unchanged

### Docs: README.md — wt gotcha and install.md blurb

#### R4: Rewrite the README wt gotcha tail
README.md's gotcha bullet (line 250) MUST replace "The Homebrew formula pulls wt in as a dependency; for non-brew installs, `brew install sahil87/tap/wt` or build from source" with: wt is not installed automatically — install it via [shll.ai](https://shll.ai) (`shll install wt`) or build from source. The bullet's usage content (shells out to `wt list --json`, no state cached, bare queries never invoke wt) SHALL be kept verbatim.

- **GIVEN** the README Gotchas section
- **WHEN** a user reads the `<name>/<wt>` suffix bullet
- **THEN** the tail contains no dependency claim and no per-formula brew line, pointing at shll.ai instead
- **AND** the bullet's usage sentences are unchanged

#### R5: Reword the README fuller-guide blurb
README.md's line-88 blurb ("— Homebrew & from-source, shell integration in depth…") MUST be reworded to match the revised install.md (e.g. "install via shll & from-source…") so it no longer advertises a Homebrew section that no longer exists.

- **GIVEN** the "Other ways to install" section's fuller-guide pointer
- **WHEN** a user reads what the install guide covers
- **THEN** the blurb names the shll install path, not "Homebrew"

### Docs: workflows.md — wt gotcha

#### R6: Rewrite the workflows.md wt gotcha tail
`docs/site/workflows.md` line 151 MUST receive the same rewrite as R4: drop "Homebrew pulls `wt` in as a dependency; for non-brew installs, `brew install sahil87/tap/wt` or build from source" in favor of the shll.ai / `shll install wt` pointer, keeping the bullet's usage content.

- **GIVEN** the workflows.md Gotchas section
- **WHEN** a user reads the `<name>/<wt>` suffix bullet
- **THEN** the tail points at shll.ai (`shll install wt`) or build-from-source, with no dependency claim or per-formula brew line

### Docs: KEEP set preserved

#### R7: Audit-clean — all remaining brew/Homebrew mentions are in the KEEP set
After the edits, re-running `grep -rniE 'brew install|brew tap|shll install|shll\.ai|homebrew' README.md docs/site/` MUST show every remaining `brew`/Homebrew mention belonging to the intake's KEEP set: README's top Install section (conformant "via Homebrew, handling tap trust" wording), the `hop update` feature descriptions (README:76, install.md §5, skill.md:36), and the equivalent "under the hood" wording in the rewritten install.md. `shll install` / `shll.ai` matches are the Policy B pointers themselves and are expected. No file outside the intake's four-file scope SHALL be edited.

- **GIVEN** the post-edit tree
- **WHEN** the audit grep is re-run
- **THEN** no `brew install sahil87/tap/<tool>` line remains in README.md or docs/site/
- **AND** the `hop update`-via-Homebrew feature descriptions remain intact

### Non-Goals

- `docs/specs/` (release-pipeline machinery mentions of homebrew-tap) — outside the audited surface
- Any code, formula, or `hop update` behavior change
- Adding Policy-A error hints to hop's code for missing `wt`

## Tasks

### Phase 2: Core Implementation

- [x] T001 Rewrite `docs/site/install.md` §1: replace the "### Homebrew (macOS and Linux)" heading + `brew install sahil87/tap/hop` block with the shll.ai-pointed install ("### Via shll (macOS and Linux)", curl bootstrap, full-toolkit variant, shll.ai link) and rewrite the wt sentence to drop the false dependency claim <!-- R1, R2 -->
- [x] T002 [P] Edit `docs/site/install.md` line 27 (From source): repoint the separate-wt-install advice to [shll.ai](https://shll.ai) (`shll install wt`) <!-- R3 -->
- [x] T003 [P] Edit `README.md` line 250 gotcha bullet: replace the dependency-claim + `brew install sahil87/tap/wt` tail with the shll.ai (`shll install wt`) pointer, keeping usage content verbatim <!-- R4 -->
- [x] T004 [P] Edit `README.md` line 88 fuller-guide blurb: reword "Homebrew & from-source" to "install via shll & from-source" <!-- R5 -->
- [x] T005 [P] Edit `docs/site/workflows.md` line 151 gotcha bullet: same rewrite as T003 <!-- R6 -->

### Phase 3: Integration & Edge Cases

- [x] T006 Re-run the audit grep (`grep -rniE 'brew install|brew tap|shll install|shll\.ai|homebrew' README.md docs/site/`) and verify every remaining brew/Homebrew match is in the KEEP set; verify no files outside the four-file scope changed (`git status`) <!-- R7 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `docs/site/install.md` §1 leads with the shll.ai curl bootstrap (`curl -fsSL https://shll.ai/install | sh -s -- hop`), mentions the full-toolkit variant and shll.ai, and carries no `brew install sahil87/tap/hop` line or brew-specific heading
- [x] A-002 R3: install.md's From source section points wt installs at shll.ai (`shll install wt`), with git-clone/just-install steps unchanged
- [x] A-003 R4: README's `<name>/<wt>` gotcha bullet points at shll.ai (`shll install wt`) or build-from-source, with its usage sentences kept verbatim
- [x] A-004 R5: README's fuller-guide blurb no longer says "Homebrew & from-source"
- [x] A-005 R6: workflows.md's `<name>/<wt>` gotcha bullet carries the same shll.ai rewrite

### Behavioral Correctness

- [x] A-006 R2: No text in README.md or docs/site/ claims the Homebrew formula pulls `wt` in as a dependency (the claim is false since PR #62)

### Removal Verification

- [x] A-007 R7: The audit grep finds zero `brew install sahil87/tap/` lines in README.md and docs/site/; every remaining brew/Homebrew mention is a KEEP-set item (README top Install wording, `hop update` feature descriptions in README:76 / install.md §5 / skill.md:36, and the rewritten section's "via Homebrew under the hood" prose)

### Scenario Coverage

- [x] A-008 R7: `git status` shows only `docs/site/install.md`, `README.md`, and `docs/site/workflows.md` modified; `docs/site/skill.md`, `docs/specs/`, and all code/formula files untouched

### Code Quality

- [x] A-009 Pattern consistency: Rewritten prose matches the surrounding docs' voice and formatting conventions (heading style, backticked commands, link style)
- [x] A-010 No unnecessary duplication: install.md's new section mirrors (not contradicts) README's existing conformant Install wording

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | New install.md §1 heading is "### Via shll (macOS and Linux)" | Intake suggests exactly this phrasing as its example ("or similar"); only the non-brew-specific property is mandated | S:70 R:95 A:85 D:70 |
| 2 | Confident | Rewritten wt prose in install.md keeps the sibling-tool explanation in the same paragraph position, ending the section rather than adding a new subsection | Intake dictates content, not layout; preserving the section's existing shape is the lowest-risk mechanical rewrite | S:65 R:95 A:85 D:75 |
| 3 | Certain | Gotcha-bullet rewrites (README:250, workflows.md:151) change only the final sentence; all preceding usage sentences stay byte-identical | Intake says "keep the bullet's usage content verbatim" | S:90 R:95 A:95 D:90 |

3 assumptions (1 certain, 2 confident, 0 tentative).
