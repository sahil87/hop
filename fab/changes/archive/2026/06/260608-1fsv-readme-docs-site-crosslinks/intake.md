# Intake: README → docs/site cross-links + command-reference link

**Change**: 260608-1fsv-readme-docs-site-crosslinks
**Created**: 2026-06-08
**Status**: Draft

## Origin

Conversational follow-up to the shll.ai README-extraction contract conformance change (`260608-0qwc-shll-readme-contract`, shipped as PR #42). The user asked:

> Also add references to docs/site/* files from README.md - example: the installation section should point to install.md. For the command reference (also) point to https://shll.ai/tools/<tool-name>/commands/ . Add a new PR for these changes.

**Mode**: one-shot, with one clarifying decision resolved before intake. The user chose **contextual pointers in each section** (over "only add the commands URL"): add a short pointer under `## Install` → `install.md`, a pointer under `## Grammar at a glance` → `workflows.md`, and a command-reference pointer (`hop --help` + `https://shll.ai/tools/hop/commands/`) in `## Reference`, while keeping the existing `## Reference` discovery links.

**Relationship to prior work.** PR #42 added the two `docs/site/` pages and two *discovery* links in `## Reference`. Those reference-section links satisfy the contract's mechanical rewrite path but are easy to miss. This change surfaces the same pages *contextually* — next to the body content they expand — and adds the public command-reference URL on shll.ai. hop's slug is `hop` (per the contract per-tool table), so the command-reference URL is `https://shll.ai/tools/hop/commands/`.

## Why

**Problem.** The two `docs/site/` depth pages are only linked from the `## Reference` section at the very bottom of the README. A reader in the `## Install` section who wants the fuller setup guide, or a reader in `## Grammar at a glance` who wants the workflows deep-dive, has no in-context pointer — they'd have to scroll to the bottom and infer the connection. There is also no link to the rendered command reference that shll.ai publishes at `/tools/hop/commands/` (fed by the `help-dump` pull); the README only mentions `hop --help`.

**Consequence if not done.** The depth pages stay under-discovered, and the published command reference on shll.ai is invisible from the canonical README. Low stakes, but a cheap discoverability win that completes the cross-linking the contract encourages.

**Why this approach.** Contextual pointers placed immediately after the relevant section body are the natural discovery path. The README→`docs/site/` links use the contract's natural relative form (`[text](docs/site/install.md)`), which shll.ai auto-rewrites to `/tools/hop/install` and `/tools/hop/workflows` — and which renders as a normal GitHub link on github.com. The command-reference link is an absolute `https://shll.ai/tools/hop/commands/` URL (it leaves the rendered set, so per contract rule 6 it must be absolute-by-author). We keep the existing `## Reference` discovery links — they do no harm and serve readers who land at the bottom.

## What Changes

Edits to `README.md` only. **No code, no CI, no docs/site content change, no new files.**

### A. `## Install` → pointer to `docs/site/install.md`

After the install subsections (Homebrew / From source) and before `## Shell integration`, add a short blockquote pointer. The install page also covers shell integration and first-run bootstrap, so a single pointer at the top of the install area covers the whole setup arc:

```markdown
> **Fuller guide:** [Installing and setting up hop](docs/site/install.md) — Homebrew & from-source, shell integration in depth, and first-run bootstrap.
```

This is a natural relative link into `docs/site/` (auto-rewritten to `/tools/hop/install` on the site; a normal link on GitHub). It is a plain inline link in a blockquote — not behind a badge/thumbnail, not a reference-style definition (avoids the contract rule-6 trap).

### B. `## Grammar at a glance` → pointer to `docs/site/workflows.md`

After the grammar table and its trailing prose, add a pointer to the workflows deep-dive:

```markdown
> **Deep-dive:** [hop workflows](docs/site/workflows.md) — the three jobs, worktree workflows, the shim-vs-binary dispatch model, and gotchas, with worked examples.
```

Same form and rationale as A.

### C. `## Reference` → command-reference link to `https://shll.ai/tools/hop/commands/`

The `## Reference` section currently leads with `` - `hop --help` — full subcommand listing ``. Add a sibling bullet pointing at the published command reference on shll.ai (the page shll.ai renders from the `help/hop.json` pull). "(also)" per the user's phrasing — `hop --help` stays as the local path; the URL is the online rendered companion:

```markdown
- `hop --help` — full subcommand listing (rendered online at [shll.ai/tools/hop/commands](https://shll.ai/tools/hop/commands/))
```

This is an absolute `https://…` URL — required because it leaves the rendered set (contract rule 6). The existing `[Install guide](docs/site/install.md)` and `[Workflows deep-dive](docs/site/workflows.md)` reference bullets are **kept unchanged**.

### Contract conformance (must still hold after these edits)

These edits must not break the contract conformance achieved in PR #42:
- README head order (H1 → toolkit blockquote → badges → prose) is untouched.
- The two new `docs/site/` pointers (A, B) are the contract's auto-rewritten relative form — the *only* new relative link targets; both point INTO `docs/site/`.
- The command-reference link (C) is absolute `https://…` (leaves the rendered set).
- No new images, no mermaid, no `#gh-*-mode-only` fragments, no denylisted footer headings introduced.

## Affected Memory

- (none) — presentation-surface change to `README.md`; no hop behavior, CLI surface, config, or build process changes. `docs/` is in `true_impact_exclude`.

## Impact

- **`README.md`** — three small additions (two blockquote pointers + one bullet augmentation). No removals beyond the inline edit to the `hop --help` bullet.
- **No** `docs/site/**`, `src/`, `.github/`, `docs/specs/**`, or `docs/memory/**` changes.
- **External** — shll.ai is not touched; the `/tools/hop/commands/` URL is a link target, rendered by shll.ai's existing pull. Lands as a new PR in `sahil87/hop`.

## Open Questions

- None. Placement was decided with the user (contextual pointers in each section). The command URL slug is fixed by the contract (`hop`).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Command-reference URL is `https://shll.ai/tools/hop/commands/` | Slug `hop` + reserved `commands` page from the contract per-tool table | S:95 R:90 A:95 D:95 |
| 2 | Certain | README→docs/site pointers use natural relative form `[text](docs/site/<page>.md)`, plain inline (blockquote) | Contract rule 4 (auto-rewrite) + rule 6 trap avoidance; same form PR #42 used in `## Reference` | S:95 R:85 A:95 D:90 |
| 3 | Certain | Command-reference link is absolute `https://…` | Leaves the rendered set → contract rule 6 mandates absolute-by-author | S:95 R:90 A:95 D:95 |
| 4 | Confident | Place install pointer once at the install-area top (covers Install + Shell integration + First run, since install.md spans all three) | One pointer avoids clutter; install.md's scope is the whole setup arc | S:80 R:85 A:80 D:80 |
| 5 | Confident | Keep the existing `## Reference` discovery links unchanged | They do no harm and serve bottom-of-README readers; user said "also" (add, not replace) | S:85 R:90 A:85 D:85 |
| 6 | Confident | `hop --help` bullet augmented in place (URL as companion) rather than a separate bullet | User wrote "(also)" — the online reference complements the local `--help`; keeping them on one bullet reads cleanest | S:75 R:90 A:80 D:75 |

6 assumptions (3 certain, 3 confident, 0 tentative, 0 unresolved).
