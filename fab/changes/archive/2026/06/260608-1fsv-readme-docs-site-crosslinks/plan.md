# Plan: README → docs/site cross-links + command-reference link

**Change**: 260608-1fsv-readme-docs-site-crosslinks
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### README: in-context cross-links

#### R1: Install section points to the install guide
The `## Install` area of `README.md` SHALL contain a plain inline link to `docs/site/install.md`, written in the contract's natural relative form so the site auto-rewrites it to `/tools/hop/install`.

- **GIVEN** a reader in the install area of the README
- **WHEN** they want the fuller setup walkthrough
- **THEN** an in-context pointer links to `docs/site/install.md`
- **AND** the link is plain inline (not behind a badge/thumbnail, not a reference-style definition), so the site rewrites it rather than 404-ing

#### R2: Grammar section points to the workflows deep-dive
The `## Grammar at a glance` section SHALL contain a plain inline link to `docs/site/workflows.md` in natural relative form (auto-rewritten to `/tools/hop/workflows`).

- **GIVEN** a reader who has seen the grammar table
- **WHEN** they want worked examples and the dispatch-model deep-dive
- **THEN** an in-context pointer links to `docs/site/workflows.md`
- **AND** the link is plain inline (same rule-6 trap avoidance as R1)

#### R3: Reference section links the online command reference
The `## Reference` section SHALL link the published command reference at `https://shll.ai/tools/hop/commands/` as an absolute `https://…` URL, alongside the existing `hop --help` mention.

- **GIVEN** a reader who wants the full command reference rendered online
- **WHEN** they read `## Reference`
- **THEN** an absolute link to `https://shll.ai/tools/hop/commands/` is present
- **AND** because the link leaves the rendered set, it is absolute-by-author (contract rule 6), not relative

#### R4: Contract conformance preserved
The edits SHALL NOT regress the shll.ai README-extraction contract conformance established in PR #42 (`260608-0qwc-shll-readme-contract`).

- **GIVEN** the README after these edits
- **WHEN** the contract Verify checklist is re-run
- **THEN** head order (H1 → toolkit blockquote → badges → prose) is unchanged
- **AND** the only new relative link targets point INTO `docs/site/`; the command link is absolute
- **AND** no new images, mermaid fences, `#gh-*-mode-only` fragments, or tail-denylist headings are introduced

### Non-Goals

- No changes to `docs/site/**` content — only links *into* it from the README.
- No replacement of the existing `## Reference` discovery links (PR #42) — they are kept ("also", per the request).
- No code, CI, `docs/specs/**`, or `docs/memory/**` changes.

### Design Decisions

1. **Single install pointer at the install-area top**: place one pointer covering Install + Shell integration + First run — *Why*: `docs/site/install.md` spans the whole setup arc, so one pointer avoids three near-identical links — *Rejected*: a pointer under each of the three subsections (clutter, redundant).
2. **Augment the `hop --help` bullet in place** rather than a separate bullet — *Why*: the online reference is the companion to the local `--help`; one bullet reads cleanest and matches the user's "(also)" phrasing — *Rejected*: a standalone bullet (visually separates two facets of the same thing).
3. **Blockquote pointers** for R1/R2 — *Why*: visually distinguishes the "go deeper" pointer from body prose; renders as a normal link on GitHub and after the site's relative-link rewrite — *Rejected*: inline parenthetical (less scannable).

## Tasks

### Phase 1: README edits

- [x] T001 Add a blockquote pointer to `docs/site/install.md` after the From-source block, before `## Shell integration`, in `README.md` <!-- R1 -->
- [x] T002 Add a blockquote pointer to `docs/site/workflows.md` after the dispatch-model paragraph, before `## Config schema`, in `README.md` <!-- R2 -->
- [x] T003 Augment the `hop --help` bullet in `## Reference` with an absolute link to `https://shll.ai/tools/hop/commands/`, keeping the existing `docs/site/` discovery bullets unchanged, in `README.md` <!-- R3 -->

### Phase 2: Verify

- [x] T004 Re-run the contract Verify checklist (head order; relative targets only into `docs/site/`; command link absolute; no images/mermaid/gh-mode/denylist headings introduced) against `README.md` <!-- R4 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `README.md` install area contains a plain inline link to `docs/site/install.md` (natural relative form)
- [x] A-002 R2: `README.md` `## Grammar at a glance` contains a plain inline link to `docs/site/workflows.md` (natural relative form)
- [x] A-003 R3: `README.md` `## Reference` contains an absolute `https://shll.ai/tools/hop/commands/` link alongside the `hop --help` mention

### Behavioral Correctness

- [x] A-004 R1: The install pointer is in a blockquote / plain inline form — not badge-wrapped, not a reference-style definition (site rewrites it to `/tools/hop/install`)
- [x] A-005 R2: The workflows pointer is plain inline (site rewrites it to `/tools/hop/workflows`)
- [x] A-006 R3: The command-reference link is absolute (leaves the rendered set per rule 6), not relative

### Scenario Coverage

- [x] A-007 R4: After the edits, a grep of `README.md` link/image targets shows the only relative targets are `docs/site/install.md` and `docs/site/workflows.md`; all others are `https://…` or `#…`

### Edge Cases & Error Handling

- [x] A-008 R4: No new image, ` ```mermaid ` fence, `#gh-dark-mode-only` / `#gh-light-mode-only` fragment, or tail-denylist heading (Contributing/Development/Building/License/Acknowledgements) was introduced; README head order (H1 → blockquote → badges → prose) is unchanged

### Code Quality

- [x] A-009 Pattern consistency: The new pointers match the README's existing voice and the existing `## Reference` discovery-link style
- [x] A-010 No unnecessary duplication: The existing `## Reference` discovery links are reused/kept, not duplicated; the install pointer is placed once for the whole setup arc

## Notes

- Check items as you review: `- [x]`
- This is a `docs` change — verification is grep-based link/structure audit; no automated tests.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Command URL `https://shll.ai/tools/hop/commands/` (slug `hop`, reserved `commands` page) | From the contract per-tool table | S:95 R:90 A:95 D:95 |
| 2 | Confident | Single install pointer at install-area top covers the whole setup arc | install.md spans Install + Shell integration + First run; avoids clutter | S:80 R:85 A:80 D:80 |
| 3 | Confident | Blockquote form for the two docs/site pointers | Distinguishes "go deeper" pointers from body prose; renders cleanly on GitHub and post-rewrite | S:80 R:90 A:80 D:80 |

3 assumptions (1 certain, 2 confident, 0 tentative).
