# Intake: Agent-Entry Standards Pointer

**Change**: 260717-qner-agent-entry-standards-pointer
**Created**: 2026-07-18

## Origin

One-shot `/fab-new qner` invocation resolving backlog item `[qner]` (2026-07-18), quoted in full:

> Add a root-level agent-entry file (`CLAUDE.md` and/or `AGENTS.md`) that POINTS AT the sahil87 toolkit standards (enumerated by `shll standards`) rather than restating them — deferred from the toolkit-standards conformance audit (260717-fcvp). Toolkit principle №10 (SHOULD): "Agent entry files (`CLAUDE.md`/`AGENTS.md`) point at these standards rather than restating them." hop ships neither today. Deferred (not fixed in fcvp) because authoring an agent-entry pointer file is documentation curation — which idioms/standards to point at, how much repo-development context to include — not a mechanical flag/stream/error fix, so it sits outside that change's small-additive fix boundary. Keep it a thin pointer (link `shll standards`, the shll.ai tool page, and `docs/specs/`), NOT a restatement — the whole point of №10 is agents read the canonical standard, not a drifting copy. Reference: the fcvp conformance report's principles §10 entry.

No prior conversation preceded the invocation — all design context comes from the backlog item, the fcvp conformance report (`fab/changes/260717-fcvp-toolkit-standards-conformance/conformance-report.md` § principles №10), and the `principles` standard itself (`shll standards principles` § 10).

## Why

1. **Problem**: Toolkit principle №10 ("Agent-discoverable documentation", SHOULD) expects each repo's agent entry files (`CLAUDE.md`/`AGENTS.md`) to point at the toolkit standards. hop ships neither file. A fresh agent session in this repo — one not entered through the fab pipeline — gets no in-repo signal that binding standards exist, where the docs landscape lives, or how the repo is built. The fcvp conformance audit graded №10 **PARTIAL** and deferred exactly this item to `[qner]`.
2. **Consequence if unfixed**: every non-fab agent session starts from zero (`--help` round-trips, guessing idioms) and risks editing governed surfaces (CLI surface, help output, `README.md`, `docs/site/`) without checking the standards that bind them. The constitution's § Toolkit Standards covers only agents that load `fab/project/constitution.md` — i.e., fab-pipeline sessions; generic agents never see it.
3. **Why this approach**: a *thin pointer* honors №10's stated intent — "agents read the canonical standard, not a drifting copy." Restating standards in-repo would create the drift №10 exists to prevent. The backlog item pre-decides this: link `shll standards`, the shll.ai tool page, and `docs/specs/`; do NOT restate.

## What Changes

### New root file: `AGENTS.md` (canonical agent-entry pointer)

The single source of truth for agent orientation. Verified facts it builds on: no toolkit repo (`shll`, `wt`, hop) ships an agent-entry file today, so hop sets the toolkit's first precedent; the four standards enumerated by `shll standards` are `principles`, `help-dump`, `readme-extraction`, `skill`; the constitution § Toolkit Standards names the governed surfaces.

Draft content (structure and links are binding; apply may refine wording — target ≤ ~30 lines, every item a pointer plus a one-line hook, never restated standard content):

```markdown
# AGENTS.md

hop is a small Go CLI that turns one config file (`hop.yaml`) into a personal
directory of git repos — locate, open, list, clone, and batch-update them.
Part of the [sahil87 toolkit](https://shll.ai); tool page: <https://shll.ai/hop>.

## Toolkit standards (binding)

This repo conforms to the sahil87 toolkit standards. Read the canonical
standard — do not work from summaries or restatements.

- Enumerate: `shll standards` · read one: `shll standards <name>`
  (current set: `principles`, `help-dump`, `readme-extraction`, `skill`)
- Canonical source if `shll` is unavailable: the sahil87/shll repo's
  `docs/site/standards/` tree, rendered on <https://shll.ai>
- Before changing the CLI surface, help output, `README.md`, or `docs/site/`,
  check the change against the standards governing that surface
  (binding per `fab/project/constitution.md` § Toolkit Standards)

## Documentation map

- `docs/specs/index.md` — pre-implementation design intent (CLI surface,
  config resolution, architecture, build & release)
- `docs/memory/index.md` — post-implementation behavior; the authoritative
  source of truth for how hop actually works

## Working on this repo

- Source lives under `src/` (`src/cmd/hop`, `src/internal/*`); build with
  `just build`, test with `just test`
- Changes flow through the fab pipeline: `fab/project/constitution.md` holds
  the binding project principles; `/fab-*` skills live in `.claude/skills/`
```

### New root file: `CLAUDE.md` (delegating stub)

A one-line delegation so Claude Code sessions load the same content without a second copy to drift:

```markdown
@AGENTS.md
```

(Claude Code resolves `@path` imports in `CLAUDE.md`; any other tool reading `CLAUDE.md` still sees an unambiguous pointer.)

### Explicitly out of scope

- No changes to `README.md`, `docs/site/`, CLI surface, or help output — so no governed-surface standards check is triggered.
- No `hop skill` subcommand — that is backlog `[armh]`, a separate deferral.
- No backlog checkoff edits — `[qner]` is marked done at archive time per `/fab-archive`.

## Affected Memory

None — docs-only curation with no spec-level behavior change. The pointer file is itself the documentation artifact; `docs/memory/` records system behavior, which is untouched.

## Impact

- **Files added**: `AGENTS.md`, `CLAUDE.md` (repo root). Nothing modified, nothing removed.
- **Code/tests**: none affected — no Go code, no CLI surface, no test changes.
- **Standards interaction**: `readme-extraction` governs `README.md` + `docs/site/` only; root agent-entry files sit outside every governed surface, so this change satisfies №10 without tripping any other standard's check.
- **Toolkit precedent**: as the first agent-entry file in the 7-tool toolkit, its shape (AGENTS.md canonical + CLAUDE.md `@`-stub, pointer-only content) is likely to be copied by sibling repos.

## Open Questions

None — the two curation decisions the backlog flagged (which file(s); how much repo-development context) graded Confident and are recorded below for `/fab-clarify` review.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Thin pointer, not restatement: link `shll standards`, <https://shll.ai/hop>, and `docs/specs/` | Backlog item pre-decides this verbatim; it is №10's stated intent | S:90 R:95 A:95 D:95 |
| 2 | Confident | Ship BOTH files: `AGENTS.md` canonical + `CLAUDE.md` as a one-line delegating stub | Backlog says "and/or" (open); no toolkit precedent exists; cross-vendor convention favors AGENTS.md as the shared source with CLAUDE.md delegating, so Claude Code and other agents read one copy | S:55 R:90 A:50 D:55 |
| 3 | Confident | `CLAUDE.md` delegates via the `@AGENTS.md` import line rather than prose | Claude Code resolves `@path` imports natively; a bare `@AGENTS.md` is still a readable pointer for any other consumer | S:45 R:95 A:60 D:60 |
| 4 | Confident | Include `docs/memory/` index, fab-pipeline note, and `just build`/`just test` one-liners alongside the three backlog-named links | Backlog names three links but leaves "how much repo-development context" open; these additions are still pointers (paths + one-line hooks), consistent with the thin-pointer rule | S:50 R:95 A:65 D:60 |
| 5 | Confident | Affected Memory: none | Docs-only curation; memory records spec-level system behavior, which does not change | S:70 R:85 A:75 D:80 |

5 assumptions (1 certain, 4 confident, 0 tentative, 0 unresolved).
