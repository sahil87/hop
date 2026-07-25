# Plan: Streamline README Onboarding

**Change**: 260724-hwxt-streamline-readme-onboarding
**Intake**: `intake.md`

## Requirements

### Docs: Reload-your-shell step

#### R1: README install block gains a reload instruction
README `## Install` MUST, immediately after the 3-line install block and its explanatory paragraph, instruct the user to open a new terminal or `exec zsh` (bash: `exec bash`), with a clause stating why: `shll shell-setup` edits the rc file, so the shim (the `hop` function, `h` alias, completion) only exists in shells started afterwards. The instruction MAY be folded into the verify beat (R3) as its lead-in. One or two lines — no new section.

- **GIVEN** a new user who has just pasted the README's 3-line install block
- **WHEN** they read the next lines of `## Install`
- **THEN** they are told to reload their shell (new terminal / `exec zsh`) before typing `h` or `hop <name>`
- **AND** the reason (rc-file edit only affects new shells) is stated in a clause

#### R2: install.md gains reload beats in TL;DR and §2
`docs/site/install.md` MUST mention the reload at both touchpoints: (a) `## TL;DR` — a line in or right after the code block making the reload the fourth beat of the happy path; (b) `## 2. Wire the shell shim` — one sentence after the `shll shell-setup` paragraph noting the wiring takes effect in new shells only (reload or open a new terminal), worded to cover §2 as a whole including the from-source manual-eval subsection.

- **GIVEN** a reader following install.md's TL;DR happy path
- **WHEN** they finish the code block
- **THEN** the next beat tells them to open a new terminal or `exec zsh`
- **GIVEN** a reader in §2 (either the shll path or the manual-eval path)
- **WHEN** they finish wiring the shim
- **THEN** one sentence tells them the wiring takes effect in new shells only

### Docs: First-success verify moment

#### R3: README gains a two-line verify beat
README `## Install` MUST, right after the reload instruction, present a tight "you should see" verify moment: `hop ls` (lists every repo from `hop.yaml` — proves the config bootstrap worked) and `h <partial>` (substring-match and cd — proves the shim works). A couple of lines, not a new tour. `docs/site/install.md` MAY mirror this in one line where it reads naturally (e.g. `## Next steps`) but MUST NOT grow a parallel tour — README is the primary site for this beat.

- **GIVEN** a user who has installed, bootstrapped, and reloaded their shell
- **WHEN** they read the verify beat and run the two commands
- **THEN** `hop ls` output confirms the bootstrap and `h <partial>` cd-ing confirms the shim
- **AND** install.md's mirror (if any) is a single line, not a duplicated walkthrough

### Docs: Dedup trim of README setup sections

#### R4: README `## Shell integration` and `## First run` shrink to pointers with zero information loss
README `## Shell integration` and `## First run` MUST shrink to a short paragraph each (or one merged short section — apply's choice) stating the one-line essence and pointing to `docs/site/install.md` for depth, the link written naturally as `docs/site/<path>.md`. Before trimming, apply MUST re-verify the intake's content-parity map line-by-line; any content found to exist ONLY in the README MUST be moved INTO install.md, never deleted. README-internal references (e.g. `## Gotchas` mentions the shim; the `[Gotchas](#gotchas)` cross-link) MUST still read coherently after the trim.

- **GIVEN** the intake's content-parity map (README setup facts → install.md §2–§4 locations)
- **WHEN** the two README sections are trimmed
- **THEN** every fact removed from the README exists in install.md (pre-existing or moved there in this change)
- **AND** each trimmed section links to `docs/site/install.md`
- **AND** README-internal cross-references still resolve and read coherently

### Docs: Toolkit standards conformance

#### R5: All edits conform to the readme-extraction standard
All edits MUST conform to `shll standards readme-extraction`: README head (H1 → toolkit blockquote → badges → prose) and tail structure untouched; links from README into the published set written naturally as `docs/site/<path>.md`; links to anything outside the published set absolute `https://…`; no relative images; no `docs/site/` links behind badges or reference-style definitions; no `#gh-*-mode-only` fragments; README keeps its hub cross-links (the `## Reference` section and the absolute `https://shll.ai/hop/commands/` URL); install.md keeps relative links only to other `docs/site/` pages.

- **GIVEN** the final state of `README.md` and `docs/site/install.md`
- **WHEN** the standard's conformance greps run (`](./`, `](../`, `](docs/` targets; relative images; reserved names)
- **THEN** every relative target from README points into `docs/site/`, every relative target inside install.md stays inside `docs/site/`, and no violations are found

### Non-Goals

- The shields.io badge "grammar diagram" rows in `## The mental model` — explicitly out of scope
- Any change to the `-h` help text — just shipped in change 3j8c / PR #65
- The shll.ai overview page — separate repo
- Go code, tests, CI — docs-only change

## Tasks

### Phase 1: Setup

- [x] T001 Re-verify the intake's content-parity map line-by-line: read README `## Shell integration` + `## First run` against `docs/site/install.md` §2–§4; list any README-unique facts that must move into install.md (intake-time read found none — confirm) <!-- R4 -->

### Phase 2: Core Implementation

- [x] T002 `README.md` `## Install`: add the reload instruction (new terminal / `exec zsh`, with the rc-file "why" clause) and the two-line verify beat (`hop ls`, `h <partial>`) immediately after the install block's explanatory paragraph, before the whole-toolkit alternative <!-- R1 --> <!-- R3 -->
- [x] T003 [P] `docs/site/install.md`: add the reload beat to `## TL;DR` (line right after the code block), one reload sentence in `## 2. Wire the shell shim` after the `shll shell-setup` paragraph (covering §2 as a whole), and a one-line verify mirror in `## Next steps` <!-- R2 --> <!-- R3 -->
- [x] T004 `README.md`: trim `## Shell integration` and `## First run` to a short paragraph each with a natural `docs/site/install.md` pointer, keeping the `[Gotchas](#gotchas)` cross-link coherent; move any T001-found README-unique facts into install.md first <!-- R4 -->

### Phase 3: Integration & Edge Cases

- [x] T005 Run the readme-extraction conformance greps over `README.md` and `docs/site/install.md` (relative-link targets `](./` `](../` `](docs/`, relative images, `#gh-*-mode-only`, reserved page names) and re-read both files' final sections for coherence, reading order, and intact hub links <!-- R5 -->

## Execution Order

- T001 blocks T004 (the parity re-verification gates the trim)
- T002 and T003 are independent of each other and of T004
- T005 runs last, after all edits

## Acceptance

### Functional Completeness

- [x] A-001 R1: README `## Install` contains a reload instruction (new terminal / `exec zsh`, bash variant noted) with the rc-file reason, placed after the install block's explanatory paragraph
- [x] A-002 R2: install.md `## TL;DR` carries the reload as the fourth beat of the happy path, and §2 carries one sentence noting the wiring takes effect in new shells only
- [x] A-003 R3: README `## Install` contains the two-line verify beat (`hop ls` proving the bootstrap, `h <partial>` proving the shim) right after the reload instruction
- [x] A-004 R4: README `## Shell integration` and `## First run` are each a short paragraph pointing to `docs/site/install.md`; the ~27 lines of duplicated setup depth are gone from the README

### Behavioral Correctness

- [x] A-005 R4: Every fact removed from the two README sections exists in install.md (parity map re-verified line-by-line; anything unique was moved, not deleted)
- [x] A-006 R3: install.md's verify mirror (if present) is a single natural line — no parallel tour was built

### Scenario Coverage

- [x] A-007 R1: A new user pasting the README install block and following the very next lines lands on a working `h <partial>` without hitting "command not found"
- [x] A-008 R4: README-internal references (`[Gotchas](#gotchas)`, the `## Gotchas` shim mentions) still resolve and read coherently after the trim

### Edge Cases & Error Handling

- [x] A-009 R2: The §2 reload sentence covers the from-source manual-eval path too (an rc-file edit doesn't affect the running shell either way)

### Code Quality

- [x] A-010 Pattern consistency: New prose matches the existing README voice (tight, imperative, comment-annotated code blocks) and install.md's walkthrough tone
- [x] A-011 No unnecessary duplication: The trim removes the near-verbatim duplication of install.md §2–§4; no new duplication is introduced (verify beat lives primarily in README, mirrored at most one line in install.md)

### Security

- [x] A-012 **N/A**: docs-only prose change — no security surface

### Standards Conformance

- [x] A-013 R5: Conformance greps pass — README relative links all point into `docs/site/`, install.md relative links stay inside `docs/site/`, everything else absolute, no relative images, no `#gh-*-mode-only`, no reserved page names, README head/tail structure and hub links (including `https://shll.ai/hop/commands/`) intact

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Keep two separate short README sections (`## Shell integration`, `## First run`) rather than merging into one | Intake explicitly offers both shapes; two headings preserve the existing reading order and scannability with minimal structural disruption | S:70 R:85 A:80 D:65 |
| 2 | Confident | Fold the reload instruction into the verify beat's lead-in inside the existing explanatory paragraph, followed by a two-line verify code block, placed before the whole-toolkit alternative | Intake explicitly permits folding ("can be folded into the verify step below as its first line"); the happy path should complete before the alternative install is offered | S:75 R:85 A:80 D:70 |
| 3 | Confident | install.md TL;DR reload is a prose line right after the code block, not a fourth command inside it | `exec zsh` inside a paste block would replace the shell mid-paste and is shell-specific; a prose line also covers "open a new terminal" | S:70 R:90 A:85 D:75 |
| 4 | Confident | The verify mirror in install.md is one clause in `## Next steps` ("in a fresh shell, `hop ls` lists… `h <partial>` cds…") | Intake names `## Next steps` as the natural spot; keeps README primary for the beat | S:70 R:90 A:80 D:70 |
| 5 | Confident | `h web` (→ `webapp`) is the concrete verify example | Matches the README's existing running example (`h web<TAB>`, `hop webapp`) throughout | S:75 R:95 A:85 D:80 |

5 assumptions (0 certain, 5 confident, 0 tentative).
