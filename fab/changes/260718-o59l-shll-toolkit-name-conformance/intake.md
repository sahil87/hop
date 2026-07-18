# Intake: shll Toolkit Name Conformance

**Change**: 260718-o59l-shll-toolkit-name-conformance
**Created**: 2026-07-18

## Origin

One-shot `/fab-new` invocation with a fully-specified conformance task:

> Task: Conform this repo to the toolkit's standardized name — "shll toolkit".
>
> The toolkit formerly named "sahil87 toolkit" is now the **shll toolkit** (sahil87/shll#56). The readme-extraction standard's canonical README blockquote changed accordingly. This repo's constitution already binds it to revised standards without amendment — this task is the conformance work.
>
> 1. **README blockquote** — replace with the exact line, byte-identical, keeping head order (H1 → blockquote → badges): `> Part of the [shll toolkit](https://shll.ai) — see all projects there.`
> 2. **Prose sweep** — `sahil87 toolkit` → `shll toolkit`, `sahil87 tool(s)` → `shll tool(s)` in README, `docs/site/**`, CLI help text / user-visible strings (+ goldens), `fab/project/` files; re-run embed sync if embedded docs change.
> 3. **Constitution (cosmetic, same PR)** — Toolkit Standards article: "part of the sahil87 toolkit" → "part of the shll toolkit", bump `Last Amended`.
> 4. **Do NOT touch identifiers**: `sahil87/tap` formula names, `github.com/sahil87/…` / `raw.githubusercontent.com/sahil87/…` URLs, the `sahil87/shll` canonical-source reference, GitHub-owner constants. `fab/changes/` archives untouched.
>
> Ship per normal flow (one fab change → PR). Tests green; help-dump JSON shape unchanged (text-only — no `schema_version` bump).

**Precondition verified at intake time**: `shll standards readme-extraction` runs on this machine and its "README structure" §1 shows the new canonical blockquote exactly as specified: `> Part of the [shll toolkit](https://shll.ai) — see all projects there.` No `shll update` was needed; the do-not-proceed-from-memory guard is satisfied.

**Scope verified at intake time** (repo-wide grep for `sahil87 toolkit`, `sahil87 tool`, `sahil87's`, `@sahil87`, excluding `fab/changes/`): the affected prose lives in exactly four files — `README.md` (3 sites), `docs/site/install.md` (1 site), `fab/project/constitution.md` (1 site), `fab/backlog.md` (1 site). No Go user-visible strings, no test goldens, and no `docs/site/skill.md` content carry the old name — so no golden updates and no `just sync-skill` re-run are required, and the skill drift-guard test is unaffected.

## Why

1. **Pain point**: The toolkit was renamed from "sahil87 toolkit" to "shll toolkit" (sahil87/shll#56), and the readme-extraction standard's canonical README blockquote changed to match. This repo still carries the old name in its README head, install docs, constitution, and one backlog entry — it is now non-conformant with a published standard that binds it.
2. **Consequence of not fixing**: The constitution's Toolkit Standards article states revised standards bind this repo *without amendment* — the repo is out of compliance today. Concretely: shll.ai pulls and renders the README slice daily, so the stale blockquote (`> Part of [@sahil87's open source toolkit](https://shll.ai) …`) is publicly visible on hop's tool page and diverges from the other toolkit repos' byte-identical blockquote.
3. **Approach**: A minimal, mechanical prose sweep — exact-line blockquote replacement plus phrase substitution — with a strict identifier exclusion list. No behavior changes, no code changes; the only non-doc file is the constitution's cosmetic wording fix (explicitly allowed in the same PR by the task).

## What Changes

### 1. README blockquote (`README.md:3`)

Replace the current blockquote:

```markdown
> Part of [@sahil87's open source toolkit](https://shll.ai) — see all projects there.
```

with this exact line, **byte-identical**:

```markdown
> Part of the [shll toolkit](https://shll.ai) — see all projects there.
```

The mandated head order is already in place (`# hop` H1 → blockquote → contiguous badge line) and MUST be preserved — this is a single-line swap, no structural movement.

### 2. Prose sweep (two more README sites + docs/site)

- `README.md:15` — `…To install the entire sahil87 toolkit instead:` → `…To install the entire shll toolkit instead:`
- `README.md:101` — `> 💡 Have other sahil87 tools? …` → `> 💡 Have other shll tools? …`
- `docs/site/install.md:58` — `### One-shot wiring for multiple sahil87 tools` → `### One-shot wiring for multiple shll tools`

Verified non-targets (already checked, nothing to do):

- `docs/site/skill.md` — contains no old-name prose → **no `just sync-skill` re-run needed**; the embedded copy `src/cmd/hop/skill.md` stays byte-identical and the drift-guard test (`skill_test.go`) is unaffected.
- CLI help text / Go user-visible strings — no occurrences (`grep` over `src/`); help-dump JSON is untouched entirely (not even text edits), so the `{tool, version, schema_version, root}` envelope and `schema_version` are trivially unchanged.
- Bare "toolkit" mentions without the old owner prefix (e.g. `docs/site/install.md:60` "several tools from the toolkit", `src/cmd/hop/skill.go:33` comment, `docs/specs/cli-surface.md:30`, `docs/memory/**`) — already conformant or identifier-adjacent; **not** part of the sweep.

### 3. Constitution cosmetic edit (`fab/project/constitution.md:33`)

In the **Toolkit Standards** article, change only the opening clause:

- `This tool is part of the sahil87 toolkit and MUST conform…` → `This tool is part of the shll toolkit and MUST conform…`

The article's `sahil87/shll` canonical-source reference ("the canonical sources are the sahil87/shll repository's docs/site/standards/ tree") stays **untouched** — it is a repo identifier. Governance line bump: `Last Amended` → `2026-07-18` (today — it already reads 2026-07-18, so the date is coincidentally unchanged) and `Version` 1.2.0 → **1.2.1** (patch — cosmetic wording, no rule change).

### 4. Backlog entry (`fab/backlog.md:15`)

The `[qner]` backlog item's prose "…POINTS AT the sahil87 toolkit standards…" → "…POINTS AT the shll toolkit standards…". This file is not in the task's enumerated sweep list, but the entry is forward-looking prose (a pending work item), not a historical artifact — see Assumption 3. The change-folder reference `260717-fcvp` and PR references in the same entry are identifiers and stay untouched.

### 5. Exclusions (MUST NOT change)

- `sahil87/tap` Homebrew formula names (e.g. `brew install sahil87/tap/hop`, `sahil87/tap/wt`)
- `github.com/sahil87/…` and `raw.githubusercontent.com/sahil87/…` URLs everywhere
- The `sahil87/shll` canonical-source reference inside the constitution article
- Any GitHub-owner constants in code
- Historical artifacts: `fab/changes/` archives, and `docs/memory/**` prose that names past change folders (e.g. `260717-fcvp-toolkit-standards-conformance`)

## Affected Memory

None — this is a docs-prose rename with no spec-level behavior change. No `docs/memory/**` file references the old "sahil87 toolkit" name (verified by grep), so hydrate has nothing to update.

## Impact

- **Files**: `README.md`, `docs/site/install.md`, `fab/project/constitution.md`, `fab/backlog.md` — 4 files, ~6 lines, prose only.
- **Code**: none. **Tests**: none change; suite must stay green (drift-guard and help-dump envelope tests unaffected by construction).
- **External surface**: shll.ai's next README pull renders the new blockquote and prose; conformance per the readme-extraction standard's "Verifying conformance" checklist (head order intact, no new relative links, no images touched).
- **Ship flow**: one fab change → one PR, per normal flow.

## Open Questions

None — the task is fully specified, the precondition passed, and the sweep scope was verified by grep at intake time.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Precondition satisfied: proceed without `shll update` | `shll standards readme-extraction` ran at intake time and shows the new canonical blockquote byte-for-byte as the task specifies | S:95 R:90 A:100 D:95 |
| 2 | Certain | Sweep scope is exactly 4 files (README ×3 sites, install.md, constitution, backlog); no Go strings, goldens, or skill-bundle sync needed | Repo-wide grep for all old-name variants (incl. possessive `@sahil87's`) found no other prose occurrences outside `fab/changes/`; `docs/site/skill.md` and `src/` are clean | S:90 R:85 A:95 D:90 |
| 3 | Confident | Include `fab/backlog.md:15` in the sweep | Not in the task's enumerated file list, but "wherever they appear as prose" is the governing clause; a pending backlog item is forward-looking prose, not a historical artifact (only `fab/changes/` archives are excluded) | S:60 R:95 A:80 D:70 |
| 4 | Confident | Constitution governance bump: Version 1.2.0 → 1.2.1, Last Amended 2026-07-18 (already today's date, so visually unchanged) | Task mandates the Last Amended bump; a wording-only edit is a patch-level amendment by semver convention — the governance line carries a version, and amending without bumping it would leave two constitution texts under one version | S:55 R:90 A:75 D:65 |

4 assumptions (2 certain, 2 confident, 0 tentative, 0 unresolved).
