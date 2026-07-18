# Intake: Adopt the Toolkit `skill` Standard (`hop skill`)

**Change**: 260717-armh-adopt-skill-standard
**Created**: 2026-07-18

## Origin

One-shot invocation: `/fab-new armh` (backlog ID). Backlog entry `[armh]` (2026-07-18), quoted in full:

> Adopt the toolkit `skill` standard — add a `hop skill` subcommand that prints a stable, ≤150-line, static, agent-facing usage bundle to stdout, byte-identical to a canonical `docs/site/skill.md` (renders at /hop/skill on shll.ai for free). Deferred (NOT a violation yet) per the standard's own phased per-repo adoption clause ("No tool ships `skill` today… a tool without a `skill` subcommand is not yet in violation") and the fcvp intake's pre-decided disposition. When adopting: command name is exactly `skill` (not `agent`/`context`); stdout-only, stderr empty, exit 0; content is a usage briefing (when-to-use, capabilities map keyed to subcommands, composition patterns with wt/fzf/git per №7, stdout/stderr + exit-code contract, gotchas) — NOT a README clone or flag table (defer those to `-h`/`help-dump`); embed via the sync + drift-guard pattern `shll standards` established (committed embedded copy + sync script + a drift-guard test failing the build on divergence). This is `hop`'s slice of the 7-tool phased rollout. Reference: the fcvp conformance report's `skill` section names this as the deferral target.

Intake-time verification (2026-07-18): `shll standards skill` read in full (the authoritative contract). The shll repo is present locally at `~/code/sahil87/shll` — its `scripts/sync-standards.sh`, `src/cmd/shll/standards.go` (`//go:embed` + `//go:generate`), and `TestStandardsEmbedMatchesCanonical` were read as the pattern to mirror. hop has no `skill` code today (`grep -ri skill src/ --include=*.go` → empty) and no `docs/site/skill.md`. Neither shll nor any other toolkit tool ships its own `skill` subcommand yet, consistent with the standard's "No tool ships `skill` today" — hop is the first adopter, so the standard text (not a sibling implementation) is the conformance reference.

## Why

Toolkit CLI principle №10 (agent-discoverable documentation) obligates each tool to serve the agent *using* it. The three existing surfaces each fall short for that caller: `-h`/`help-dump` is flag/tree reference (structure, not judgment), README/`docs/site` needs a checkout or a network round-trip, and `fab/project` context is repo-*development*-scoped. The `skill` standard closes that gap: an embedded, offline, version-locked-by-construction usage briefing — the prose ships inside the same binary as the flags it describes, so it can never document a capability the installed binary lacks.

The fcvp conformance change (PR #50, merged) audited hop against all four toolkit standards and recorded exactly one №10 gap deferred to this backlog ID: `hop skill` ("deferred, not yet adopted" per the standard's phased-adoption clause and the fcvp intake's §4 pre-decided disposition). This change is hop's slice of the 7-tool phased rollout, on hop's own release cadence.

If we don't adopt: hop stays conformant-by-grace-period but agents operating an installed hop keep guessing at the selection-first grammar, the shell-only verb split, and the exit-code contract from flag dumps — precisely the failure mode the standard exists to prevent. And when `shll agent-setup` (the standard's forward design) starts aggregating every installed tool's `<tool> skill` output, hop would be the silent hole in the aggregate.

## What Changes

### 1. Canonical bundle: `docs/site/skill.md` (new)

The canonical agent skill bundle, authored at apply time in the standard's genre — a **usage briefing**, not a README clone and not a flag table. Hard constraints from the standard:

- **≤150 lines** (hard budget; the standard's rationale: bundles are later aggregated across all installed tools, so every line is paid N times).
- **Static only** — byte-identical on every invocation, no timestamps, no environment lookups (contrast `run-kit context`'s dynamic Environment header; that genre stays out).
- Renders at `https://shll.ai/hop/skill` for free (part of the pulled `docs/site/**` tree) — no extra publishing work.

Content outline (the standard's five sections, instantiated for hop; final prose is apply-time work):

- **When to use**: locating, opening, and batch-operating registered repos from `hop.yaml`; when NOT (worktree lifecycle → `wt`; unregistered/ad-hoc dirs → plain `cd`).
- **Capabilities map** keyed to the selection-first grammar `hop <selection> <action>`: bare `hop` picker, `<name> cd|where|open`, batch verbs `pull|push|sync` over `<name>`/`<group>`/`--all`, `ls [--json]`, `clone`, `add`/`rm [--dry-run]`, `config` family, `update`, `shell-init`.
- **Composition patterns** (principle №7): the shim + `eval "$(hop shell-init zsh)"` parent-shell model, `wt` for the open menu, `fzf` for pickers, `git` under the batch verbs; agent-side composition via `hop ls --json` and `hop <name> where`.
- **Output & exit-code contracts**: stdout is data / stderr is diagnostics; exit `0` success, `1` application error, `2` usage error, `3` no TTY (distinct from cancel), `130` fzf cancelled.
- **Gotchas**: `cd` and tool-form are shell-only (shim required — without it the binary errors with a hint); no-TTY paths exit 3 instead of hanging; unique substring matches short-circuit the picker; hop is stateless (re-reads `hop.yaml` every invocation, so retries are safe).

Existing memory (`docs/memory/cli/agent-non-interactive-usage.md`, `cli/subcommands.md`, `cli/match-resolution.md`) is the source material for the gotchas/contract sections — the bundle condenses what those files already record.

### 2. `hop skill` subcommand (`src/cmd/hop/skill.go`, new)

A cobra command mirroring the tone of `help_dump.go`:

- `Use: "skill"` — the name is fixed by the standard (exactly `skill`, not `agent`/`context`).
- `Args: cobra.NoArgs`, no flags.
- Visible (NOT `Hidden`, unlike `help-dump`) — the standard says each tool "exposes" the subcommand, the bundle is a published page on shll.ai, and visibility serves №10 discovery. It therefore appears in `hop --help` and the help-dump tree.
- `RunE` writes the embedded bundle bytes to `cmd.OutOrStdout()` verbatim — no rendering, no pager, no added framing (principle №2: stdout is data). stderr empty on success, exit 0.
- Embed via the shll pattern (the Go module root is `src/`, and `//go:embed` cannot reach `docs/site/` above it — same layout constraint shll documents in `standards.go`):

```go
//go:generate ../../../scripts/sync-skill.sh

//go:embed skill.md
var skillBundle []byte
```

- Committed embedded copy at `src/cmd/hop/skill.md` (synced from canonical; committed so a clean `go build ./...` compiles without running the script).
- Registered in `root.go`'s `AddCommand(...)` list.

**Constitution VI justification (required for any new top-level subcommand)**: "could this be a flag on an existing subcommand, or a separate tool?" — **No.** The standard's invocation contract fixes a uniform top-level `<tool> skill` subcommand across all seven tools with exactly this name; a flag or external tool would not satisfy it, and the bundle must ship inside hop's own binary to be version-locked.

### 3. Sync script + justfile recipe (new)

`scripts/sync-skill.sh`, modeled line-for-line on shll's `scripts/sync-standards.sh` (Constitution V: thin justfile, logic in scripts/):

```bash
#!/usr/bin/env bash
# Copy the canonical docs/site/skill.md into src/cmd/hop/ so it can be embedded
# via //go:embed (module root is src/, docs/site/ sits above it).
set -euo pipefail
cd "$(dirname "$0")/.."
cp -f docs/site/skill.md src/cmd/hop/skill.md
echo "synced src/cmd/hop/skill.md from docs/site/skill.md"
```

justfile gains a one-liner recipe:

```
# Refresh the embedded copy of docs/site/skill.md (run after editing the bundle).
sync-skill:
    ./scripts/sync-skill.sh
```

### 4. Drift guard + contract tests (`src/cmd/hop/skill_test.go`, new)

Mirroring shll's `TestStandardsEmbedMatchesCanonical`:

- **Drift guard**: embedded `skill.md` bytes MUST equal `../../../docs/site/skill.md` (test file lives at `src/cmd/hop/`); on divergence, fail naming the fix (`just sync-skill` / `scripts/sync-skill.sh`). Runs on every `go test ./...` and in the existing CI PR workflow — this is the "drift-guard test failing the build on divergence" the standard mandates.
- **Invocation contract**: `skill` runs with exit-0 semantics (RunE returns nil), stdout byte-identical to the embedded copy, stderr empty, no args accepted.
- **Budget guard**: the bundle is ≤150 lines — pins the standard's hard budget so a future edit can't silently blow it.

`help_dump_test.go` pins document fields, not the top-level command list, so the new visible command requires no test edits there — but the help-dump "Verifying conformance" checklist is re-run after the tree change (fcvp fix-policy convention), recorded as an acceptance item.

### 5. Docs upkeep (modify)

- **`docs/specs/cli-surface.md`**: add the `hop skill` row (args: none; streams: bundle markdown → stdout, stderr empty; exit 0) — the fcvp fix policy requires CLI-surface changes to update this spec in the same commit.
- **`README.md`**: add a Docs-section line (alongside the existing install/workflows links, ~line 254) pointing at `docs/site/skill.md` — keeps the readme-extraction cross-link posture (README cross-links its `docs/site/` pages).

## Affected Memory

- `cli/subcommands`: (modify) new `skill` subcommand — invocation contract, visibility, embed mechanism
- `cli/agent-non-interactive-usage`: (modify) `hop skill` joins the agent-facing surface (offline usage briefing alongside `ls --json` and `help-dump`)
- `build/local`: (modify) `just sync-skill` recipe + `scripts/sync-skill.sh` + drift-guard test in the local build/test loop

## Impact

- **New files**: `docs/site/skill.md` (canonical bundle), `src/cmd/hop/skill.go`, `src/cmd/hop/skill.md` (committed embed copy), `src/cmd/hop/skill_test.go`, `scripts/sync-skill.sh`.
- **Modified files**: `src/cmd/hop/root.go` (one `AddCommand` entry), `justfile` (one recipe), `docs/specs/cli-surface.md` (one row), `README.md` (one Docs link).
- **No behavior change** to any existing command; purely additive surface. No new dependencies (uses stdlib `embed` — precedent: `src/internal/config/starter.yaml`).
- **Verification** (the standard's own checklist): `hop skill` exits 0, bundle to stdout only, stderr empty; stdout byte-identical to `docs/site/skill.md`; ≤150 lines, no dynamic content; genre check (usage briefing, not README/flag table); help-dump checklist re-verified after the tree change. Tests: `cd src && go test ./...`.

## Open Questions

- None — the backlog entry, the standard text (`shll standards skill`), and the locally readable shll implementation resolve all decision points.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Command is exactly `skill`, raw markdown to stdout, stderr empty, exit 0, no flags/args | Standard's invocation contract is explicit and uniform; backlog restates it verbatim | S:95 R:80 A:95 D:95 |
| 2 | Certain | Embed via committed copy `src/cmd/hop/skill.md` + `scripts/sync-skill.sh` + drift-guard test + `//go:generate`, mirroring shll's sync-standards pattern | Standard mandates "reuse the exact mechanism `shll standards` established"; same module-root layout constraint applies (go:embed cannot reach docs/site/); pattern read directly from the shll repo | S:90 R:75 A:95 D:90 |
| 3 | Certain | Constitution VI satisfied: new top-level subcommand justified — name and shape fixed by the toolkit standard, cannot be a flag or separate tool | The standard's cross-tool uniform contract answers the "could it be a flag?" test with "no" | S:90 R:60 A:90 D:95 |
| 4 | Confident | `skill` ships visible (not `Hidden`, unlike `help-dump`) | Standard says each tool "exposes" the subcommand and the page is published on shll.ai; visibility serves №10 discovery; trivially reversible (flip one field); no sibling-tool precedent exists yet either way | S:40 R:90 A:60 D:50 |
| 5 | Confident | Bundle prose is authored at apply following the intake's five-section outline; the ≤150-line budget is pinned by a test | Standard fixes genre + budget; hop-specific content is well-sourced from existing cli memory files; exact wording is apply-time editorial work | S:85 R:85 A:75 D:70 |
| 6 | Confident | Same-change docs upkeep: `cli-surface.md` row + README Docs-section cross-link | fcvp fix policy (spec updated alongside CLI-surface changes) and readme-extraction cross-link posture; both small and additive | S:55 R:90 A:80 D:75 |

6 assumptions (3 certain, 3 confident, 0 tentative, 0 unresolved).
