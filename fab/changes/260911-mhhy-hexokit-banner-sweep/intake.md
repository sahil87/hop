# Intake: HexoKit banner sweep (hop)

**Change**: 260911-mhhy-hexokit-banner-sweep
**Created**: 2026-09-11

## Origin

One-shot `/fab-new` invocation, driven by the cross-repo HexoKit rebrand plan (run-kit repo, `fab/plans/sahil/26-09-10-hexokit-rebrand.md`), row **C7** (`hexokit-banner-sweep`), applied to the `hop` repo. Raw input:

> Per fab/plans/sahil/26-09-10-hexokit-rebrand.md row C7 (hexokit-banner-sweep), applied to the hop repo, gated on C1 (shll change ttoa, PR shll#98 up, review-pr done): apply the readme-extraction standard's revised mandated blockquote to this repo's README (-> "Part of [HexoKit](https://hexokit.com) -- see all projects there"); flip any PRESENT-TENSE "run-kit" PRODUCT mentions (prose referring to the dashboard product) to HexoKit. The `rk`/`run-kit` SUBSTRATE (binary name, verbs, options) is UNTOUCHED. Repo links stay as-is until R2. Read the plan doc's Decision log (D1-D14) and row C7 first; run `shll standards` and read `readme-extraction` before editing.

Pre-intake reading done per the plan's pickup protocol: the plan's Decision log D1–D14 and row C7; `shll standards` (nine standards listed); `readme-extraction` — both the **installed** copy (still carries the old blockquote, because C1 is not yet merged) and the **revised** copy on the C1 branch (`sahil87/shll` PR #98, head `260911-ttoa-hexokit-banner-and-policy`, state OPEN/draft, `mergeStateStatus: CLEAN`, review-pr done). The only diff between the two copies is the mandated blockquote line.

Gate check: **C1 is satisfied** as stated by the user — shll change `ttoa` has run through review-pr and PR shll#98 is up. This change does not wait for shll#98 to merge.

Micro-change backstop note: the diff is one README line and memory is untouched, so the three micro criteria nearly hold. Tracked anyway because (a) the plan's "one row = one PR" protocol needs a PR and a row-status update, (b) the identical-shape precedent `260718-o59l-shll-toolkit-name-conformance` (the previous banner rename) was run as a tracked docs change, and (c) the sweep half of the scope is a verified-negative that review should confirm, not a single edit.

## Why

**Problem.** The `readme-extraction` standard mandates one exact blockquote under every toolkit README's H1 — "this exact line in all seven repos". C1 revised that line from the shll.ai brand to the HexoKit brand (plan D14). Until hop follows, hop's README (and its rendered `/hop/readme` page on the consuming site) carries the *old* brand on the most visible cross-repo brand surface — the first line under the H1.

**Consequence of not doing it.** The rebrand's whole point is findability: "run-kit"/shll.ai is unsearchable and buried, HexoKit is the product name (D1). A toolkit repo still saying "shll toolkit" reintroduces the two-brand split the plan exists to remove (D5, D14 "leaving it reintroduces the two-brand split for a saving of seven lines"). It also makes hop non-conformant with a binding toolkit standard (constitution § Toolkit Standards: revised standards bind this repo without amendment).

**Why this approach.** Content-only, standards-first (plan § Pickup protocol step 2): the standard was revised first (C1), and each satellite repo applies the revised text mechanically. No rename of anything else in this repo — repo links, badges, `shll.ai` endpoint URLs, and every "shll toolkit" phrase outside the blockquote are owned by later rows (R2, X2, X4). The sweep for present-tense "run-kit" product mentions is part of the C7 row scope and was executed; in hop it finds nothing to flip (see What Changes).

## What Changes

### 1. `README.md` line 3 — the mandated blockquote

Replace the old canonical line with the revised canonical line, byte-exact from the C1 branch of `docs/site/standards/readme-extraction.md` (note the em-dash `—`, not the ASCII `--` used as shorthand in the invocation):

```diff
 # hop

-> Part of the [shll toolkit](https://shll.ai) — see all projects there.
+> Part of [HexoKit](https://hexokit.com) — see all projects there.

 [![Latest release](https://img.shields.io/github/v/release/sahil87/hop)](...)
```

Nothing else in the head moves: `# hop` H1 → blockquote → the single badge line → tagline prose, exactly the order the standard's rule 1 requires. Verification per the standard's § Verifying conformance: README top is H1 → toolkit blockquote → badges; first prose line unchanged.

### 2. Present-tense "run-kit" product-mention sweep — result: nothing to flip

Whole-repo grep for `run-kit|runkit` (case-insensitive) across live surfaces, classified per the plan's naming tiers and D11:

| Surface | Hits | Classification | Action |
|---------|------|----------------|--------|
| `README.md` | 0 | — | none |
| `docs/site/{install,skill,workflows}.md` | 0 | — | none |
| `cmd/`, `internal/`, `scripts/`, `.github/`, `justfile` | 0 | — | none |
| `docs/specs/build-and-release.md` (lines 5, 95, 112, 118, 122, 126, 159, 222, 229) | 9 | References to the **`sahil87/run-kit` repository** and its release workflow files (`~/code/sahil87/run-kit/.github/workflows/release.yml`, `formula-template.rb`, the tap's `Formula/rk.rb`) — repo links/paths, not prose about the dashboard product | **leave** — repo links stay until R2 (user instruction; plan R2(d)) |
| `docs/memory/build/release-pipeline.md` (7, 23, 41, 68, 108, 127), `docs/memory/build/ci-pipeline.md` (9) | 7 | Same repo/workflow references, in memory narrative | **leave** — repo links until R2, and memory prose is the historical tier (D11); the run-kit repo's own X3 row owns memory identity sweeps |
| `fab/changes/**`, `fab/backlog.md` | many | Historical tier | **leave** (D11) |

Conclusion: hop has **zero** present-tense prose that names the dashboard *product* as "run-kit". The sweep's deliverable is this verified-negative; the plan's C7 risk note ("fab-kit skill prose drift … C7 needs a careful present-tense-only pass") applies to fab-kit, not hop. Substrate identifiers (`rk`, `Formula/rk.rb`) are untouched by definition — hop contains no `rk` verbs to touch anyway.

### 3. Explicitly out of scope (owned by later plan rows)

- **"shll toolkit" phrases outside the blockquote** — `README.md:24` ("To install the entire shll toolkit instead"), `README.md:239` ("install it via [shll.ai](https://shll.ai)"), `docs/site/install.md:27` ("To install the entire shll toolkit instead … see shll.ai for the complete install story"). D14 assigns these to **X4** (Phase 2 second pass); the plan's C1 row says "Leave every other `shll.ai` / 'shll toolkit' mention for X4". Same rule applied here.
- **Every `shll.ai` URL** — the install one-liner (`https://shll.ai/install`), `shll.ai/hop/commands/`, `shll.ai/hop/skill`. These are live endpoints (D4) and consuming-site links that flip at **X2/X4** after hexokit.com is the consuming site.
- **Badges and `sahil87/hop` repo links** — unchanged (they don't even mention run-kit).
- **Code, help text, CLI surface, `hop skill` bundle** — untouched; no help-dump or skill-standard re-check needed because neither surface changes.
- **Cross-repo bookkeeping (not in this repo's diff)** — the plan doc's C7 row in the run-kit repo gets its "PR / Status" cells filled per the plan's pickup protocol step 4 ("fill folder/PR when you create the change"). That is an operator edit in `~/code/sahil87/run-kit`, done alongside this change, not a task in this repo's plan. Note C7 spans several repos (fab-kit, wt, idea, tu, hop, sahil87 profile); this change is the **hop** slice only — the row's status should say so.

## Affected Memory

None — the README banner line is not documented anywhere in `docs/memory/**` (grep for `blockquote`, `see all projects`, `readme-extraction`, `shll toolkit`, `banner` across `docs/memory` and `docs/specs` returns only an unrelated `depends_on` design-decision line in `build/release-pipeline.md`). No spec-level behavior changes. Identical posture to precedent `260718-o59l-shll-toolkit-name-conformance`, whose Affected Memory was also none. Hydrate has nothing to update beyond the domain logs' no-op.

## Impact

- **Files**: `README.md` — 1 line changed. No other file in this repo changes.
- **Rendered site**: the consuming extractor matches any leading blockquote (`BLOCKQUOTE_RE` in `extract-readme.ts`, per D14), so the README slice boundary is unaffected; only the banner text changes on `/hop/readme` after the next daily pull.
- **Standards**: conformant with the revised `readme-extraction` rule 1 (C1 text). All other rules (images absolute, tail heading, links) are untouched by a one-line change.
- **Tests/CI**: no Go code changes; CI (`go test`) is unaffected. `help/hop.json` and the embedded `skill.md` do not change.
- **Ordering**: this PR may merge before or after shll#98 — the standard's text on the C1 branch is final (review-pr done) and the extractor is banner-agnostic, so merge order is cosmetic.

## Open Questions

None.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Blockquote text is the C1 branch's byte-exact line `> Part of [HexoKit](https://hexokit.com) — see all projects there.` (em-dash), not the invocation's ASCII `--` shorthand | Standard says "this exact line in all seven repos"; read from `origin/260911-ttoa-hexokit-banner-and-policy:docs/site/standards/readme-extraction.md`; installed `shll standards readme-extraction` still shows the pre-C1 line | S:90 R:95 A:95 D:90 |
| 2 | Certain | C1 gate is satisfied; do not wait for shll#98 to merge | User stated it; verified PR #98 OPEN (draft), `mergeStateStatus: CLEAN`; plan says pipeline through review-pr done 2026-09-11 | S:95 R:90 A:95 D:95 |
| 3 | Certain | Sweep result is "nothing to flip": all 16 live `run-kit` hits (docs/specs, docs/memory) are references to the `sahil87/run-kit` repo/workflow files, i.e. repo links, not product prose | Each hit inspected; every one names a path or file under `~/code/sahil87/run-kit` or the tap's `Formula/rk.rb`; user instruction "repo links stay as-is until R2"; D11 for memory prose | S:85 R:90 A:90 D:85 |
| 4 | Certain | Leave "shll toolkit" phrases and `shll.ai` URLs outside the blockquote untouched | D14 assigns them to X4; D4 keeps the endpoints; C1's own row says "Leave every other shll.ai / shll toolkit mention for X4" | S:95 R:95 A:95 D:95 |
| 5 | Certain | No substrate identifiers change | User instruction + plan § Naming tiers; hop contains no `rk` verbs/options at all | S:95 R:100 A:100 D:100 |
| 6 | Certain | Affected Memory is none; hydrate is a no-op | Grep of docs/memory + docs/specs for banner/blockquote/readme-extraction terms found nothing; precedent o59l identical | S:80 R:95 A:85 D:85 |
| 7 | Certain | Change type `docs` | README-only prose edit, no behavior; precedent o59l/hwxt are `docs` | S:70 R:100 A:85 D:85 |
| 8 | Certain | Plan doc C7 row status update is a cross-repo operator edit in the run-kit repo, not a task in this repo's plan | Plan pickup protocol step 4 requires it; it lives in another repo's file, so it cannot be in this PR | S:80 R:95 A:80 D:80 |
| 9 | Confident | Merge order relative to shll#98 is independent | Extractor is banner-agnostic (D14); standard text on C1 branch is post-review; a few hours of hop leading the standard is cosmetic | S:60 R:95 A:75 D:70 |

9 assumptions (8 certain, 1 confident, 0 tentative, 0 unresolved).
