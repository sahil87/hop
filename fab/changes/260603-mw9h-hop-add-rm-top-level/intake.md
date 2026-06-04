# Intake: Promote `add` and `rm` to top-level commands

**Change**: 260603-mw9h-hop-add-rm-top-level
**Created**: 2026-06-03
**Status**: Draft

## Origin

> Make hop add and hop rm top level commands (instead of hop config add and hop config rm)

Initiated conversationally via `/fab-new` following a `/fab-discuss` orientation session. The raw one-liner was refined through a short clarification dialogue because the request collided with hop's "subcommand xor repo at `$1`" grammar and with Constitution VI (Minimal Surface Area), and because the user's mental model (`hop add/rm <folder-name>`) implied a *symmetric, argument-taking* shape that today's commands do not share.

Key decisions reached in the dialogue (all encoded in Assumptions):

- **Migration**: top-level `hop add` / `hop rm` become canonical; `hop config add` / `hop config rm` are kept as **hidden aliases** (zero breakage for existing scripts and muscle memory). Chosen over a hard removal (cleaner surface but breaks callers) and over documenting both (worst fit with Constitution VI).
- **`hop add <dir>`**: a *pure promotion* — identical behavior to today's `hop config add <dir>` (on-disk directory path, classify, register remote URL). No behavior change.
- **`hop rm [<name>]`**: a promotion **plus** a genuinely new capability — an optional positional. `hop rm <name>` resolves the named repo via the existing match-or-fzf helper and removes it directly; bare `hop rm` retains today's interactive fzf picker. This is the symmetric `add`/`rm` shape the user described, and the only real behavior addition in this change.
- **`rm` naming**: keep the literal `rm` (not `remove`). The grammar collision (a repo literally named `rm`) is already handled by the existing `subNames` collision filter; the only residual concern (the filesystem-`rm`-implies-a-positional expectation) is now *resolved by design* because `hop rm <name>` accepts a positional.

## Why

**Problem.** The two most common registry-editing operations — adding a repo you already have on disk, and removing a stale/unwanted entry — are buried one level deep under `hop config`. `config` is otherwise a namespace for *meta* operations on the config file itself (`init`, `where`, `print`, `scan`). `add` and `rm` are the everyday "manage my repo list" verbs and deserve first-class, top-level placement alongside `clone`, `pull`, `push`, `sync`, `ls`. The extra `config ` prefix is friction on the hottest registry-editing paths.

**Consequence of not fixing.** Users keep typing (and mistyping) a longer command for the two operations they reach for most. The asymmetry with `clone`/`ls` (top-level) vs `config add`/`config rm` (nested) is an inconsistency that makes the CLI harder to learn. The `config rm` picker-only shape also means there is *no* fast path to remove a repo you can already name — you must open fzf and hunt for it.

**Why this approach over alternatives.** Constitution VI requires justifying every new top-level subcommand ("could this be a flag on an existing subcommand, or a separate tool?"). The answer is no on both counts: these are distinct verbs (not flags), and they operate on the in-process registry (not a separable tool). Promotion with hidden aliases is the lowest-risk migration — it adds discoverability without breaking any existing caller, mirroring how the project already keeps backward-compat seams (e.g., legacy `spec.md` ingestion). The positional on `rm` is added now because the user's mental model demands it and because the resolver plumbing (`resolveByName`) already exists — deferring it would ship an `add`/`rm` pair that feels asymmetric and incomplete.

## What Changes

Source lives under `src/cmd/hop/` (module rooted at `src/go.mod`). The `config.yaml` `source_paths` list (`cmd`, `internal`) is relative to `src/`.

### 1. New top-level `hop add <dir>` (promotion, no behavior change)

Add a top-level cobra subcommand `add` that is the canonical spelling of today's `hop config add <dir>`. Behavior is **identical** to `runConfigAdd` (`src/cmd/hop/config_add.go`):

- `Args: cobra.ExactArgs(1)` — one directory path.
- Validates via the shared `validateConfigDir`, resolves `hop.yaml`, classifies via `scan.ClassifyOne`, merges via `buildScanPlan` + `yamled.MergeScan`.
- Same exit-code contract: **2** on bad dir, **1** on missing `hop.yaml` / load / write / `git`-missing failure, **0** on success and on every forgiving no-op (non-git dir, worktree/bare/no-remote, already-registered).
- Same stderr lines: `added: <url>` + `wrote: <path>` on a real add; the forgiving `addSkipMessage` lines otherwise.

**Implementation note for plan**: the cleanest factoring is to have a single `RunE` body (today's `runConfigAdd`) and wire it under *both* the new top-level factory and the existing `config` subtree. The stderr command-prefix constant (`addCmdName = "hop config add"`) needs attention — see Assumption 8 (the canonical command name should become `"hop add"` for the top-level path; the alias may keep emitting `hop config add` or be unified — recorded as Tentative).

### 2. New top-level `hop rm [<name>]` (promotion + new positional)

Add a top-level cobra subcommand `rm` that supersedes `hop config rm`. Two argument shapes:

- **`hop rm` (no positional)** — `cobra.MaximumNArgs(1)`. With zero args, behaves exactly like today's `runConfigRm`: load registry, optional `--stale` pre-filter, fzf picker (`pickRepo` / `pickOne` seam), map-back by unique path column, `yamled.RemoveURL`. Same exit codes (**1** fzf-missing/no-config/load/write-failure, **130** fzf cancel, **0** success and forgiving no-ops) and same stderr (`removed: <url>` + `wrote: <path>`).
- **`hop rm <name>` (one positional)** — **NEW**. Resolve `<name>` to a single `*repos.Repo` via the existing `resolveByName` match-or-fzf helper (`src/cmd/hop/resolve.go`), then call `yamled.RemoveURL(configPath, repo.Group, repo.URL)` directly — skipping the picker. This reuses the resolver that `hop <name> where`, `-R`, and `open` already use, so substring matching and fzf-on-ambiguity come for free and behave consistently with the rest of the CLI.

`--stale` interaction with a positional: `--stale` is a picker-scoping flag. Combining it with a positional name is a usage error (exit 2) — `--stale` only makes sense for the no-arg picker path. (Assumption 9, Tentative.)

**`--stale` resolution semantics for the positional**: `hop rm <name>` removes the registry entry regardless of whether the repo exists on disk (you can prune an entry whose folder you already deleted by naming it). No on-disk existence check on the positional path. (Assumption 6, Confident.)

### 3. Migration: hidden aliases under `config`

Keep `hop config add` and `hop config rm` working as **hidden** aliases (`Hidden: true` on the cobra factories, or equivalent) so they:

- Still execute the same `RunE` (zero breakage for scripts / muscle memory).
- Disappear from `hop config --help` and from `hop --help`.
- Self-filter out of `hop help-dump` JSON output — the existing `shouldSkipChild` already drops `Hidden` children, so the published `help/hop.json` reference will list `add`/`rm` at the top level and omit the `config add`/`config rm` aliases automatically (consistent with how `help-dump` hides itself). Verify this in review.

The `config` parent's `Short` ("config helpers (init, where, scan, add, rm, print)") should drop `add, rm` from its summary since they're no longer the documented home.

### 4. Grammar / shim / completion wiring

Promoting to top-level means `add` and `rm` join the "subcommand xor repo at `$1`" set. Required touch-points:

- **`root.go::newRootCmd()`** — `AddCommand(newAddCmd(), newRmCmd(), ...)` alongside the existing factories.
- **`shell_init.go::posixInit`** — the known-subcommand case (line 51): `clone|pull|push|sync|ls|shell-init|config|update|help|--help|-h|--version|completion` MUST gain `add|rm`. Without this, `hop rm` would fall into the shim's repo-first "otherwise" branch and misroute (`hop rm` → `_hop_dispatch cd "rm"`, and `hop add ~/x` → `command hop -R add ~/x` → `-R: 'add' not found`). This is the same misroute story the memory documents for `pull`/`push`/`sync`.
- **`repo_completion.go`** — the `subNames` collision filter (line 74-82) is built dynamically from `cmd.Commands()`, so it picks up `add`/`rm` automatically; a repo literally named `add` or `rm` is dropped from bare-name completion (reachable via `hop add where` / `hop -R add ...`). No code change expected here, but confirm in review.
- **`rootLong` help text** (`root.go`) — add `hop add <dir>` and `hop rm [<name>]` rows to the `Usage:` table; the `config` rows for `add`/`rm` are removed (they become hidden).

### 5. Tests

- New top-level command tests mirroring `config_add_test.go` / `config_rm_test.go` (the `pickOne` seam and `gitRunner` injection points already exist and are reusable).
- A new test for the `hop rm <name>` positional path: inject a resolvable name, assert direct `RemoveURL` without invoking the picker seam.
- `integration_test.go` updates: exercise `hop add <dir>` and `hop rm <name>` end-to-end; assert `hop config add`/`hop config rm` still work (hidden alias) and are absent from `--help`.
- Per Constitution (Test Integrity) and `code-quality.md` (test-alongside): tests conform to the spec, written alongside.

## Affected Memory

- `cli/subcommands`: (modify) Add `hop add <dir>` and `hop rm [<name>]` rows to the Inventory table; document the new `rm <name>` positional path and its resolver reuse; move the `config add`/`config rm` rows to a "hidden alias" note; update the "Removed subcommands" / grammar narrative. Update the `shell-init` known-subcommand list documentation (the `add|rm` additions) and the `help-dump` child-list (now eight → ten top-level user-facing subcommands; confirm exact count).
- `architecture/package-layout`: (modify) Note the new top-level `add`/`rm` factories and where their `RunE` bodies live (shared with the hidden `config` aliases). Likely no new files — the new factories can live in `config_add.go` / `config_rm.go` or be split out; recorded as a Tentative file-placement decision (Assumption 7).
- `cli/match-resolution`: (modify, minor) Note that `hop rm <name>` is a new consumer of `resolveByName` (the match-or-fzf algorithm), alongside `where` / `-R` / `open` / `clone`.

## Impact

- **Code**: `src/cmd/hop/root.go` (wiring + help text), `src/cmd/hop/config.go` (alias hiding + `Short` text), `src/cmd/hop/config_add.go` and `src/cmd/hop/config_rm.go` (new top-level factories + `rm <name>` positional logic + command-name constant handling), `src/cmd/hop/shell_init.go` (known-subcommand list), plus adjacent `_test.go` files and `integration_test.go`.
- **No `internal/` changes expected** — `scan.ClassifyOne`, `yamled.MergeScan`/`RemoveURL`, `repos.FromConfig`, `resolveByName` are all reused as-is.
- **External contract**: published `help/hop.json` (shll.ai reference) changes shape — `add`/`rm` move to the top level, `config add`/`config rm` disappear from the tree (hidden). CI republishes on the next `v*` tag — no action needed in this change beyond verifying the `help-dump` output.
- **Dependencies**: none added (Constitution III — no new flags/deps without justification; this adds no deps).
- **Cross-platform**: no platform-specific code touched.

## Open Questions

*(All intake open questions resolved during the 2026-06-03 clarification session — see `## Clarifications`.)*

- ~~Should the top-level command-name prefix in stderr be `hop add` / `hop rm`?~~ **Resolved**: canonical commands emit `hop add:` / `hop rm:`; hidden aliases keep their historical `hop config add:` / `hop config rm:` prefix (Assumption 8, now Certain).
- ~~Should `hop rm <name>` confirm before removing?~~ **Resolved**: no confirmation prompt — parity with the picker; it's a reversible YAML edit, not a disk delete (Assumption 5, now Certain).

## Clarifications

### Session 2026-06-03

| # | Action | Detail |
|---|--------|--------|
| 5 | Confirmed | No confirmation prompt on `hop rm <name>` — parity with picker; reversible YAML edit |
| 7 | Confirmed | Factories live in existing `config_add.go` / `config_rm.go` (shared `RunE`) |
| 8 | Confirmed | Canonical prefix `hop add:` / `hop rm:`; aliases keep `hop config add:` / `hop config rm:` |
| 9 | Confirmed | `--stale` + positional name = usage error (exit 2) |

### Session 2026-06-03 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 1 | Confirmed | — |
| 2 | Confirmed | — |
| 3 | Confirmed | — |
| 4 | Confirmed | — |
| 6 | Confirmed | — |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Migration strategy: top-level `add`/`rm` canonical; `config add`/`config rm` kept as **hidden** cobra aliases (same `RunE`, hidden from help + help-dump). | Clarified — user confirmed (originally discussed: chose "Move, keep aliases" over hard-removal and over documenting both). | S:95 R:70 A:80 D:85 |
| 2 | Certain | `hop add <dir>` is a pure promotion of `hop config add <dir>` — identical behavior, exit codes, and stderr. | Clarified — user confirmed (originally discussed: "On-disk dir path (same as today)"). Existing `runConfigAdd` is the spec; nothing new to design. | S:95 R:80 A:90 D:90 |
| 3 | Certain | `hop rm <name>` (NEW positional) resolves `<name>` to a `*repos.Repo` and removes it directly; bare `hop rm` keeps the fzf picker. | Clarified — user confirmed (originally discussed: "Add positional: hop rm <name>"). The genuine behavior addition. | S:95 R:55 A:75 D:80 |
| 4 | Certain | `hop rm <name>` reuses the existing `resolveByName` match-or-fzf helper (substring match + fzf-on-ambiguity), consistent with `where`/`-R`/`open`/`clone`. | Clarified — user confirmed. `resolveByName` is the established name-resolution seam; reusing it is the consistent choice. | S:95 R:75 A:90 D:80 |
| 5 | Certain | `hop rm <name>` does NOT prompt for confirmation (parity with the picker, which is its own implicit confirmation). | Clarified — user confirmed "No prompt": it's a reversible YAML edit, not a disk delete; matches hop's no-friction spirit. | S:95 R:70 A:55 D:50 |
| 6 | Certain | `hop rm <name>` removes the registry entry regardless of on-disk existence (no Stat check on the positional path); `--stale` remains a picker-only filter. | Clarified — user confirmed. `RemoveURL` is a pure YAML edit; naming an entry to prune it (even after deleting its folder) is the natural semantics. | S:95 R:75 A:80 D:75 |
| 7 | Certain | New top-level factories (`newAddCmd`/`newRmCmd`) live in the existing `config_add.go`/`config_rm.go` files (shared `RunE`), not new files. | Clarified — user confirmed: keeps the shared body in one place. | S:95 R:85 A:70 D:55 |
| 8 | Certain | Canonical stderr prefix becomes `hop add:` / `hop rm:`; hidden aliases keep their historical `hop config add:` / `hop config rm:` prefix. | Clarified — user confirmed: per-path prefix preserves historical accuracy for the aliases while the canonical commands self-identify. | S:95 R:80 A:65 D:55 |
| 9 | Certain | `--stale` combined with a positional name is a usage error (exit 2). | Clarified — user confirmed: `--stale` is a picker-scoping flag; pairing it with an explicit name is contradictory. | S:95 R:75 A:70 D:60 |
| 10 | Certain | Shim known-subcommand list (`shell_init.go::posixInit` line 51) MUST gain `add|rm`; `root.go` wiring + `rootLong` help table updated; `repo_completion.go` `subNames` filter picks them up automatically. | Determined by the codebase grammar — without the shim entries, `hop rm`/`hop add` misroute through the repo-first branch (documented precedent for `pull`/`push`/`sync`). No judgment call. | S:90 R:60 A:95 D:90 |
| 11 | Certain | No `internal/` package changes; no new dependencies. Reuses `scan.ClassifyOne`, `yamled.MergeScan`/`RemoveURL`, `repos.FromConfig`, `resolveByName`. | Determined by the existing architecture — all needed primitives exist. Constitution III/IV (no new deps, wrap don't reinvent). | S:85 R:85 A:95 D:90 |

11 assumptions (11 certain, 0 confident, 0 tentative, 0 unresolved).
