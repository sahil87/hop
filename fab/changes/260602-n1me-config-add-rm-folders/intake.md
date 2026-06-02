# Intake: Config Add / Remove Folders

**Change**: 260602-n1me-config-add-rm-folders
**Created**: 2026-06-03
**Status**: Draft

## Origin

> User (via `/fab-discuss` then `/fab-new for all 3 in one go`): "I want easy ways to add / remove folders from hop yaml. Any ideas?"

Conversational. A `/fab-discuss` session explored the design space and surfaced three candidate capabilities, with the user making explicit choices via structured questions:

1. **Register-after-clone** — wanted, user chose "save by default, `--no-add` opt-out".
2. **Add an existing on-disk folder** — `hop config add <dir>`, on-disk dir only (no URL/auto-detect).
3. **Remove stale / unwanted entries** — interactive fzf picker, including entries whose path no longer exists.

Surface decision: **sub-verbs under `config`** (not new top-level subcommands) — to satisfy Constitution VI (Minimal Surface Area).
Empty-group decision: on removing a group's last URL, **leave the group as an empty placeholder** (schema already permits this; it stays a valid `clone --group` target).

**Gap-analysis correction made during `/fab-new`**: capability #1 (register-after-clone) is **already implemented**. `hop clone <url>` auto-registers into `hop.yaml` by default via `registerURL` (`src/cmd/hop/clone.go:108-223`) using `yamled.AppendURL`, with `--no-add` to opt out and idempotent dedup on re-clone. The discuss-session "decision" to make clone save-by-default already ships. Scope therefore narrows to **#2 (`config add`) + #3 (`config rm`)**.

**Follow-up decisions (post-create, structured Q&A)** — resolved all soft assumptions, lifting intake confidence from 0.0 (gate-fail) to 4.4: (a) `config rm` is **single-select v1**; (b) **no** non-interactive `--stale --yes` in v1; (c) edge cases (non-git dir, URL not found) are **forgiving** (message + exit 0); (d) `config add` **writes by default**. See Assumptions 7, 9, 10, 11, 12.

## Why

**Problem.** Today there are exactly two ways to mutate `hop.yaml`: hand-edit the file, or run a full directory `hop config scan`. There is no *surgical, single-entry* path. To register one already-cloned repo you must either scan its whole parent tree (pulling in siblings you may not want) or open an editor. To remove an entry — e.g. a repo you deleted from disk months ago — your only option is the editor; nothing in the CLI removes a line, and `yamled` has no removal primitive at all.

**Consequence if unaddressed.** `hop.yaml` drifts from reality. Stale entries accumulate (deleted repos still listed → `hop <name>` resolves to a missing path, `hop clone --all` and the fzf picker show ghosts). Adding a single existing repo stays an editor chore, which discourages keeping the config curated — eroding hop's core promise that it's the source of truth for "where my repos are."

**Why this approach.**
- `config add <dir>` is the **inverse granularity** of `scan` (one dir, non-recursive) and reuses scan's entire classify→plan→merge pipeline. Minimal new code, maximal consistency: the same repo lands in the same group whether added singly or via scan.
- `config rm` via fzf leans on hop's **existing interaction model** (the picker already drives bare `hop`, `clone`, `pull`, `push`). "Wrap, don't reinvent" (Constitution IV) — no new selection UI.
- Both live under `config` as sub-verbs, so no top-level surface is added (Constitution VI). They join `init`/`where`/`scan`/`print`.
- Rejected alternative: a URL-accepting `config add <url>`. Ruled out because `clone --no-add`'s inverse (clone *does* register) plus the existing `clone` registration already cover URL entry; adding a URL mode to `config add` would duplicate `registerURL`. `config add` is deliberately **on-disk-dir-only**.
- Rejected alternative: a `--prune` flag on `scan`. Ruled out in favor of `rm --stale` so removal lives with the removal verb, not bolted onto the (additive) scan command.

## What Changes

### `hop config add <dir>` — register a single on-disk repo

A non-recursive, single-directory front-end to the scan pipeline.

- Validates `<dir>` exactly as scan does: `filepath.Clean` → `filepath.EvalSymlinks` → `os.Stat` is-dir. Failure → `hop config add: '<dir>' is not a directory.` exit 2.
- Requires `hop.yaml` to exist (same precondition + message style as scan): missing → guidance to `hop config init` first, exit 1.
- Classifies the single dir via scan's `classifyDir` (`src/internal/scan/scan.go`) — first-match-wins: worktree → skip; normal repo (has `.git` dir, has a remote) → register via `git remote get-url`; bare repo → skip; **plain directory (no `.git`) → not a repo, error/skip** (note: unlike scan, there is no recursion, so a plain dir yields zero repos — surface a clear "not a git repo" message rather than silently doing nothing).
- Builds a one-element `yamled.ScanPlan` via the existing `buildScanPlan` (`config_scan.go:169`) — convention match → `default` group; else invented group (slugify parent dir basename). Then `yamled.MergeScan` (atomic, comment-preserving) — `--write`-equivalent is the **default** for `add` (the user named a specific dir; printing would be odd), though a print/dry-run mode is an open question below.
- Idempotent: URL already present anywhere in `hop.yaml` → dedup'd, "already registered" message, no write (matches scan + `registerURL`).
- `git` lazy-required (only if a `.git` candidate is found), 5s timeout, via `internal/proc` (Constitution I).

```
hop config add ~/code/acme/widget        # register one existing repo into hop.yaml
```

### `hop config rm [--stale]` — interactive removal

Interactive removal of a registered entry via the fzf picker.

- **`hop config rm`** (no arg): loads repos (`repos.FromConfig`), builds picker lines via the existing `buildPickerLines` (`resolve.go:182`, renders `name [group]` with collision disambiguation), pipes to `fzf.Pick`. The selected line maps back to its source `Repo` (same line-match-back logic resolve.go uses), and that repo's **URL** is removed from `hop.yaml`.
- **`hop config rm --stale`**: pre-filters candidates to repos whose resolved `Path` does **not** exist on disk (`os.Stat`), then opens the picker over only those. Directly serves "remove items that no longer exist." If zero stale → friendly "nothing stale" message, exit 0, no picker.
- Removal uses a **new `yamled.RemoveURL(path, group, url)`** primitive (see Impact) — comment-preserving, atomic temp+rename, handles both flat-list and `urls:`-map group shapes.
- **Empty-group placeholder**: removing a group's last URL leaves the (now-empty) group node intact — `default: []` or `mygroup: { dir: ~/x, urls: [] }`. Per user decision; also the simpler implementation (delete one sequence item, stop).
- fzf-missing → reuse `fzfMissingHint` + `errFzfMissing` handling (`resolve.go:24,34`). fzf cancel (Esc/Ctrl-C) → no-op (no file write), exit 130 (via the existing `errFzfCancelled` sentinel → `translateExit`).

```
hop config rm            # picker over all registered repos → remove selected
hop config rm --stale    # picker over only repos missing from disk
```

### Wiring

`newConfigCmd` (`config.go`) gains `newConfigAddCmd()` and `newConfigRmCmd()` alongside the existing four. `config` Short updated to mention add/rm.

## Affected Memory

- `config/yaml-schema`: (modify) — note that removal leaves empty-group placeholders; cross-ref `RemoveURL`.
- `cli/subcommands`: (modify) — document `config add` and `config rm [--stale]` under the `config` parent.
- `config/scan`: (modify) — cross-ref `config add` as the single-dir sibling sharing `buildScanPlan`/`classifyDir`.
- `architecture/package-layout`: (modify) — `yamled` gains `RemoveURL`; `internal/scan` `classifyDir` now consumed by `config add` too.

## Impact

- **`src/internal/yamled/yamled.go`** — net-new `RemoveURL(path, group, url string) error`: round-trip the `yaml.Node` tree, locate the group (flat list or `urls:` map), drop the matching URL scalar node, preserve comments, atomic-write via existing `atomicWrite`. The trickiest behavior is comment attachment (yaml.v3 attaches comments to nodes; removing a node may orphan an adjacent standalone `# comment`) — needs explicit tests. URL-not-found and group-not-found are no-op-with-message vs error: TBD.
- **`src/cmd/hop/config.go`** — register two new subcommands.
- **`src/cmd/hop/config_add.go`** (new) — validation + single-dir classify + `buildScanPlan` + `MergeScan`. Reuses `config_scan.go` helpers (may require lifting a couple of currently-unexported helpers, or calling `runConfigScan`-adjacent code paths).
- **`src/cmd/hop/config_rm.go`** (new) — load repos, `--stale` filter, `buildPickerLines`, `fzf.Pick`, map-back, `RemoveURL`.
- **`src/internal/fzf/fzf.go`** — `Pick` returns a single `string` (no multi-select). Multi-remove (`fzf -m` → `[]string`) would require extending this — see Open Questions / Assumption 6.
- **`internal/scan.classifyDir`** — currently consumed only by `Walk`; `config add` becomes a second caller. Confirm it's callable for a single dir without the walk scaffolding (it takes a canonical path and returns a classification — should be fine).
- No new external dependencies. `git` and `fzf` already wrapped.
- Cross-platform: no new platform-specific code (reuses existing scan/fzf abstractions).

## Open Questions

All resolved during `/fab-new` follow-up (decisions recorded as assumptions below):

- ~~`config add` print vs write default?~~ → **Write by default** (optional `--dry-run` deferred to spec).
- ~~`config rm` single vs multi-select?~~ → **Single-select v1** (`fzf.Pick` unchanged).
- ~~`config rm --stale` non-interactive escape hatch?~~ → **Interactive only v1** (no `--yes`; defer to follow-up).
- ~~`RemoveURL` URL/group not found?~~ → **Forgiving**: message + exit 0.
- ~~`config add` on a non-git dir?~~ → **Forgiving**: message + exit 0.
- ~~`--stale` + worktrees?~~ → Check the repo's own `Path` only (Assumption 8).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Register-after-clone is already shipped (`clone` auto-registers, `--no-add` opts out); NOT in scope | Verified in `clone.go:108-223` + `registerURL` during gap analysis. Discuss-session decision already implemented. | S:95 R:95 A:95 D:95 |
| 2 | Certain | New capabilities are sub-verbs under `config` (`add`, `rm`), not top-level | User chose "sub-verbs under config"; satisfies Constitution VI; joins existing init/where/scan/print. | S:95 R:90 A:95 D:95 |
| 3 | Certain | `config add` accepts a single on-disk **dir only** (no URL mode) | User chose "on-disk dir only"; URL entry covered by `clone`'s registration. | S:90 R:90 A:90 D:90 |
| 4 | Certain | `config rm` is interactive via the existing fzf picker | User chose "interactive fzf picker"; reuses `buildPickerLines` + `fzf.Pick` (Constitution IV). | S:90 R:85 A:90 D:90 |
| 5 | Certain | Removing a group's last URL leaves an **empty-group placeholder** | User chose "leave it"; schema permits empty groups; simpler impl (delete one seq item). | S:90 R:90 A:90 D:90 |
| 6 | Confident | `config add` reuses scan's `classifyDir` + `buildScanPlan` + `MergeScan`; net-new code is `yamled.RemoveURL` + two thin CLI files | Verified these symbols exist and are shaped for reuse. `classifyDir` takes a canonical path (single-dir callable). | S:80 R:75 A:80 D:75 |
| 7 | Certain | `config add` writes by default; optional `--dry-run` previews (deferred to spec) | Discussed — user chose "write by default". User named a specific dir, so writing is expected. | S:90 R:75 A:85 D:90 |
| 8 | Confident | `--stale` checks only the repo's own resolved `Path` on disk, ignoring worktrees | Keeps the staleness check simple and predictable; worktree-aware staleness is out of scope. | S:75 R:70 A:75 D:70 |
| 9 | Certain | `config rm` is single-select in v1 (no `fzf -m` multi-remove); `fzf.Pick` unchanged | Discussed — user chose "single-select v1". Multi-select deferred; avoids `fzf` package changes. | S:90 R:80 A:90 D:90 |
| 10 | Certain | `RemoveURL` not-found (URL or group absent) → no-op + stderr message, exit 0 | Discussed — user chose "forgiving". Matches `registerURL`'s "already registered" tone. | S:90 R:85 A:90 D:85 |
| 11 | Certain | `config add` on a non-git plain dir → message + exit 0 (forgiving, not error) | Discussed — user chose "forgiving". Consistent with `RemoveURL` and `clone` no-op tone. | S:90 R:85 A:90 D:85 |
| 12 | Certain | No `--stale --yes` non-interactive prune in v1; `rm --stale` is always interactive | Discussed — user chose "interactive only v1". Avoids a destructive headless path; defer to follow-up. | S:90 R:85 A:90 D:90 |

12 assumptions (11 certain, 1 confident, 0 tentative, 0 unresolved).
