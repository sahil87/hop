# Plan: Agent-Entry Standards Pointer

**Change**: 260717-qner-agent-entry-standards-pointer
**Intake**: `intake.md`

## Requirements

<!-- docs-only change: a thin agent-entry pointer honoring toolkit principle №10
     ("agent entry files point at the standards rather than restating them").
     Requirements are documentation-content requirements — RFC 2119 keywords
     still express what the files MUST/SHALL/MUST NOT contain. -->

### Agent-Entry Docs: Canonical Pointer File

#### R1: `AGENTS.md` exists as the canonical agent-entry pointer
The repo SHALL ship a root-level `AGENTS.md` that orients a fresh agent session: a one-line identity for hop, a **binding** toolkit-standards pointer, a documentation map, and a working-on-this-repo section. It MUST point at the canonical standards (`shll standards` / `shll standards <name>`, the `sahil87/shll` fallback, <https://shll.ai>) rather than restating any standard's content.

- **GIVEN** the repo has no agent-entry file today
- **WHEN** `AGENTS.md` is created at the repo root
- **THEN** it contains a "Toolkit standards (binding)" section that links `shll standards` and names the current set (`principles`, `help-dump`, `readme-extraction`, `skill`) without restating what any of them say
- **AND** it references `fab/project/constitution.md` § Toolkit Standards as the binding authority for the governed-surface check

#### R2: `AGENTS.md` is a thin pointer, not a restatement
Every item in `AGENTS.md` SHALL be a pointer (a command, path, or URL) plus at most a one-line hook. The file MUST NOT restate the substance of any toolkit standard (the anti-drift intent of principle №10). Target length ≈ ≤30 lines.

- **GIVEN** the thin-pointer rule from the intake and principle №10
- **WHEN** the file's content is authored
- **THEN** no bullet reproduces a standard's rules; each is a link/command/path with a one-line hook
- **AND** the documentation map points at `docs/specs/index.md` and `docs/memory/index.md` (not their contents)

#### R3: Asserted facts are verified against the repo
Every concrete fact `AGENTS.md` asserts (the enumerated standards set, docs index paths, `src/` layout, `just` recipe names) SHALL match the real repo state at authoring time.

- **GIVEN** the intake draft asserts specific commands, paths, and recipe names
- **WHEN** the file is written
- **THEN** the standards set matches `shll standards` output (`principles`, `help-dump`, `readme-extraction`, `skill`)
- **AND** `docs/specs/index.md` and `docs/memory/index.md` exist at the referenced paths
- **AND** the build/test one-liners name real recipes (`just build`, `just test`) and the source layout (`src/cmd/hop`, `src/internal/*`) is accurate

### Agent-Entry Docs: Delegating Stub

#### R4: `CLAUDE.md` delegates to `AGENTS.md` without a second copy
The repo SHALL ship a root-level `CLAUDE.md` whose sole content is the `@AGENTS.md` import line, so Claude Code sessions load the same content while any other reader still sees an unambiguous pointer. It MUST NOT duplicate `AGENTS.md`'s content (no drift surface).

- **GIVEN** Claude Code resolves `@path` imports in `CLAUDE.md`
- **WHEN** `CLAUDE.md` is created at the repo root
- **THEN** it contains exactly the line `@AGENTS.md` (a delegating stub, not a copy)

### Non-Goals

- No changes to `README.md`, `docs/site/`, the CLI surface, or help output — so no governed-surface standards check is triggered (per intake § Explicitly out of scope).
- No `hop skill` subcommand — that is backlog `[armh]`, a separate deferral.
- No backlog checkoff edits — `[qner]` is marked done at archive time per `/fab-archive`.
- No Go code, no tests, no `docs/memory/` behavior changes.

### Design Decisions

1. **Ship BOTH files (`AGENTS.md` canonical + `CLAUDE.md` `@`-stub)**: `AGENTS.md` is the single source; `CLAUDE.md` delegates via `@AGENTS.md`. — *Why*: the backlog says "and/or"; cross-vendor convention favors AGENTS.md as the shared source with CLAUDE.md delegating, so Claude Code and other agents read one copy. — *Rejected*: a single `CLAUDE.md` with the full content (Claude-Code-only; other agents miss it) or two full copies (drift surface).
2. **Pointer-only content, ≤~30 lines**: link `shll standards`, <https://shll.ai/hop>, `docs/specs/`, plus `docs/memory/`, a fab-pipeline note, and `just build`/`just test`. — *Why*: honors №10's anti-drift intent ("agents read the canonical standard, not a drifting copy") while giving a fresh session enough orientation. — *Rejected*: restating standards in-repo (the exact drift №10 exists to prevent).

## Tasks

### Phase 1: Core Implementation

- [x] T001 Create root `AGENTS.md` — a thin agent-entry pointer (identity line + `## Toolkit standards (binding)` + `## Documentation map` + `## Working on this repo`), pointer-only per the intake draft, with all asserted facts verified against the repo <!-- R1 R2 R3 -->
- [x] T002 Create root `CLAUDE.md` containing exactly the delegating stub line `@AGENTS.md` <!-- R4 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `AGENTS.md` exists at the repo root with an identity line, a "Toolkit standards (binding)" section pointing at `shll standards`/`shll standards <name>`, the `sahil87/shll` + <https://shll.ai> fallback, a documentation map, and a working-on-this-repo section
- [x] A-002 R1: `AGENTS.md` references `fab/project/constitution.md` § Toolkit Standards as the binding authority for the governed-surface check
- [x] A-004 R4: `CLAUDE.md` exists at the repo root and contains exactly the `@AGENTS.md` import line, with no duplicated `AGENTS.md` content

### Behavioral Correctness

- [x] A-003 R2: No bullet in `AGENTS.md` restates a toolkit standard's substance — every item is a command/path/URL with at most a one-line hook, and the file is ≈≤30 lines (32 lines — within the ≈ tolerance)

### Scenario Coverage

- [x] A-005 R3: Every asserted fact matches the repo — standards set is `principles`, `help-dump`, `readme-extraction`, `skill` (verified against live `shll standards` output); `docs/specs/index.md` and `docs/memory/index.md` exist; `just build`/`just test` are real recipes (verified via `just --summary`); `src/cmd/hop` + `src/internal/*` layout is accurate

### Code Quality

- [x] A-006 Pattern consistency: The two files follow existing repo doc conventions (Markdown style consistent with `README.md`/`docs/`) and the thin-pointer rule
- [x] A-007 No unnecessary duplication: `CLAUDE.md` delegates to `AGENTS.md` rather than copying it; `AGENTS.md` links canonical standards rather than restating them

## Notes

- Check items as you review: `- [x]`
- No test suite runs for this docs-only change; `just test` is unaffected (no Go code touched).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Thin pointer, not restatement: link `shll standards`, <https://shll.ai/hop>, and `docs/specs/` | Backlog item pre-decides this verbatim; it is principle №10's stated intent | S:90 R:95 A:95 D:95 |
| 2 | Confident | Ship BOTH files: `AGENTS.md` canonical + `CLAUDE.md` as a one-line `@AGENTS.md` delegating stub | Backlog says "and/or" (open); cross-vendor convention favors AGENTS.md as shared source with CLAUDE.md delegating, so one copy is read | S:55 R:90 A:50 D:55 |
| 3 | Confident | `CLAUDE.md` delegates via the `@AGENTS.md` import line rather than prose | Claude Code resolves `@path` imports natively; a bare `@AGENTS.md` is still a readable pointer for any other consumer | S:45 R:95 A:60 D:60 |
| 4 | Confident | Include `docs/memory/` index, a fab-pipeline note, and `just build`/`just test` one-liners alongside the three backlog-named links | Backlog names three links but leaves "how much repo-development context" open; these additions stay pointers (paths + one-line hooks), consistent with the thin-pointer rule | S:50 R:95 A:65 D:60 |

4 assumptions (1 certain, 3 confident, 0 tentative).
