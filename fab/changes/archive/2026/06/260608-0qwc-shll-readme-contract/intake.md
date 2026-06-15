# Intake: Conform repo to shll.ai README-extraction contract

**Change**: 260608-0qwc-shll-readme-contract
**Created**: 2026-06-08
**Status**: Draft

## Origin

Initiated one-shot via `/fab-new` with a frozen task prompt pointing at an external contract:

> Task: conform this repo to shll.ai's README-extraction contract.
>
> shll.ai (the toolkit landing page) renders your tool's page by mechanically pulling a slice of your README.md and your docs/site/** tree on a daily schedule — nothing is hand-copied, and you push nothing. Your job is to structure your repo so that pull renders cleanly.
>
> Read the contract and follow its §Producer conformance directive end-to-end: `https://github.com/sahil87/shll.ai/blob/main/docs/specs/readme-extraction-contract.md`
>
> 1. Find this repo's row in the directive's per-tool table (by repo name) for your slug and reserved page names.
> 2. Do Part 1 — restructure README.md: head order (# H1 → toolkit blockquote → badges → prose), drop GitHub-footer sections below the tail denylist, make all images absolute https://… URLs, render any mermaid to a committed image, and write any link that leaves the site as an absolute URL (relative links 404 on the site).
> 3. Do Part 2 (optional but encouraged) — add a docs/site/**/*.md tree for depth (install guide, deep-dives, etc.), following the four closed-set rules. Use docs/site/install.md / docs/site/workflows.md for those pages.
> 4. Run the directive's Verify checklist before opening the PR.
>
> Ship it as a single PR in this repo. Do not touch shll.ai — it already pulls and renders automatically.

**Mode**: one-shot. The contract is an external frozen spec — the design is pinned there, not decided here. During intake the contract was fetched and read in full, and the current `README.md` and `docs/` tree were audited against every rule.

**Relationship to prior changes.** This is the **README/docs-site half** of hop's shll.ai pull-model integration. Two prior changes handled the *command-reference* half against a *different* spec (`help-dump-contract.md`):
- `260602-jr5f-help-dump-shllai-json` added the `help-dump` producer (cobra-tree walk → `help/hop.json`) plus push wiring.
- `260603-g56l-teardown-shllai-push-wiring` removed the push transport when shll.ai inverted to a scheduled **pull** model, deliberately preserving the `help-dump` command as the contract surface for the `commands` reserved page.

This change is complementary and non-overlapping: it feeds the `readme`/`overview` reserved pages (from `README.md`) and the optional `install`/`workflows` pages (from `docs/site/**`). There is no Go source change and no CI change — it is purely repo content structure. Same transport model (shll.ai pulls daily; hop pushes nothing).

**hop's row in the per-tool table** (looked up by repo name `hop`):

| Repo | Binary | `content/<slug>/` | URL space | Reserved static slugs |
|------|--------|-------------------|-----------|----------------------|
| `hop` | `hop` | `content/hop/` | `/tools/hop/` | `overview`, `readme`, `commands` |

So: slug `hop`, URL space `/tools/hop/`, reserved page names `overview` / `readme` / `commands` (must not be used as `docs/site/` page names). `install` and `workflows` are explicitly **not** reserved and are the prescribed names for the two depth pages.

## Why

**Problem.** shll.ai renders hop's tool page by mechanically pulling a slice of `README.md` (and any `docs/site/**` tree) on a daily schedule. If the repo's structure doesn't conform to the extraction contract, the pulled page renders wrong: broken images (relative paths the site never vendors), 404'd links (relative links to source/specs that escape the rendered set), stripped diagrams, or a slice that runs past the intended boundary into GitHub-footer chrome.

**Consequence if not done.** hop's landing page on shll.ai degrades silently and without warning — the maintainer pushes nothing and gets no build error, because the pull is one-directional and daily. Broken images and dead links ship to the public tool page. hop also loses parity with the other six tools in the rollout, several of which publish `docs/site/install` and `docs/site/workflows` depth pages.

**Why this approach.** The contract is a frozen external spec with an explicit Verify checklist. We follow it precisely rather than redesign: conform the README head/tail/links/images to the rules, and add the two encouraged depth pages under `docs/site/`. The current README is **already ~90% conformant** (correct head order, verbatim toolkit blockquote, absolute shields.io images, no mermaid, no `#gh-*-mode-only` fragments, no denylisted footer sections) — the audit found only two real defects to fix, plus the optional Part 2 tree to add. Minimal, surgical, contract-driven.

## What Changes

This is a content-structure change to `README.md` plus two new files under `docs/site/`. **No code, no CI, no `fab/` artifacts beyond this change folder.**

### A. Fix the two site-escaping relative links in `README.md` (Part 1, rule 6 — the only Part 1 defect)

The audit found exactly two relative links that escape the rendered set. They live in the `## Reference` section (currently lines 245–246):

```markdown
- [`docs/specs/cli-surface.md`](docs/specs/cli-surface.md) — canonical CLI contract ...
- [`docs/specs/config-resolution.md`](docs/specs/config-resolution.md) — config search order ...
```

`docs/specs/**` is **not** pulled by the site (only `README.md` and `docs/site/**` are), and the site does **not** rewrite arbitrary relative links — so these render as dead `/tools/hop/docs/specs/...` links that 404. Per rule 6, any link leaving the rendered set must be written by us as an absolute `https://…` URL.

**Fix**: rewrite both targets to absolute GitHub blob URLs:

```markdown
- [`docs/specs/cli-surface.md`](https://github.com/sahil87/hop/blob/main/docs/specs/cli-surface.md) — canonical CLI contract ...
- [`docs/specs/config-resolution.md`](https://github.com/sahil87/hop/blob/main/docs/specs/config-resolution.md) — config search order ...
```

The link *text* (`` `docs/specs/cli-surface.md` ``) stays as-is — only the *target* becomes absolute. (Note: in the new `docs/site/` pages we should instead link these to the rendered `/tools/hop/...` pages where an equivalent page exists, and to absolute GitHub URLs otherwise — see closed-set rules in C.)

### B. Confirm — no other Part 1 changes needed (audited conformant)

Each Part 1 rule was checked against the current README; the following are already satisfied and require **no edit**:

- **§1 Head order** — file opens with markdown `# hop` (not `<h1>`), then the single `>` toolkit blockquote *verbatim* (`> Part of [@sahil87's open source toolkit](https://shll.ai) — see all projects there.`, target `https://shll.ai`), then the contiguous badge line, then prose. No YAML frontmatter, no HTML comment above the H1. ✅
- **§2 Tail denylist** — grep for `^#{2,3}\s*(contributing|development|building|license|acknowledge)` returns nothing. The README has no GitHub-footer sections to drop; the slice already runs to EOF cleanly. `Install` (present) is explicitly *not* denylisted. ✅
- **§3 Images** — every image is an absolute `https://img.shields.io/...` URL with alt text. No relative image paths exist. ✅
- **§4 Dark-theme** — no `#gh-dark-mode-only` / `#gh-light-mode-only` fragments present. ✅
- **§5 Mermaid** — no ` ```mermaid ` fences anywhere in the README. Nothing to render to an image. ✅
- **Intra-README anchors** — `#grammar-at-a-glance` and `#gotchas` are same-document anchors, not site-escaping links; they resolve within the pulled slice and are fine. ✅

So Part 1 reduces to the single two-line link fix in A.

### C. Add the `docs/site/**` depth tree (Part 2 — encouraged, two pages)

Create two new files, the exact names the directive prescribes for hop:

- `docs/site/install.md` → renders at `/tools/hop/install`
- `docs/site/workflows.md` → renders at `/tools/hop/workflows`

Neither name collides with the reserved set (`overview`/`readme`/`commands`), so both are allowed.

**Content sourcing.** These pages give depth that doesn't belong inline in the README. Draw from the README's own Install / Shell integration / First run sections (for `install.md`) and the Quick tour / grammar / Gotchas sections (for `workflows.md`), expanded with material that would bloat the README — without duplicating the README verbatim. Both must be self-contained pages with their own `# H1`.

**`docs/site/install.md`** — a fuller install + setup guide:
- Homebrew install (incl. that the formula pulls in `wt` as a dependency).
- From-source install (`git clone` → `just install` → `~/.local/bin/hop` on PATH).
- Shell integration in depth: what `eval "$(hop shell-init zsh)"` installs (the `hop` function, the `h` alias, completion), why the shim is required (Unix cwd constraint), bash variant.
- `shll shell-install` as the one-shot wiring path for users with multiple sahil87 tools.
- First run / bootstrap: `hop add -r ~/code`, preview mode `-p`, depth control `--depth N`, group derivation, where `hop.yaml` lives (`~/.config/hop/hop.yaml`), and the dotfiles-symlink portability pattern.
- `hop update` self-upgrade behavior (brew vs. source-install hint).

**`docs/site/workflows.md`** — task-oriented deep-dive on the grammar and daily workflows:
- The one grammar (`hop <selection> <action>`), singular vs. plural vs. worktree forms.
- Navigate / run-anything-in-a-repo / batch-git-ops, with examples.
- Worktree suffix (`<name>/<wt>`) resolution via `wt list --json`.
- The shim-vs-binary dispatch model (three answers: cd / run-in-parent-shell / passthrough) and `command hop` for scripting/CI.
- The Gotchas (trailing `.` for editors, shim-only navigation, name-only substring match, no `--force` on batch verbs, `wt` on PATH for `/<wt>`).

**Closed-set rules (§9.1) these two files MUST satisfy** — these are the acceptance-critical constraints for Part 2:

1. **Closure** — every *relative* link/image inside `docs/site/**` resolves to a path *inside* `docs/site/`. No `..` escapes. For these two flat sibling pages, an intra-tree link is simply `[workflows](workflows.md)` / `[install](install.md)` (relative, sibling). Cross-links between the two pages use this form.
2. **External links absolute-by-author** — any link leaving the rendered set (GitHub source, `wt` repo, `shll` repo, `docs/specs/**`) is written as a full `https://…` URL. E.g. `https://github.com/sahil87/wt`, `https://github.com/sahil87/hop/blob/main/docs/specs/cli-surface.md`.
3. **All images absolute** — same as Part 1 rule 3. If any badge/diagram image is used, it is an absolute `https://…` URL with alt text. (Likely none needed in these prose pages.)
4. **README → `docs/site/` links written naturally** — *if* we choose to link the README to the new pages, write them as `[Install guide](docs/site/install.md)` and the site rewrites to `/tools/hop/install`. **Trap to avoid (rule 6):** such a link must be a plain inline link — not behind a badge/thumbnail and not a reference-style definition, or it won't be rewritten and will 404.

**Decision on whether to link the README → new pages:** Yes — add two plain inline links in the README `## Reference` section pointing at `docs/site/install.md` and `docs/site/workflows.md` (natural relative form, auto-rewritten). This is the contract's intended discovery path and is low-risk. These are *into-`docs/site/`* relative links, which the site **does** rewrite — distinct from the *escaping* `docs/specs/` links fixed in A.

### D. No `help/<slug>.json` action (§7 is non-fatal and out of scope)

§7's command/flag divergence check cross-references the pulled prose against `help/hop.json`. That artifact is produced by shll.ai's own scheduled `help-dump` pull (per the `help-dump-contract.md`, handled by changes jr5f/g56l) — not committed to this repo. A divergence emits a non-blocking `::warning::` and never blocks the pull; the README is canonical and ships verbatim. No action here.

## Affected Memory

This is a content/docs-structure change with no spec-level *behavior* change to hop itself (the CLI surface, config resolution, and architecture are unchanged). The change is excluded from true-impact via the existing `docs/` exclusion in `config.yaml`. No memory file create/modify/remove is warranted.

- (none) — no spec-level behavior change; `README.md` and `docs/site/**` are presentation surfaces, not documented hop behaviors.

## Impact

- **`README.md`** — two link targets rewritten to absolute URLs (section C/A); optionally two new inline links added to `## Reference` pointing into `docs/site/`.
- **`docs/site/install.md`** — new file.
- **`docs/site/workflows.md`** — new file.
- **No code** under `src/`. **No CI** under `.github/`. **No** `docs/specs/**` or `docs/memory/**` changes (those keep their fab meanings and are not pulled by the site).
- **External system** — shll.ai is *not* touched; it pulls automatically on its daily schedule. The PR lands only in `sahil87/hop`.

## Open Questions

- None blocking. The contract is frozen and the audit is unambiguous. The only authored judgment is the *content* of the two new `docs/site/` pages (depth, examples, length), which is a writing task bounded by the README's existing material and the four closed-set rules — not a decision requiring user input.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | hop's slug is `hop`, URL space `/tools/hop/`, reserved page names `overview`/`readme`/`commands`; depth pages are `install.md` + `workflows.md` | Read directly from the contract's per-tool table by repo name; `install`/`workflows` named as non-reserved in the same doc | S:98 R:90 A:95 D:95 |
| 2 | Certain | Only Part 1 defect is the two `docs/specs/*.md` relative links in `## Reference`; fix = absolute GitHub blob URLs | Mechanical audit: grep found exactly these two non-https, non-anchor relative links; contract rule 6 dictates the fix verbatim | S:95 R:85 A:95 D:90 |
| 3 | Certain | README head order, toolkit blockquote, images, mermaid, dark-theme, tail denylist already conform — no edits there | Each rule checked against the file; blockquote is verbatim, images all absolute shields.io, no mermaid/`gh-*` fragments, no denylisted headings (grep clean) | S:95 R:80 A:95 D:90 |
| 4 | Confident | Add `docs/site/install.md` and `docs/site/workflows.md` (Part 2 is optional but explicitly encouraged, and these exact names are prescribed) | Task says "optional but encouraged" and names the two files; parity with other tools in the rollout favors doing it | S:85 R:75 A:80 D:85 |
| 5 | Confident | Link README `## Reference` → the two new `docs/site/` pages using natural relative form (`[..](docs/site/install.md)`), as plain inline links | Rule 4 says write these naturally and the site rewrites them; rule 6 trap (no badge/reference-style) is avoidable; improves discovery | S:80 R:80 A:80 D:80 |
| 6 | Confident | `docs/site/` pages source depth from existing README sections (install/shell/first-run; tour/grammar/gotchas), expanded — not verbatim duplication | "depth that doesn't belong inline" is the contract's stated purpose; README is the authoritative source material already in-repo | S:80 R:75 A:80 D:75 |
| 7 | Certain | No `help/hop.json` work (§7 divergence is non-fatal; artifact is shll.ai's pull responsibility, covered by jr5f/g56l) | Contract states §7 is report-only `::warning::`, never blocks; help artifact handled by the separate help-dump-contract pull | S:95 R:90 A:90 D:90 |
| 8 | Certain | No code/CI/memory/spec changes; ship as a single PR into `sahil87/hop`; do not touch shll.ai | Task explicit ("Ship it as a single PR in this repo. Do not touch shll.ai"); change is presentation-surface only | S:98 R:90 A:95 D:95 |

8 assumptions (6 certain, 2 confident, 0 tentative, 0 unresolved).
