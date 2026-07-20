# Intake: Centralize Install Docs (Policy B)

**Change**: 260720-of6z-centralize-install-docs
**Created**: 2026-07-20

## Origin

One-shot `/fab-new` invocation with a detailed directive:

> Conform this repo's install documentation to the shll toolkit's install-composition standard, Policy B. Read the authoritative standard first: /home/sahil/code/sahil87/shll/docs/site/standards/install-composition.md (rendered on https://shll.ai). Policy B: per-tool READMEs and doc pages must not carry per-formula "brew install sahil87/tap/<tool>" install instructions; installation points to https://shll.ai (curl bootstrap: `curl -fsSL https://shll.ai/install | sh`; subset installs remain supported via `shll install <tool>`). Task: audit README.md and docs/site/ for per-formula install instructions and replace them with the shll.ai pointer. IMPORTANT distinction: replace install *instructions* (sections telling the user how to install), but KEEP incidental mentions such as actionable error-hint examples in standards/conformance text (Policy A mandates those hints) and historical/changelog references. Mechanical docs-only change; keep all usage and feature content intact.

The authoritative standard was read at intake time. Key bindings for this repo (a roster-tool repo, squarely in Policy B's producer scope):

- Per-tool READMEs and doc pages MUST NOT carry per-formula `brew install` instructions; they link to https://shll.ai for install steps (curl bootstrap or `shll install`).
- Individual formula installs remain *supported* — what is unsupported is *documenting* them per-repo.
- Policy A context: sibling tools carry no formula `depends_on` edges (hop's `depends_on "wt"` was removed — commit 719650b, PR #62), presence is probed at runtime, and missing-sibling paths emit the actionable hint `<tool> is not installed. Install it: brew install sahil87/tap/<tool>`. Such hint text in standards/conformance prose is the KEEP carve-out — hop's docs and code were audited and carry no such hint text, so nothing here triggers it.
- The constitution's Toolkit Standards section binds this repo to the standard without further amendment.

An intake-time audit (`grep -rniE 'brew install|brew tap|shll install|shll\.ai|homebrew' README.md docs/site/`) produced the exact violation inventory in What Changes.

## Why

1. **Pain point**: `docs/site/install.md` leads with a `brew install sahil87/tap/hop` section, and the wt gotcha in both README.md and `docs/site/workflows.md` tells users to `brew install sahil87/tap/wt` — per-formula install documentation that Policy B forbids in roster-tool repos. Worse, three places claim "the Homebrew formula pulls `wt` in as a dependency," which has been false since PR #62 removed the `depends_on "wt"` edge per Policy A — the docs currently promise a behavior the formula no longer has.
2. **Consequence of not fixing**: seven repos' copies of the install dance drift (Policy B's stated failure mode) — any change to the install story (tap trust, bootstrap changes) must be chased across every repo. Concretely, hop's docs are *already* drifted: a user following install.md's from-source path is told brew handles `wt` for brew users, which it no longer does.
3. **Approach**: point install documentation at https://shll.ai (curl bootstrap / `shll install <tool>`) per Policy B, rather than updating the brew lines in place — centralizing is the standard's entire point; per-repo copies are the disease, not the specific text.

## What Changes

Docs-only. Four files; no code, no formula, no behavior changes. Full violation inventory and per-site treatment:

### docs/site/install.md — replace the Homebrew section, fix the wt story

**§1 "Install the binary" → "### Homebrew (macOS and Linux)"** (lines 9–15) currently:

```sh
brew install sahil87/tap/hop
```

plus the sentence "The formula pulls in `wt` as a dependency…brew installs it for you." Replace the heading/section with the shll.ai-pointed install, e.g.:

```sh
curl -fsSL https://shll.ai/install | sh -s -- hop
```

with prose: installs via Homebrew under the hood, handling tap trust automatically (mirroring README's existing conformant wording); the full toolkit via `curl -fsSL https://shll.ai/install | sh`; see https://shll.ai for the complete install story. The wt sentence is rewritten to drop the false dependency claim: `wt` is a sibling tool that is NOT installed automatically; hop shells out to `wt list --json` for the `<name>/<wt>` suffix, so install wt (`shll install wt`, or the full-toolkit bootstrap) for worktree navigation; bare `hop <name>` never touches wt. The heading should no longer be brew-specific (e.g. "### Via shll (macOS and Linux)" or similar).

**§1 "From source"** (line 27): "install it separately: `brew install sahil87/tap/wt`, or build it from source the same way" → "install it separately via [shll.ai](https://shll.ai) (`shll install wt`), or build it from source the same way". The from-source instructions themselves (git clone / just install) are usage content and stay intact.

**§5 "Keeping hop up to date"** (line 98): KEEP — `hop update` self-upgrading through brew is feature description, not an install instruction.

### README.md — wt gotcha + install.md pointer blurb

**Top `## Install` section (lines 9–19)**: KEEP unchanged — already conformant (curl bootstrap `curl -fsSL https://shll.ai/install | sh -s -- hop`, full-toolkit variant, shll.ai link at line 3).

**Gotcha bullet (line 250)**: "The Homebrew formula pulls wt in as a dependency; for non-brew installs, `brew install sahil87/tap/wt` or build from source." → rewrite tail: wt is not installed automatically — install it via [shll.ai](https://shll.ai) (`shll install wt`) or build from source. Keep the bullet's usage content (shells out to `wt list --json`, no state cached, bare queries never invoke wt) verbatim.

**Fuller-guide blurb (line 88)**: "…Homebrew & from-source, shell integration in depth…" — reword "Homebrew" to match the revised install.md (e.g. "install via shll & from-source…").

**Line 76** ("self-upgrades via Homebrew"): KEEP — feature description.

### docs/site/workflows.md — wt gotcha

**Line 151**: same gotcha as README:250 ("Homebrew pulls `wt` in as a dependency; for non-brew installs, `brew install sahil87/tap/wt` or build from source") → same rewrite: wt is not installed automatically; install via [shll.ai](https://shll.ai) (`shll install wt`) or build from source. Usage content in the bullet stays.

### docs/site/skill.md — no change

**Line 36** (`hop update` — "Self-update via Homebrew"): KEEP — feature description, not an install instruction.

### Explicitly out of scope

- `docs/specs/` (build-and-release.md mentions homebrew-tap as release-pipeline machinery — historical/architecture content, not user install instructions; and specs are outside the audited surface).
- Any code, formula, or `hop update` behavior.
- Adding Policy-A error hints to hop's code (none exist today for missing `wt` — hop's `/`-suffix path may fail without a hint, but that is a Policy A code concern, a separate change if desired).

## Affected Memory

None — docs-only change to user-facing install documentation; no spec-level behavior changes. (`build/release-pipeline` covers the tap/release machinery, which is untouched.)

## Impact

- **Files**: `docs/site/install.md` (install section rewrite), `README.md` (2 small edits), `docs/site/workflows.md` (1 bullet edit). ~15 lines net.
- **No source code, tests, formula, or CI changes.** `true_impact_exclude` already excludes `docs/` — this change is near-zero true impact by design.
- **Verification**: re-run the audit grep after editing — remaining `brew`/Homebrew mentions must all be in the KEEP set (`hop update` feature descriptions). Standard's conformance checklist item: "The README's install section links to https://shll.ai instead of carrying per-formula `brew install` lines."

## Open Questions

None — the directive plus the authoritative standard resolve every decision point.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Replace install.md's `brew install sahil87/tap/hop` section with the shll.ai curl-bootstrap pointer | Directly mandated by Policy B and the task directive; exact target named by the audit | S:95 R:90 A:95 D:95 |
| 2 | Certain | README's top Install section stays as-is (curl bootstrap inline + shll.ai link) | Task directive quotes this exact curl bootstrap as the Policy B pointer; it is the centralized entry point, not a per-formula line | S:90 R:90 A:90 D:85 |
| 3 | Confident | The wt gotcha lines (README:250, workflows.md:151) and install.md:27 are install *instructions*, not KEEP-carve-out material — replace `brew install sahil87/tap/wt` with a shll.ai / `shll install wt` pointer | The KEEP carve-out covers error-hint examples in standards/conformance text and changelogs; these are user-facing "how to get wt" advice in usage docs — exactly the per-formula documentation Policy B forbids | S:75 R:85 A:80 D:70 |
| 4 | Confident | Fix the stale "formula pulls wt in as a dependency" claims in the same sentences being rewritten | The claim is false since PR #62 (Policy A edge removal) and is entangled with the install advice being replaced — a mechanical rewrite cannot preserve a false claim; still docs-only | S:65 R:85 A:85 D:70 |
| 5 | Certain | Keep all `hop update`-via-Homebrew mentions (README:76, install.md §5, skill.md:36) | Feature/usage descriptions of existing binary behavior, not install instructions; directive says keep feature content intact | S:85 R:90 A:90 D:85 |
| 6 | Confident | Keep install.md's from-source section (and README's) intact apart from the wt line | Policy B bans per-formula brew lines, not from-source build docs; from-source is usage content the directive preserves | S:75 R:90 A:85 D:75 |

6 assumptions (3 certain, 3 confident, 0 tentative, 0 unresolved).
