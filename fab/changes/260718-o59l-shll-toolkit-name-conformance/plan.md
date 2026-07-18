# Plan: shll Toolkit Name Conformance

**Change**: 260718-o59l-shll-toolkit-name-conformance
**Intake**: `intake.md`

## Requirements

<!-- Docs-prose conformance sweep: rename "sahil87 toolkit" → "shll toolkit"
     across user-visible prose, per the readme-extraction standard's revised
     canonical blockquote (sahil87/shll#56). Identifier occurrences
     (sahil87/tap, github.com/sahil87/…, sahil87/shll repo ref, GitHub-owner
     constants) are explicitly excluded. No behavior/code changes. -->

### Docs: README Head Conformance

#### R1: README blockquote matches the canonical readme-extraction line byte-identically
The `README.md` head blockquote (line 3) SHALL be replaced, byte-identical, with `> Part of the [shll toolkit](https://shll.ai) — see all projects there.`, and the mandated head order (H1 `# hop` → blockquote → contiguous badge line) SHALL be preserved.

- **GIVEN** `README.md:3` reads `> Part of [@sahil87's open source toolkit](https://shll.ai) — see all projects there.`
- **WHEN** the conformance sweep runs
- **THEN** line 3 reads exactly `> Part of the [shll toolkit](https://shll.ai) — see all projects there.`
- **AND** the H1 → blockquote → badges order is unchanged (single-line swap, no structural movement)

### Docs: README Body Prose

#### R2: README body prose uses the new toolkit name
The `sahil87 toolkit` / `sahil87 tool(s)` prose in `README.md` body SHALL be renamed to `shll toolkit` / `shll tool(s)`, touching only the prose phrase and leaving surrounding identifiers (URLs, formula names) untouched.

- **GIVEN** `README.md:15` reads `…To install the entire sahil87 toolkit instead:` and `README.md:101` reads `> 💡 Have other sahil87 tools? …`
- **WHEN** the sweep runs
- **THEN** line 15 reads `…To install the entire shll toolkit instead:` and line 101 reads `> 💡 Have other shll tools? …`
- **AND** the `github.com/sahil87/shll…` link in line 101 is unchanged

### Docs: Site Install Guide

#### R3: install.md heading uses the new toolkit name
The `docs/site/install.md` heading (line 58) SHALL rename `multiple sahil87 tools` → `multiple shll tools`.

- **GIVEN** `docs/site/install.md:58` reads `### One-shot wiring for multiple sahil87 tools`
- **WHEN** the sweep runs
- **THEN** line 58 reads `### One-shot wiring for multiple shll tools`
- **AND** the bare "toolkit" mention on line 60 ("several tools from the toolkit") is left untouched (not part of the sweep)

### Governance: Constitution Toolkit Standards Article

#### R4: constitution Toolkit Standards clause uses the new name, with a governance bump
The `fab/project/constitution.md` Toolkit Standards article (line 33) SHALL rename only the opening clause `part of the sahil87 toolkit` → `part of the shll toolkit`, leaving the later `sahil87/shll` canonical-source repo reference untouched; and the governance line SHALL bump `Version` 1.2.0 → 1.2.1 (patch — cosmetic wording), with `Last Amended` remaining `2026-07-18`.

- **GIVEN** line 33 opens `This tool is part of the sahil87 toolkit and MUST conform…` and later reads `the canonical sources are the sahil87/shll repository's docs/site/standards/ tree`
- **WHEN** the sweep runs
- **THEN** line 33 opens `This tool is part of the shll toolkit and MUST conform…`
- **AND** the `sahil87/shll` repository reference later in the same paragraph is unchanged
- **AND** the governance line reads `**Version**: 1.2.1 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-07-18`

### Docs: Backlog Entry

#### R5: backlog item [qner] prose uses the new toolkit name
The `fab/backlog.md` `[qner]` entry (line 15) SHALL rename `the sahil87 toolkit standards` → `the shll toolkit standards`, changing nothing else in that entry (the `260717-fcvp` change-folder ref and PR references are identifiers and stay).

- **GIVEN** `fab/backlog.md:15` contains `POINTS AT the sahil87 toolkit standards`
- **WHEN** the sweep runs
- **THEN** it contains `POINTS AT the shll toolkit standards`
- **AND** the `260717-fcvp` reference and `PR #53` reference in the same entry are unchanged

### Verification: Suite Green + No Residual Prose

#### R6: the Go test suite stays green and no old-name prose remains outside archives
`go test ./...` from `src/` SHALL pass (notably the skill drift-guard and help-dump envelope tests, which are unaffected by construction — no skill-bundle or Go-string edits), and a repo-wide grep SHALL find no `sahil87 toolkit` / `sahil87 tool` / `@sahil87's` prose outside `fab/changes/` archives.

- **GIVEN** the sweep edits are applied
- **WHEN** `go test ./...` runs from `src/` and the residual-prose grep runs
- **THEN** all tests pass
- **AND** the grep returns zero matches outside `fab/changes/`

### Non-Goals

- Go source / CLI user-visible strings — no occurrences; help-dump JSON envelope (`{tool, version, schema_version, root}`) and `schema_version` untouched (text-only sweep does not reach code).
- `docs/site/skill.md` and the embedded `src/cmd/hop/skill.md` — carry no old-name prose; no `just sync-skill` re-run; drift-guard test unaffected.
- Identifiers: `sahil87/tap` formula names, `github.com/sahil87/…` / `raw.githubusercontent.com/sahil87/…` URLs, the constitution's `sahil87/shll` repo reference, GitHub-owner constants.
- Historical artifacts: `fab/changes/` archives and `docs/memory/**` prose naming past change folders.
- Bare "toolkit" mentions without the old owner prefix (already conformant).

## Tasks

### Phase 2: Core Implementation

- [x] T001 [P] Replace `README.md:3` blockquote with the byte-identical line `> Part of the [shll toolkit](https://shll.ai) — see all projects there.`, preserving H1 → blockquote → badges order <!-- R1 -->
- [x] T002 [P] Rename README body prose: `README.md:15` "the entire sahil87 toolkit" → "the entire shll toolkit"; `README.md:101` "Have other sahil87 tools?" → "Have other shll tools?" (leave the github.com/sahil87/shll link intact) <!-- R2 -->
- [x] T003 [P] Rename `docs/site/install.md:58` heading "multiple sahil87 tools" → "multiple shll tools" (leave line 60 bare "toolkit" untouched) <!-- R3 -->
- [x] T004 [P] Edit `fab/project/constitution.md:33` opening clause "part of the sahil87 toolkit" → "part of the shll toolkit" (leave the later `sahil87/shll` repo reference); bump governance line Version 1.2.0 → 1.2.1, Last Amended stays 2026-07-18 <!-- R4 -->
- [x] T005 [P] Rename `fab/backlog.md:15` `[qner]` prose "the sahil87 toolkit standards" → "the shll toolkit standards" (leave `260717-fcvp` and PR refs) <!-- R5 -->

### Phase 3: Verification

- [x] T006 Run `go test ./...` from `src/` and confirm green (esp. skill drift-guard + help-dump envelope tests) <!-- R6 -->
- [x] T007 Run a repo-wide grep for `sahil87 toolkit` / `sahil87 tool` / `@sahil87's` prose and confirm zero matches outside `fab/changes/` <!-- R6 -->

## Execution Order

- T001–T005 are independent single-line prose edits across distinct files — all `[P]`, any order.
- T006 and T007 run after T001–T005 complete (verification of the applied edits).

## Acceptance

### Functional Completeness

- [x] A-001 R1: `README.md:3` reads exactly `> Part of the [shll toolkit](https://shll.ai) — see all projects there.` with head order (H1 → blockquote → badges) intact
- [x] A-002 R2: `README.md:15` reads "…the entire shll toolkit instead:" and `README.md:101` reads "> 💡 Have other shll tools? …" with the github.com/sahil87/shll link unchanged
- [x] A-003 R3: `docs/site/install.md:58` reads "### One-shot wiring for multiple shll tools"; line 60 bare "toolkit" untouched
- [x] A-004 R4: `fab/project/constitution.md:33` opens "This tool is part of the shll toolkit…"; the `sahil87/shll` repo ref later in the paragraph is unchanged; governance line reads `**Version**: 1.2.1 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-07-18`
- [x] A-005 R5: `fab/backlog.md:15` reads "POINTS AT the shll toolkit standards"; `260717-fcvp` and PR refs unchanged

### Behavioral Correctness

- [x] A-006 R6: `go test ./...` from `src/` passes (skill drift-guard and help-dump envelope tests green)

### Scenario Coverage

- [x] A-007 R6: repo-wide grep for `sahil87 toolkit` / `sahil87 tool` / `@sahil87's` returns zero matches outside `fab/changes/`

### Removal Verification

- [x] A-008 R1: no `@sahil87's open source toolkit` / `sahil87 toolkit` / `sahil87 tool(s)` prose remains in `README.md`, `docs/site/install.md`, `fab/project/constitution.md` (Toolkit Standards clause), or `fab/backlog.md`

### Edge Cases & Error Handling

- [x] A-009 R4: exclusion boundary honored — no identifier occurrence changed anywhere (`sahil87/tap`, `github.com/sahil87/…`, `raw.githubusercontent.com/sahil87/…`, `sahil87/shll` repo ref, GitHub-owner constants)

### Code Quality

- [x] A-010 Pattern consistency: edits are minimal single-phrase swaps that match the surrounding prose voice and markdown structure; no reflow or structural movement
- [x] A-011 No unnecessary duplication: no new files or content introduced — pure in-place prose substitution

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- `change_type: docs` → review skips the parsimony / deletion-candidate pass; no `## Deletion Candidates` section is generated.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Blockquote replaced with the exact byte-identical line from the intake, keeping H1 → blockquote → badges order | Intake pins the target line byte-for-byte and the precondition (`shll standards readme-extraction`) confirmed it matches the canonical source; head order already in place | S:95 R:90 A:100 D:95 |
| 2 | Certain | Sweep touches exactly 4 files / 6 sites; no Go strings, goldens, skill-bundle sync, or `docs/memory/**` edits | Repo-wide grep at apply entry reproduced the intake's scope exactly — 6 matches, all outside `fab/changes/`; `src/` and `docs/site/skill.md` clean | S:90 R:85 A:95 D:90 |
| 3 | Certain | Constitution governance bump Version 1.2.0 → 1.2.1, Last Amended stays 2026-07-18 | Intake mandates the bump; a wording-only edit is a patch-level amendment by semver; Last Amended already reads today's date | S:60 R:90 A:85 D:80 |
| 4 | Certain | Include `fab/backlog.md:15` and exclude identifier/URL/archive occurrences | Intake's exclusion list and inclusion of the forward-looking backlog entry are explicit and byte-scoped; boundary is unambiguous | S:70 R:90 A:90 D:85 |

4 assumptions (4 certain, 0 confident, 0 tentative).
