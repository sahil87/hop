# Plan: Conform repo to shll.ai README-extraction contract

**Change**: 260608-0qwc-shll-readme-contract
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

> Derived from the frozen external contract (`shll.ai/docs/specs/readme-extraction-contract.md`)
> as audited and pinned in `intake.md`. shll.ai pulls a slice of `README.md` plus the
> `docs/site/**` tree on a daily schedule; the repo must be structured so that pull renders
> cleanly. RFC-2119 keywords are used throughout.

### README: Site-escaping links (Part 1, rule 6)

#### R1: README links that leave the rendered set MUST be absolute URLs
Any link in the pulled README slice whose target is **not** an in-document anchor and **not**
a path the site rewrites (i.e. it leaves the rendered set) MUST be written by the author as a
full `https://…` URL. The two `docs/specs/**` links in `## Reference` are the only such defects
and MUST be rewritten to absolute GitHub blob URLs; the link *text* MUST be preserved verbatim.

- **GIVEN** the `## Reference` section currently links `[\`docs/specs/cli-surface.md\`](docs/specs/cli-surface.md)` and `[\`docs/specs/config-resolution.md\`](docs/specs/config-resolution.md)`
- **WHEN** the contract is applied
- **THEN** both link *targets* become `https://github.com/sahil87/hop/blob/main/docs/specs/cli-surface.md` and `https://github.com/sahil87/hop/blob/main/docs/specs/config-resolution.md` respectively
- **AND** the link text (the backtick-wrapped path) and the trailing description prose are unchanged

#### R2: No relative README link MUST escape the rendered set
After the fix, every relative link/image target in `README.md` MUST either be an in-document
anchor (`#…`), an absolute `https://…` URL, or a relative path that points **into**
`docs/site/`. No relative link MUST resolve outside the pulled set (e.g. into `docs/specs/`,
`src/`, or any `..` escape).

- **GIVEN** the full `README.md` after edits
- **WHEN** auditing every markdown link/image target
- **THEN** the only relative targets are `docs/site/install.md` and `docs/site/workflows.md` (into the rendered tree); all others are `https://…` or `#…`

### README: Conformance preserved (Part 1, rules 1–5)

#### R3: README head, tail, images, and theme conformance MUST be preserved
The change MUST NOT alter the conformant head order (`#` H1 → toolkit blockquote → badge line
→ prose), MUST NOT introduce any GitHub-footer (tail-denylisted) sections, MUST NOT add any
relative or `#gh-*-mode-only` images, and MUST NOT add mermaid fences. Only the link edits in
R1 and the discovery links in R4 are permitted README modifications.

- **GIVEN** the audited-conformant `README.md`
- **WHEN** the change is applied
- **THEN** the file still opens with `# hop`, then the verbatim `> Part of [@sahil87's open source toolkit](https://shll.ai) …` blockquote, then the badge line, then prose
- **AND** no `^#{2,3}\s*(contributing|development|building|license|acknowledge)` heading exists
- **AND** no `#gh-dark-mode-only`/`#gh-light-mode-only` fragment and no ` ```mermaid ` fence is present

### README: Discovery links into docs/site (Part 2, rule 4)

#### R4: README MUST link into the new docs/site pages as plain inline relative links
The `## Reference` section MUST gain two NEW plain inline links pointing into the depth pages,
written in natural relative form (`docs/site/install.md`, `docs/site/workflows.md`) so the site
auto-rewrites them to `/tools/hop/install` and `/tools/hop/workflows`. These MUST be plain
inline links — NOT behind a badge/thumbnail and NOT reference-style definitions (the rule-6
rewrite trap), or they 404.

- **GIVEN** the `## Reference` section
- **WHEN** the discovery links are added
- **THEN** two inline `[text](docs/site/install.md)` / `[text](docs/site/workflows.md)` links exist in `## Reference`
- **AND** neither is wrapped in an image/badge nor expressed as a `[text][ref]` reference-style link

### docs/site: Depth pages exist and are self-contained (Part 2)

#### R5: docs/site/install.md MUST exist as a self-contained install + setup guide
A new file `docs/site/install.md` MUST be created with its own `# H1`. It MUST cover, expanded
beyond the README without verbatim duplication: Homebrew install (incl. the `wt` dependency),
from-source install (`git clone` → `just install` → `~/.local/bin/hop`), shell integration
depth (what `hop shell-init` installs, why the shim is required, the bash variant),
`shll shell-install` one-shot wiring, first-run bootstrap (`hop add -r ~/code`, `-p` preview,
`--depth N`, group derivation, `~/.config/hop/hop.yaml` location + dotfiles-symlink pattern),
and `hop update` self-upgrade behavior.

- **GIVEN** the repo before the change
- **WHEN** the change is applied
- **THEN** `docs/site/install.md` exists, opens with a single `# ` H1, and covers each listed topic
- **AND** its content is sourced from and expanded beyond the README's Install / Shell integration / First run sections (not copied verbatim)

#### R6: docs/site/workflows.md MUST exist as a self-contained grammar/workflows deep-dive
A new file `docs/site/workflows.md` MUST be created with its own `# H1`. It MUST cover, expanded
beyond the README without verbatim duplication: the one grammar `hop <selection> <action>`
(singular / plural / worktree forms), navigate / run-anything / batch-git-ops with examples, the
worktree suffix `<name>/<wt>` resolved via `wt list --json`, the shim-vs-binary dispatch model
(cd / run-in-parent-shell / passthrough, plus `command hop` for scripting/CI), and the Gotchas.

- **GIVEN** the repo before the change
- **WHEN** the change is applied
- **THEN** `docs/site/workflows.md` exists, opens with a single `# ` H1, and covers each listed topic
- **AND** its content is sourced from and expanded beyond the README's Quick tour / grammar / Gotchas sections (not copied verbatim)

### docs/site: Closed-set rules (Part 2, §9.1)

#### R7: Every relative link/image inside docs/site MUST resolve inside docs/site (Closure)
Within `docs/site/**`, every *relative* link or image target MUST resolve to a path inside
`docs/site/`. No `..` escape MUST appear. Cross-links between the two sibling pages MUST use
sibling relative form (`install.md` / `workflows.md`).

- **GIVEN** the two new pages
- **WHEN** auditing relative targets (grep for `](../` and bare relative paths)
- **THEN** the only relative targets are the sibling files `install.md` / `workflows.md`
- **AND** no `](../` (parent escape) appears anywhere under `docs/site/`

#### R8: Every external link/image inside docs/site MUST be an absolute https URL
Within `docs/site/**`, any link leaving the rendered set (GitHub source, the `wt` repo, the
`shll` repo, `docs/specs/**`, shields.io, shll.ai) MUST be a full `https://…` URL. Any image
MUST be absolute `https://…` with alt text.

- **GIVEN** the two new pages
- **WHEN** auditing every external link/image target
- **THEN** all such targets begin with `https://` (e.g. `https://github.com/sahil87/wt`, `https://github.com/sahil87/hop/blob/main/docs/specs/cli-surface.md`)
- **AND** any image (if present) is absolute with alt text

### Non-Goals

- No Go source change under `cmd/` or `internal/` — this is a presentation-surface change only.
- No CI change under `.github/`.
- No `docs/specs/**` or `docs/memory/**` change — these keep their fab meanings and are not pulled by the site.
- No `help/hop.json` work — §7 divergence is a non-fatal `::warning::`, and that artifact is shll.ai's pull responsibility (covered by changes jr5f/g56l).
- README head/tail/images/mermaid/dark-theme are already conformant (audited) — no edits there beyond R1/R4.
- shll.ai itself is not touched — it pulls daily and renders automatically.

### Design Decisions

1. **Absolute GitHub blob URLs for spec links in README**: rewrite `docs/specs/*` targets to `https://github.com/sahil87/hop/blob/main/...` — *Why*: the site does not vendor `docs/specs/**` nor rewrite arbitrary relative links, so the only render-safe form is an author-written absolute URL (contract rule 6). — *Rejected*: leaving them relative (404s on the site); pointing them at `/tools/hop/...` (no equivalent rendered page exists for the specs).
2. **Plain inline form for README → docs/site discovery links**: write `[text](docs/site/install.md)` as plain inline links. — *Why*: the site rewrites *into-`docs/site/`* relative links to `/tools/hop/...`; rule 6's trap is that badge-wrapped or reference-style links are NOT rewritten and 404. — *Rejected*: badge/thumbnail links (not rewritten); reference-style `[text][ref]` (not rewritten).
3. **Two flat sibling pages, sibling cross-links**: `install.md` and `workflows.md` at the top of `docs/site/`, cross-linking via bare `install.md` / `workflows.md`. — *Why*: simplest closed set satisfying R7; no subdirectories needed. — *Rejected*: nesting under subfolders (adds relative-path complexity for no benefit).

## Tasks

### Phase 1: README link fixes

- [x] T001 Rewrite the two `docs/specs/*.md` link *targets* in `README.md` `## Reference` (lines ~245–246) to absolute GitHub blob URLs (`https://github.com/sahil87/hop/blob/main/docs/specs/cli-surface.md` and `.../config-resolution.md`), preserving link text and description prose <!-- R1 -->
- [x] T002 Add two NEW plain inline discovery links in `README.md` `## Reference` pointing at `docs/site/install.md` and `docs/site/workflows.md` in natural relative form (not badge-wrapped, not reference-style) <!-- R4 -->

### Phase 2: docs/site depth pages

- [x] T003 [P] Create `docs/site/install.md` — self-contained `# H1` install + setup guide (Homebrew incl. wt dep; from-source; shell integration depth + bash variant; `shll shell-install`; first-run bootstrap + `hop.yaml` location + dotfiles symlink; `hop update`), expanded from the README, with a sibling cross-link to `workflows.md` and all external links absolute `https://…` <!-- R5 -->
- [x] T004 [P] Create `docs/site/workflows.md` — self-contained `# H1` grammar/workflows deep-dive (one grammar; singular/plural/worktree forms; navigate / run-anything / batch-git-ops examples; `<name>/<wt>` via `wt list --json`; shim-vs-binary dispatch + `command hop`; Gotchas), expanded from the README, with a sibling cross-link to `install.md` and all external links absolute `https://…` <!-- R6 -->

### Phase 3: Verification

- [x] T005 Audit `README.md`: confirm head order (R3), no tail-denylist headings (R3), no `#gh-*-mode-only` fragments / no mermaid (R3), and that the only relative link targets are the two `docs/site/` discovery links (R2) <!-- R2 -->
- [x] T006 Audit `docs/site/**`: grep for `](../` (must be none — R7) and for non-`https`, non-anchor, non-sibling relative targets; confirm every external link is absolute `https://…` (R8) and each file has exactly one `# ` H1 <!-- R7 -->

## Execution Order

- T001, T002 edit the same file (`README.md`) — run sequentially (T001 then T002).
- T003 and T004 are independent new files — `[P]`, may run in parallel.
- T005 depends on T001/T002; T006 depends on T003/T004.

## Acceptance

### Functional Completeness

- [x] A-001 R1: Both `docs/specs/*.md` links in `README.md` `## Reference` have absolute `https://github.com/sahil87/hop/blob/main/...` targets with original link text preserved
- [x] A-002 R4: `README.md` `## Reference` contains two plain inline links to `docs/site/install.md` and `docs/site/workflows.md`
- [x] A-003 R5: `docs/site/install.md` exists with a single `# ` H1 and covers Homebrew (+wt dep), from-source, shell integration (+bash, +shim rationale), `shll shell-install`, first-run bootstrap (`hop add -r`, `-p`, `--depth N`, group derivation, `~/.config/hop/hop.yaml`, dotfiles symlink), and `hop update`
- [x] A-004 R6: `docs/site/workflows.md` exists with a single `# ` H1 and covers the grammar (singular/plural/worktree), navigate / run-anything / batch-git-ops, `<name>/<wt>` via `wt list --json`, shim-vs-binary dispatch + `command hop`, and the Gotchas

### Behavioral Correctness

- [x] A-005 R3: `README.md` still opens with `# hop` → verbatim toolkit blockquote → badge line → prose; no GitHub-footer (tail-denylist) heading was introduced
- [x] A-006 R4: The README discovery links are plain inline (not image/badge-wrapped, not reference-style) so the site rewrites them to `/tools/hop/install` and `/tools/hop/workflows`

### Scenario Coverage

- [x] A-007 R2: A grep of `README.md` link/image targets shows the only relative targets are `docs/site/install.md` and `docs/site/workflows.md`; all others are `https://…` or `#…`
- [x] A-008 R7: A grep of `docs/site/**` shows no `](../` and no relative target other than the sibling `install.md` / `workflows.md`
- [x] A-009 R8: Every external link/image in `docs/site/**` begins with `https://`

### Edge Cases & Error Handling

- [x] A-010 R3: No `#gh-dark-mode-only` / `#gh-light-mode-only` fragment and no ` ```mermaid ` fence exists in `README.md` or the new pages

### Code Quality

- [x] A-011 Pattern consistency: New docs match the project's existing doc voice (terse, example-driven, like the README and `docs/specs`)
- [x] A-012 No unnecessary duplication: `docs/site/` pages expand on the README rather than copying it verbatim

## Notes

- Check items as you review: `- [x]`
- This is a `docs` change — no automated tests; verification is grep-based link/structure audits.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Spec links rewritten to `https://github.com/sahil87/hop/blob/main/docs/specs/...` (blob/main form) | Intake fix is verbatim; `main` is hop's default branch (confirmed by existing absolute README URLs using `/main`-adjacent forms and the repo's default branch) | S:95 R:85 A:95 D:90 |
| 2 | Certain | Two README discovery links added to `## Reference` as plain inline links | Intake assumption #5 (Confident) + section C decision explicitly prescribes this; rule-6 trap noted and avoided | S:90 R:85 A:90 D:90 |
| 3 | Confident | docs/site pages cross-link each other via bare sibling form (`install.md` / `workflows.md`) | Closed-set rule 1 names this exact form for flat sibling pages; lowest-risk satisfaction of Closure | S:85 R:80 A:85 D:85 |
| 4 | Confident | docs/site page content/length/examples authored from README source, expanded | Intake assumption #6 (Confident); contract's stated purpose is "depth that doesn't belong inline"; README is the in-repo source of truth | S:80 R:75 A:80 D:75 |
| 5 | Confident | Discovery-link text: "Install guide" / "Workflows deep-dive" (descriptive) | Intake suggests "or similar descriptive text"; exact wording is low-blast-radius prose | S:80 R:90 A:80 D:75 |

5 assumptions (2 certain, 3 confident, 0 tentative, 0 unresolved).
