# Plan: Adopt the Toolkit `skill` Standard (`hop skill`)

**Change**: 260717-armh-adopt-skill-standard
**Intake**: `intake.md`

## Requirements

### Skill Bundle: Canonical Content

#### R1: Canonical `docs/site/skill.md` bundle
hop SHALL ship a canonical agent skill bundle at `docs/site/skill.md`. The bundle MUST be a **usage briefing** in the standard's genre — NOT a README clone and NOT a flag table. It MUST be **≤150 lines** (hard budget), **static only** (byte-identical on every read; no timestamps, no environment lookups), and cover the standard's five sections instantiated for hop: When to use, Capabilities map (keyed to the `hop <selection> <action>` grammar and subcommands), Composition patterns (principle №7 — shim/`shell-init`, `wt`, `fzf`, `git`, `hop ls --json`), Output & exit-code contracts (stdout=data / stderr=diagnostics; exit `0`/`1`/`2`/`3`/`130`), and Gotchas (shell-only forms, no-TTY exit 3, unique-substring short-circuit, statelessness).

- **GIVEN** the standard's genre and budget constraints
- **WHEN** `docs/site/skill.md` is authored
- **THEN** the file is ≤150 lines, contains no dynamic/environment-derived content, and reads as a usage briefing
- **AND** it renders at `https://shll.ai/hop/skill` for free as part of the pulled `docs/site/**` tree (no extra publishing work)

### CLI Surface: `hop skill` Subcommand

#### R2: `hop skill` invocation contract
hop SHALL expose a `hop skill` cobra subcommand. `Use` MUST be exactly `"skill"` (not `agent`/`context`). It MUST take `cobra.NoArgs` (no flags, no positionals) and MUST be **visible** (NOT `Hidden`, unlike `help-dump`), so it appears in `hop --help` and the help-dump tree. On invocation it MUST write the embedded bundle bytes verbatim to `cmd.OutOrStdout()` — no rendering, no pager, no added framing — leave stderr empty, and exit 0.

- **GIVEN** the `hop` binary
- **WHEN** I run `hop skill`
- **THEN** stdout is byte-identical to the embedded bundle, stderr is empty, and exit code is 0
- **AND** `hop skill extra-arg` is rejected by `cobra.NoArgs`
- **AND** `hop skill` appears in `hop --help` and `hop help-dump` output (visible command)

#### R3: Embed mechanism (committed copy + sync + go:generate)
The bundle MUST be embedded into the binary via the sync + drift-guard pattern `shll standards` established. A committed copy MUST live at `src/cmd/hop/skill.md` (embedded via `//go:embed skill.md`), synced from the canonical `docs/site/skill.md` by `scripts/sync-skill.sh`, with a `//go:generate` directive pointing at that script. The Go module root is `src/`, so `//go:embed` cannot reach `docs/site/` above it — the committed copy bridges the gap and lets a clean `go build ./...` compile without running the script.

- **GIVEN** the module-root layout constraint (`//go:embed` cannot reach `../../../docs/site/`)
- **WHEN** the package is built
- **THEN** `src/cmd/hop/skill.md` is embedded via `//go:embed` and a clean `go build ./...` succeeds without running the sync script
- **AND** a `//go:generate ../../../scripts/sync-skill.sh` directive is present so `go generate` refreshes the copy

### Build: Sync Script + Justfile Recipe

#### R4: `scripts/sync-skill.sh` + `just sync-skill`
A `scripts/sync-skill.sh` script MUST copy `docs/site/skill.md` into `src/cmd/hop/skill.md`, modeled on shll's `scripts/sync-standards.sh` (Constitution V: thin justfile, logic in scripts/). The `justfile` MUST gain a one-line `sync-skill` recipe that delegates to the script.

- **GIVEN** an edit to `docs/site/skill.md`
- **WHEN** I run `just sync-skill` (or `scripts/sync-skill.sh`)
- **THEN** `src/cmd/hop/skill.md` is refreshed to match the canonical file and a confirmation line is printed
- **AND** the justfile recipe is a one-liner delegating to the script (Constitution V)

### Test: Drift Guard + Contract Tests

#### R5: `src/cmd/hop/skill_test.go` guards
A test file `src/cmd/hop/skill_test.go` MUST assert: (a) a **drift guard** — the embedded `skill.md` bytes equal `../../../docs/site/skill.md`, failing on divergence with a message naming the fix (`just sync-skill`); (b) an **invocation contract** — `hop skill` returns nil from RunE (exit 0), stdout byte-identical to the embedded copy, stderr empty, and rejects extra args; (c) a **budget guard** — the bundle is ≤150 lines.

- **GIVEN** the embedded copy and the canonical file diverge
- **WHEN** `go test ./...` runs
- **THEN** the drift guard fails, naming `docs/site/skill.md` and the `just sync-skill` fix
- **AND** the invocation-contract test confirms stdout==embedded, stderr empty, exit 0, and NoArgs rejection
- **AND** the budget guard fails if the bundle exceeds 150 lines

### Docs: CLI-Surface Spec + README Cross-Link

#### R6: Same-change docs upkeep
`docs/specs/cli-surface.md` MUST gain a `hop skill` row in the Subcommand Inventory (args: none; behavior: print embedded bundle markdown to stdout, stderr empty; exit 0) — the fcvp fix policy requires CLI-surface changes to update this spec in the same commit. `README.md` MUST gain a Docs-section line pointing at `docs/site/skill.md`, preserving the readme-extraction cross-link posture.

- **GIVEN** a change that adds a subcommand and a `docs/site/` page
- **WHEN** the change is applied
- **THEN** `docs/specs/cli-surface.md` has a `hop skill` inventory row and `README.md`'s Reference section links `docs/site/skill.md`

### Non-Goals

- Adding flags to `hop skill` (the standard fixes it as arg-free) — out of scope.
- Rendering/paging the bundle — the standard mandates raw bytes to stdout.
- Adopting `skill` for any sibling toolkit tool — hop is the first adopter; this is hop's slice only.
- Building `shll agent-setup` (the standard's forward design) — not part of this change.

### Design Decisions

1. **Single-file `//go:embed skill.md` (not `embed.FS` like shll)**: hop embeds ONE bundle file, so `//go:embed skill.md` into a `[]byte` is the minimal correct form — *Why*: shll's `embed.FS` + roster struct exists because shll serves N standards documents through one `standards` command; hop serves exactly one bundle through `skill`, so the roster indirection would be dead weight — *Rejected*: mirroring shll's `embed.FS`/roster verbatim (over-engineered for a single file); embedding from `docs/site/` directly (impossible — outside the `src/` module root).
2. **Visible command (not `Hidden`)**: `hop skill` ships visible — *Why*: the standard says each tool "exposes" the subcommand, the bundle is a published page on shll.ai, and visibility serves №10 discovery; it therefore appears in `hop --help` and the help-dump tree — *Rejected*: `Hidden: true` (as `help-dump` is) — help-dump is an internal build-time contract surface, whereas `skill` is a user/agent-facing discovery surface.
3. **Bundle content sourced from existing cli memory**: the gotchas/contract prose condenses `cli/agent-non-interactive-usage.md`, `cli/subcommands.md`, and `cli/match-resolution.md` — *Why*: those files already record the exit-code contract, shell-only forms, and statelessness; the bundle restates them in agent-first briefing form rather than inventing new claims.

## Tasks

### Phase 1: Setup

- [x] T001 Author the canonical bundle `docs/site/skill.md` — usage briefing, ≤150 lines, static-only, five sections (When to use / Capabilities map / Composition patterns / Output & exit-code contracts / Gotchas) instantiated for hop from the intake outline and cli memory files. <!-- R1 -->
- [x] T002 Create `scripts/sync-skill.sh` (executable, modeled on shll's `scripts/sync-standards.sh`): copy `docs/site/skill.md` → `src/cmd/hop/skill.md`, print a confirmation line. <!-- R4 -->
- [x] T003 [P] Add a `sync-skill` one-liner recipe to `justfile` delegating to `./scripts/sync-skill.sh` (Constitution V). <!-- R4 -->

### Phase 2: Core Implementation

- [x] T004 Sync the committed embed copy: run `scripts/sync-skill.sh` to produce `src/cmd/hop/skill.md` (byte-identical to the canonical file). <!-- R3 -->
- [x] T005 Create `src/cmd/hop/skill.go`: `//go:generate ../../../scripts/sync-skill.sh` + `//go:embed skill.md` into `var skillBundle []byte`; `newSkillCmd()` factory with `Use: "skill"`, `cobra.NoArgs`, visible, `Short`/`Long` describing the bundle, `RunE` writing `skillBundle` verbatim to `cmd.OutOrStdout()`. <!-- R2 -->
- [x] T006 Register `newSkillCmd()` in `src/cmd/hop/root.go`'s `newRootCmd().AddCommand(...)` list. <!-- R2 -->

### Phase 3: Integration & Edge Cases

- [x] T007 Create `src/cmd/hop/skill_test.go`: drift guard (embedded == `../../../docs/site/skill.md`, message names `just sync-skill`), invocation contract (RunE nil / stdout==embedded / stderr empty / NoArgs rejects an extra arg), budget guard (≤150 lines). <!-- R5 -->

### Phase 4: Polish

- [x] T008 [P] Add the `hop skill` row to `docs/specs/cli-surface.md` Subcommand Inventory (args none; bundle markdown → stdout, stderr empty; exit 0). <!-- R6 -->
- [x] T009 [P] Add a `docs/site/skill.md` cross-link to `README.md`'s Reference section. <!-- R6 -->
- [x] T010 Re-verify the help-dump conformance checklist after the tree change (fcvp fix-policy convention): confirm `hop help-dump` still emits valid JSON and now includes the visible `skill` node. <!-- R2 -->

## Execution Order

- T001 blocks T004 (sync copies the authored canonical file) which blocks T005 (embed needs the committed copy present to compile).
- T002 blocks T004 (the sync script must exist to run).
- T005 blocks T006 (register after the factory exists) and T007 (tests exercise the command).
- T003, T008, T009 are independent `[P]` and can run alongside the core work.
- T010 runs last (after T005/T006 add the visible command).

## Acceptance

### Functional Completeness

- [x] A-001 R1: `docs/site/skill.md` exists, is ≤150 lines, static-only (no timestamps/env lookups), and covers all five briefing sections in genre (usage briefing, not README/flag table).
- [x] A-002 R2: `hop skill` is a visible cobra command named exactly `skill` with `cobra.NoArgs`, present in `hop --help`.
- [x] A-003 R3: `src/cmd/hop/skill.md` is embedded via `//go:embed skill.md`, a `//go:generate` directive points at `scripts/sync-skill.sh`, and a clean `cd src && go build ./...` compiles without running the script.
- [x] A-004 R4: `scripts/sync-skill.sh` copies the canonical file to the embed copy and `just sync-skill` delegates to it in one line.
- [x] A-005 R5: `src/cmd/hop/skill_test.go` implements the drift guard, invocation contract, and budget guard; all pass under `go test ./...`.
- [x] A-006 R6: `docs/specs/cli-surface.md` has a `hop skill` inventory row and `README.md` links `docs/site/skill.md`.

### Behavioral Correctness

- [x] A-007 R2: Running `hop skill` writes stdout byte-identical to the embedded bundle, leaves stderr empty, and exits 0.

### Scenario Coverage

- [x] A-008 R2: `hop skill extra` is rejected (NoArgs) with a non-zero exit and a usage diagnostic.
- [x] A-009 R5: A deliberate divergence between the committed copy and the canonical file makes the drift-guard test fail (verified by the test's construction against `../../../docs/site/skill.md`).

### Edge Cases & Error Handling

- [x] A-010 R2: `hop skill` does not add framing/rendering — a subsequent `hop skill | diff - docs/site/skill.md` (via the embedded copy) yields no differences.

### Code Quality

- [x] A-011 Pattern consistency: `skill.go` follows the `newXxxCmd()` factory + `RunE` + `cmd.OutOrStdout()` pattern of `help_dump.go` and the other hop command files; the sync script and test mirror shll's naming.
- [x] A-012 No unnecessary duplication: the single-file embed reuses stdlib `embed` (precedent: `src/internal/config/starter.yaml`); no roster indirection is introduced for one file.
- [x] A-013 Constitution I/II: `skill.go` uses no subprocess (pure `embed` + cobra) and reads no state — nothing to route through `internal/proc`, no `hop.yaml`/filesystem lookup.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change adds new functionality without making existing code redundant (purely additive: one new subcommand, one new script, one new test file; no existing surface replaced or superseded).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Command is exactly `skill`, raw markdown to stdout, stderr empty, exit 0, no flags/args | Standard's invocation contract is explicit and uniform; backlog restates it verbatim | S:95 R:80 A:95 D:95 |
| 2 | Certain | Single-file `//go:embed skill.md` into a `[]byte` (not shll's `embed.FS`/roster) | hop serves ONE bundle through one command; the roster indirection is dead weight for a single file; `//go:embed` into `[]byte` is the minimal correct form and matches the intake's code snippet verbatim | S:90 R:85 A:90 D:85 |
| 3 | Certain | Committed copy `src/cmd/hop/skill.md` + `scripts/sync-skill.sh` + drift-guard test + `//go:generate`, mirroring shll's sync-standards pattern | Standard mandates reuse of the exact mechanism `shll standards` established; same module-root layout constraint applies | S:90 R:75 A:95 D:90 |
| 4 | Confident | `skill` ships visible (not `Hidden`) | Standard says each tool "exposes" the subcommand and the page is published on shll.ai; visibility serves №10 discovery; trivially reversible (flip one field) | S:40 R:90 A:60 D:50 |
| 5 | Confident | Bundle prose authored following the intake's five-section outline; ≤150-line budget pinned by a test | Standard fixes genre + budget; hop-specific content is well-sourced from existing cli memory files; exact wording is apply-time editorial work | S:85 R:85 A:75 D:70 |
| 6 | Confident | Same-change docs upkeep: `cli-surface.md` row + README Reference cross-link | fcvp fix policy (spec updated alongside CLI-surface changes) and readme-extraction cross-link posture; both small and additive | S:55 R:90 A:80 D:75 |

6 assumptions (3 certain, 3 confident, 0 tentative).
