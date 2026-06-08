# Registering Repos via `hop add`

How `hop add` registers on-disk git repos into `hop.yaml` — at two breadths (single dir, or a recursive `-r` tree walk) and two sinks (write by default, or `-p` dry-run print). This is **one command, two breadths** — there is no separate `scan` subcommand. The CLI lives in `src/cmd/hop/config_add.go` (factories `newAddCmd` / `newConfigAddCmd`, backed by the shared `runAdd`); the shared plan-building helpers (`validateConfigDir`, `buildScanPlan`, slugify, conflict resolution) live in `src/cmd/hop/config_scan.go`; the filesystem walk lives in `src/internal/scan/scan.go`; YAML emission goes through `src/internal/yamled/yamled.go::MergeScan` / `RenderScan`.

History: recursion was unified onto `hop add` and the standalone `hop config scan` command was **deleted with no alias** in change `260608-w2bj-unify-recursive-add`. The DFS/classification mechanics described below are carried over unchanged from the original scan design — spec [`fab/changes/260506-ceh2-config-scan-populate-repos/spec.md`](../../../fab/changes/260506-ceh2-config-scan-populate-repos/spec.md) and intake [`fab/changes/260608-w2bj-unify-recursive-add/intake.md`](../../../fab/changes/260608-w2bj-unify-recursive-add/intake.md).

## Overview

`hop add <dir>` discovers repos under (or at) `<dir>`, classifies each candidate directory, assigns each to a group, and either merges them into `hop.yaml` (default) or renders the plan to stdout (`-p`). Both breadths share the same plan-build + render path — the only difference between single-dir and recursive is which `scan` entry point produces the `[]scan.Found` slice.

`hop add` is the **canonical top-level** command; `hop config add` is its **hidden alias** sharing the exact same `runAdd` body (differing only in the per-path stderr prefix `"hop config add"` vs `"hop add"`) — see [cli/subcommands § Migration: hidden config aliases](../cli/subcommands.md#migration-hidden-config-aliases).

### Flags

`runAdd` takes a parsed `addOpts{recursive, print, depth, group}` (bound by the shared `bindAddFlags`, so the canonical command and the hidden alias expose the identical surface):

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `-r`, `--recursive` | bool | `false` | DFS-walk `<dir>` for git repos (via `scan.Walk`) instead of classifying just `<dir>` (`scan.ClassifyOne`). |
| `-p`, `--print` | bool | `false` | Render the merge plan to stdout instead of writing to `hop.yaml` (a dry-run). Valid at **both** breadths — `-p` does NOT require `-r`. |
| `--depth N` | int | `3` | Maximum DFS depth (root is depth 0; bound is **inclusive** — `--depth 3` examines up through `R/*/*/*`). Only meaningful with `-r`, but validated whenever supplied. `N < 1` → exit 2 with `<cmdName>: --depth must be >= 1.`. |
| `-g`, `--group <name>` | string | `""` | Force ALL discovered repos into the named group, auto-creating it if absent (write mode). Valid at both breadths. |

### Resulting grammar

```
hop add <dir>                    # single dir, write          (zero-flag baseline)
hop add -r <dir>                 # whole tree, write          (the onboarding bootstrap)
hop add -r -p <dir>              # whole tree, print only      (recursive dry-run)
hop add -p <dir>                 # single dir, print only      (single-dir dry-run)
hop add -r --depth 2 <dir>       # bounded recursion
hop add -g vendor <dir>          # force group (any breadth)
hop add -r -g work ~/clients     # tree into a forced group
```

**Write is the default at every breadth.** `-p` is the opt-in dry-run. (This inverts the deleted `config scan`'s print-by-default behavior — a first scan run appearing to do nothing was the biggest onboarding papercut.) Onboarding collapses to a single command: **`hop add -r <dir>`**.

`code_root` is **never** modified by `add` — it is durable and load-bearing. A user can `hop config init` to set `code_root` first, but it is not required: `hop add -r` auto-inits a missing config (see [`hop.yaml` precondition](#hopyaml-precondition)).

## `runAdd` flow

`runAdd(cmd, cmdName, userArg, opts)` (`config_add.go`) is the single body for both spellings. Step order:

1. **`--depth` validation** — `opts.depth < 1` → `fmt.Fprintf(stderr, "%s: --depth must be >= 1.\n", cmdName)` and `*errExitCode{code: 2}`. Validated up front (whenever supplied) so a bad value never silently no-ops, even without `-r`.
2. **`<dir>` validation** — the shared `validateConfigDir(userArg, cmdName, stderr)` (see [Argument validation](#argument-validation)). Failure → exit 2.
3. **Resolve `hop.yaml`** — `resolveAddConfig(stderr, cmdName, opts.print)` (see [`hop.yaml` precondition](#hopyaml-precondition)). Print mode resolves read-only; write mode resolves the write target and auto-inits.
4. **Load existing config** — `config.Load(configPath)` (for the convention check, dedup, and the `-g` group-exists check). Load error → `errSilent` (exit 1).
5. **Discover repos** — `discoverRepos(canonicalDir, opts, ...)` returns `([]scan.Found, []scan.Skip, error)` for the chosen breadth (see [Discovery](#discovery-single-dir-vs-recursive)).
6. **Build the plan** — `buildAddPlan(cfg, found, opts, ...)` returns a `yamled.ScanPlan` + a `scanPlanSummary` (see [Group assignment](#group-assignment) and [`-g` forced group](#-g-forced-group-auto-create)).
7. **Render or write** — `-p` → `yamled.RenderScan` to `cmd.OutOrStdout()` prefixed by the [print header](#print-mode-header); else (and only when the plan is non-empty) `yamled.MergeScan`.
8. **Stderr summary** — `emitAddSummary(...)` last (after stdout in print mode).

Returns `errSilent` / `*errExitCode` on user-visible failures; `nil` on success **and on every forgiving no-op** (non-git dir, worktree/bare/no-remote skip, already-registered URL).

## Argument validation

The single positional `<dir>` is normalized in this order before any further processing, via the shared `config_scan.go::validateConfigDir(userArg, cmdName, stderr)`:

1. `filepath.Abs(<dir>)` — joins a relative argument with the process CWD (and runs `Clean`), so the returned canonical path is always **absolute**. Failure → usage error. (Change `260605-c92v-fix-relative-dir-args` replaced the prior `filepath.Clean` step with `filepath.Abs`; `EvalSymlinks` preserves a relative input as relative, which left `Found.Path` relative and broke group derivation / convention matching — so a bare relative name like `hop add fab-kit` from its parent dir now resolves correctly instead of failing with `cannot derive group name from parent dir '.'`.)
2. `filepath.EvalSymlinks(<abs>)` — resolves symlinks. Failure (including ENOENT) → usage error.
3. `os.Stat(<resolved>)` — must indicate a directory; otherwise usage error.

The exact stderr on validation failure (with the user-supplied form, not the cleaned/resolved form), prefixed by the caller's `cmdName`:

```
hop add: '<dir>' is not a directory.
```

Exit 2. No `git` invocation occurs on a failed validation (Constitution I).

## `hop.yaml` precondition

How `add` resolves (or creates) `hop.yaml` before discovery depends on the **sink**, not the breadth — `resolveAddConfig(stderr, cmdName, print)`:

- **Write mode (default, no `-p`)** auto-inits a missing config (change `260605-44hm-auto-init-on-write`). It calls `config.ResolveWriteTarget()` (no stat — only errors on `$HOME` unset) then `config.EnsureSkeleton(path)`, which writes the minimal `repos: {}` skeleton when absent and announces `created: <path>` to stderr (see [init-bootstrap § Auto-init-on-write](init-bootstrap.md#auto-init-on-write-change-260605-44hm-auto-init-on-write)). The merge then proceeds against the now-existing file. So `hop add -r ~/code` on a fresh machine creates the config and registers discovered repos in one step — no `config init` prerequisite.
- **Print mode (`-p`)** never touches the file, so there is nothing to bootstrap — it calls `config.Resolve()` and **errors on absence**. The message is the refined resolver text (change `260605-44hm-auto-init-on-write`), surfaced under the add prefix:

  ```
  hop add: hop: no hop.yaml found at <path>. Run 'hop add <dir>' to register a repo (creates the config), or 'hop config init' for a starter.
  ```

  `<path>` is the fixed `~/.config/hop/hop.yaml`. Exit 1. No write, no auto-init.

This is the same read-vs-write split the deleted `config scan` had (print mode errored, `--write` auto-inited) — just relocated to the `-p` flag with the default inverted to write.

## Discovery (single-dir vs. recursive)

`discoverRepos(canonicalDir, opts, ...)` returns the `Found`/`Skip` slices for the chosen breadth, both feeding the *same* downstream plan path (the `Found` slice is one element from `ClassifyOne`, or many from `Walk`):

- **Recursive (`-r`)** → `scan.Walk(ctx, canonicalDir, scan.Options{Depth: opts.depth, GitRunner: gitRunner})` — the depth-bounded, symlink-following, (dev,inode)-dedup DFS. Returns all discovered repos + classified skips.
- **Single-dir (no `-r`)** → `scan.ClassifyOne(ctx, canonicalDir, opts)`. When `isRepo` → a one-element `[]scan.Found`. When `!isRepo` (plain dir, or a worktree/bare/no-remote candidate) → the forgiving no-op message via `addSkipMessage` and **empty slices** (exit 0).

A discovery error is mapped by `addGitError`: `proc.ErrNotFound` → the shared `gitMissingHint` (`hop: git is not installed.`); anything else → `<cmdName>: <err>`. Both return `errSilent` (exit 1).

### DFS algorithm and depth handling

`scan.Walk` (in `src/internal/scan/scan.go`) implements a stack-based DFS using `stackEntry{path, depth}`. The root is enqueued at depth 0. For each popped entry:

1. If `depth > opts.Depth` → skip (do not descend, do not register).
2. `os.Stat(path)` (resolves symlinks) — if it fails or the entry isn't a directory, skip silently.
3. **(dev, inode) dedup**: keyed by `syscall.Stat_t.{Dev,Ino}`. If the inode is already in the visited set → skip silently (loop suppression — not a user-facing skip).
4. `filepath.EvalSymlinks(path)` to canonicalize before classification.
5. `classifyDir(canonical)` → first-match-wins (see Repo classification below).
6. After classifying as a repo (or skip), do **not** descend into the directory's children — repos' children are never themselves repos.
7. Otherwise (plain dir): enqueue immediate subdirectories at `depth+1` in **reverse lexical order** so the DFS pop order yields lexical visit order (deterministic for tests and slug-tie tiebreaking).

`scan.ClassifyOne(ctx, dir, opts) (found Found, skipReason string, isRepo bool, err error)` is the **single-dir entry point** — it applies the *exact* unexported `classifyDir` + `inspectRepo` logic `Walk` runs per directory (no recursion, `opts.Depth` ignored). See [architecture/package-layout § internal/scan](../architecture/package-layout.md#internalscan) for the exported-seam rationale.

### Repo classification rules

Implemented in `scan.go::classifyDir`. First-match-wins:

1. **Worktree** — `D/.git` exists and is a regular file. Skip with reason `"worktree"`. Do not descend.
2. **Normal repo** — `D/.git` is a directory. Invoke `git -C D remote`; if empty → skip with reason `"no remote"`. Otherwise pick `origin` if listed (else first non-empty line); invoke `git -C D remote get-url <selected>` for the URL; emit `Found{Path: canonical(D), URL: trim(out)}`. Do not descend.
3. **Bare repo (heuristic)** — `D` contains `HEAD` (regular file), `config` (regular file), and `objects/` (directory) at top level, AND `D/.git` does not exist. Skip with reason `"bare repo"`. Do not descend. Stat-only — does not shell out to `git rev-parse --is-bare-repository`.
4. **Plain directory** — none of the above; recurse into children at `depth+1`.

### Submodule handling

`ReasonSubmodule` is reserved in the public Skip enum but **never emitted by the current implementation**. The `internal/scan` walker relies solely on the no-descent invariant from rule 2: once a directory is classified as a normal repo, Walk never enqueues its children, so a nested `.git` inside a parent repo is unreachable through DFS. This was an explicit choice (spec assumption #17 permits "the implementation MAY rely solely on the no-descent invariant if it materially simplifies code"). The constant remains exported for forward compatibility.

If a user passes a submodule path directly as `<dir>` (recursive root, or single-dir `add`), behavior depends on the submodule's `.git` shape (per rule 1 vs. rule 2 above):

- **Standard git submodules** (the typical case) have `.git` as a *file* containing `gitdir: ../.git/modules/<name>`. These hit **rule 1 (worktree)** in `classifyDir` and are skipped with reason `"worktree"`.
- **Nested checkouts with a real `.git` directory** (less common — e.g., a manually cloned repo placed inside another's tree) hit **rule 2 (normal repo)** and are registered as Found.

The classifier does not distinguish "submodule" from "git worktree" — both surface `.git` as a regular file and both route through rule 1's `"worktree"` skip.

### Symlinks and loop detection

- Symlinks are followed during the walk (intentional — users symlink directories for Time Machine, network mounts, ad-hoc aliases).
- Loops dedup'd via `(device, inode)` of the canonical directory (`syscall.Stat_t`). On hit → silent skip (no `Skip` entry). Standard `find -L` approach.
- Each `Found.Path` is the `filepath.EvalSymlinks` resolution. The same repo reached via two paths is registered exactly once.

### Git invocation contract

All `git` invocations route through `internal/proc.RunCapture(ctx, dir, "git", args...)` (Constitution Principle I — no direct `os/exec` outside `internal/proc`). The `GitRunner` field on `scan.Options` is the injectable seam; production binds the package var `gitRunner` (in `config_scan.go`, bound to `proc.RunCapture`), tests inject a fake.

Each invocation gets a 5-second timeout via `context.WithTimeout(ctx, 5*time.Second)` (constant `gitTimeout` in `scan.go`).

`git` is **lazy-checked**: it is only required when discovery actually finds a `.git` candidate that requires `git remote`. Empty trees (zero `.git` discoveries) succeed with exit 0 and no `git` invocation. When `git` is missing on PATH AND a `.git` candidate is encountered, the CLI emits `hop: git is not installed.` (the shared `gitMissingHint`, also used by `hop clone`) and exits 1. The walk halts on the first `git`-not-found rather than continuing.

## Group assignment

The CLI layer assigns each `Found` to a group; this logic is **not** in `internal/scan`, which stays UI-free. `buildAddPlan(cfg, found, opts, ...)` dispatches:

- **`-g <name>` set** → `buildForcedGroupPlan` (see [`-g` forced group](#-g-forced-group-auto-create)).
- **Single-dir, non-recursive, no `-g`** → preserves the explicit idempotency reporting (see [Single-dir idempotency](#single-dir-idempotency-reporting)).
- **Otherwise** (recursive, or any breadth with no `-g`) → the shared `buildScanPlan` (convention/invented logic below).

### Convention check

For each `Found{Path, URL}` (`config_scan.go::matchesConvention`):

1. `org := repos.DeriveOrg(URL)`, `name := repos.DeriveName(URL)`.
2. `convention := filepath.Join(repos.ExpandDir(cfg.CodeRoot, ""), org, name)` (org dropped when empty; `code_root` defaults to `~` when unset).
3. Both sides run through `filepath.EvalSymlinks` before string comparison. This handles platforms where `$HOME` (or its ancestors) is itself symlinked — e.g., macOS, where `t.TempDir()` threads through `/var/folders → /private/var/folders`. EvalSymlinks failure (e.g., the convention dir doesn't exist on disk yet) falls back to `filepath.Clean`.
4. Match → assign to the `default` flat group (URL only, no per-repo `dir:`).
5. No match → invented group (next section).

### Invented group naming (slugify)

Slugify rule (`config_scan.go::slugifyGroupName`):

1. `base := filepath.Base(filepath.Dir(Path))` — the parent dir basename.
2. Lowercase.
3. Replace any run of characters not matching `[a-z0-9_-]` with a single `-`.
4. Trim leading and trailing `-` AND `_`. The extended trim charset (`-_`) is required so all-underscore input (`___`) trims to empty per the spec's pathological-input examples; internal `_` runs are preserved.
5. If empty → skip the repo with stderr:
   ```
   skip: <Path>: cannot derive group name from parent dir '<base>'
   ```
   Counts as a generic skip; does NOT block other repos.
6. If the leading char is not `[a-z]` → prefix `g` (e.g., `9-experiments` → `g9-experiments`).
7. Final defensive check against the schema regex `^[a-z][a-z0-9_-]*$`; non-conforming → treat as empty (bail out).

### Per-parent-dir granularity

One group per *distinct* parent dir (after canonicalization). Two different parent dirs are **never** merged even if their slugify outputs collide — see Conflict resolution. `buildScanPlan` tracks invented groups by `parentDir → index in plan.InventedGroups` so two repos under the same parent share a group.

### Group dir rendering

The `dir:` field of an invented group is the canonical parent path with `$HOME` substituted to `~/...` when the path begins under `$HOME`; otherwise the absolute path verbatim (`config_scan.go::homeSubstitute`).

### Conflict resolution

When the merge plan is built (`config_scan.go::resolveInventedName`):

1. **Slug matches existing group, dirs match (canonicalized)** → reuse that existing group; new URLs append to it. No stderr note.
2. **Slug matches existing group, dirs differ** → suffix with the smallest integer `-N` (starting at `-2`) that does not collide with any existing or already-invented group name. Stderr note:
   ```
   note: invented group '<original-slug>' already exists in hop.yaml with a different dir; using '<original-slug>-2' for <new-dir>.
   ```
3. **Two distinct parent dirs slugify to the same name during a single walk** → first one wins; second is suffixed `-2` (and so on). Same stderr note.

The smallest available suffix is found by linear scan starting at 2 (`nextAvailableSuffix`).

### Single-dir idempotency reporting

For the **single-dir, non-recursive, non-`-g`** case, `buildAddPlan` preserves the historical explicit "already registered" vs. "could not be registered" distinction (change `260605-c92v-fix-relative-dir-args`):

- `urlAlreadyRegistered(cfg, found[0].URL)` — an exact URL match across `cfg.Groups[*].URLs`, mirroring `buildScanPlan`'s `existingURLs` dedup — is checked **before** building the plan. A genuine duplicate prints `<cmdName>: <url> already registered in <path>. Nothing to add.` (exit 0, empty plan).
- If the URL is *not* a duplicate but the built plan is still empty (the sole candidate was skipped for a non-dedup reason, e.g. a slugify failure where `buildScanPlan` already emitted a `skip:` line), it prints the distinct fallback `<cmdName>: '<dir>' could not be registered (see skip above). Nothing to add.` — fixing the prior bug where any skip-to-empty-plan was misreported as "already registered."

Recursive and forced-group breadths skip this per-repo distinction (they use the scan-style aggregate summary instead).

## `-g` forced group (auto-create)

`buildForcedGroupPlan(cfg, found, opts, ...)` assigns **every** discovered repo to `opts.group`, bypassing the convention/invented logic entirely:

- **Dedup**: URLs already present anywhere in `hop.yaml` are dropped (and counted via `summary.skipAlreadyRegistered`, with a per-repo `skip: <path>: <url> already registered in hop.yaml` line) so the plan and summary agree with what `MergeScan` would actually change.
- **Auto-create**: in **write mode**, if the group does not already exist, `yamled.EnsureGroup(configPath, opts.group)` idempotently adds `<name>: []` under `repos`, then announces `created group: <name>` to stderr (colon form, matching the neighboring `created:` / `added:` / `wrote:` voice). The `created group:` line fires **only** when the group was genuinely new (gated on `groupExists`, computed before the ensure). In **print mode** the group is forced into the rendered plan **without** writing or ensuring anything (print never mutates the file).
- **Plan shape**: a forced group renders as a flat-list group `<name>: ['url', ...]` via `yamled.InventedGroup{Name, Flat: true, URLs}` (see [the `Flat` field](#the-flat-field-byte-identical-print-vs-write)).
- **Summary**: the stderr block reports `forced group: <name> (<n> new)` (labeled `forced group:`, NOT `invented groups:`, because `-g` names the group explicitly — it was forced, not invented).

This diverges deliberately from `hop clone`'s `--group` (which errors on a typo'd group against a pre-existing config): registration-from-disk is a bulk/onboarding operation where forcing a group into existence is the expected intent, and it is consistent with `add` already auto-initing the config. Both reuse the same `yamled.EnsureGroup` helper (Constitution IV).

## Output rendering

Both sinks share the in-memory render produced by `internal/yamled` — the only difference is where the bytes go. Print mode (`-p`) emits to `cmd.OutOrStdout()` via `yamled.RenderScan`; write mode performs `yamled.MergeScan` (atomic temp+rename, file mode preserved) only when the plan is non-empty.

### Print mode header

Print mode prepends a two-line header comment before the rendered YAML (`config_add.go::addPrintHeader`):

```
# hop config — generated by 'hop add -r -p <user-arg>' on <YYYY-MM-DD> (UTC).
# Run without --print to merge into <resolved-hop.yaml-path>.
```

`<user-arg>` is the user-supplied directory verbatim (not canonicalized); `<YYYY-MM-DD>` is `time.Now().UTC().Format("2006-01-02")` (UTC for reproducibility across collaborators); the literal `(UTC)` suffix removes timezone ambiguity. The header is part of the *stdout* render only — write mode does not modify the file's existing head comments. (Reworded from the deleted scan's `'hop config scan <arg>'` / `Run with --write to merge into` phrasing.) The header literal always names the recursive form (`'hop add -r -p <user-arg>'`) even for a single-dir `-p` dry-run — the `-r` in the header is the canonical onboarding example, not a reflection of the actual flags passed.

### The `Flat` field (byte-identical print vs. write)

`yamled.InventedGroup` gained a `Flat bool` field (change `260608-w2bj-unify-recursive-add`):

```go
type InventedGroup struct {
    Name string
    Dir  string
    Flat bool   // when true, render `<Name>: [...]` (flat list, no dir); Dir ignored
    URLs []string
}
```

- **`Flat: false`** (default, the convention/invented path via `buildScanPlan`) → `mergeScanIntoTree` renders a **map-shaped** group `{ dir: <Dir>, urls: [...] }`. This preserves the existing behavior for invented groups, which carry a derived `dir:` override.
- **`Flat: true`** (the `-g` forced-group path via `buildForcedGroupPlan`) → renders a **flat-list** group `<Name>: ['url', ...]` with no `dir` key, using `yaml.FlowStyle` on the synthesized sequence node.

Why `Flat` exists: a forced group has no `dir` override. Without the flag, print mode rendered an invalid map-shape `dir: ""` node (a rework finding — `M1`: print-mode `-g` rendered invalid map-shape `dir:""` instead of a flat list, which fails to reload). With `Flat`, print mode (`RenderScan`, where the group does not yet exist on disk) and write mode (`MergeScan` after `EnsureGroup` first seeds an empty `<name>: []`, which reparses as a flow sequence) both produce the identical flat `<name>: ['url', ...]` shape — keeping the `-p` dry-run byte-identical to the write (requirement R2).

### Group ordering

In the rendered YAML (both sinks):

1. **Existing groups** from the loaded `hop.yaml`, in their original source order.
2. **`default`** (if not already present in #1, AND `add` is contributing entries to it; if `default` already exists in #1 it stays in its source-ordered slot).
3. **Invented / forced groups** (those not present in #1), in the order given by `plan.InventedGroups` (the convention path pre-sorts alphabetically; a single forced group is one element).

Existing groups retain their existing URLs; contributed URLs are appended within each group at the end of the URL list (or `urls:` sequence for map-shaped groups).

### Stdout / stderr split

- **stdout** in print mode: rendered YAML (header comment + body). In write mode: empty.
- **stderr** in both sinks: the human-readable summary block (`emitAddSummary`). In write mode it ends with `wrote: <resolved-hop.yaml-path>`; in print mode it ends with `Run without --print to merge into <resolved-hop.yaml-path>.`.
- Per-repo skip lines (slugify failure, dedup) and conflict-resolution `note:` lines also go to stderr.

This matches `hop clone`'s precedent: status to stderr, useful piping payload to stdout. Print mode is pipeable: `hop add -r -p ~/code > hop.yaml` captures only the rendered YAML.

### Stderr summary block (`emitAddSummary`)

The single-dir, non-recursive, non-print, non-`-g` path emits its own inline lines (preserving historical wording): a real add → `added: <url>` + `wrote: <path>`; a no-op → the already-registered / could-not-register / not-a-git-repo line from discovery or plan building. **All other breadths/sinks** emit the scan-style aggregate block:

```
hop add: scanned <user-arg>, found <K> repos.
  matched convention (default): <C> (<C-new> new, <C-existing> already registered)
  forced group: <name> (<n> new)                       # -g only
  invented groups: <I> (<comma-separated names>)        # non-(-g) invented path
  skipped: <S1> worktree, <S2> bare repo, <S3> no remote[, <S4> no group name][, <S5> already registered]
[write only:  wrote: <resolved-hop.yaml-path>]
[print only:  Run without --print to merge into <resolved-hop.yaml-path>.]
```

The `forced group:` and `invented groups:` lines are mutually exclusive (`-g` produces the former, the convention path the latter). The `already registered` skip bucket counts non-convention URLs already present somewhere in `hop.yaml`; `yamled.MergeScan` would silently dedup these on write, so the CLI dedups them up-front so the plan, summary, and printed `skip:` lines all agree. Zero-count buckets are elided. Pluralization is per-reason (`1 worktree` vs `2 worktrees`). Zero-repos case is short-circuited to `hop add: scanned <user-arg>, found 0 repos. Nothing to add.` followed by the sink-appropriate `wrote:` / `Run without --print` trailer.

## Write / merge semantics

Implemented in `internal/yamled.MergeScan` (with `RenderScan` as the shared rendering primitive used by print mode):

- **Dedup**: any URL in `plan.DefaultURLs` or `plan.InventedGroups[i].URLs` already present in any existing group is **silently dropped** (matches the parser's URL-uniqueness rule and `AppendURL`'s contract). The CLI is responsible for any user-visible skip lines.
- **Default group**: if absent in the loaded file, created as a new flat-list group appended after existing groups.
- **Invented groups**: appended after existing groups in the order given by `plan.InventedGroups`. Map-shaped (`Flat: false`): `{ dir: <Dir>, urls: [<URLs>] }`. Flat-list (`Flat: true`, forced `-g` groups): `<Name>: [<URLs>]`.
- **`code_root`** and existing groups' `dir:`s are never modified.
- **Atomicity**: temp file + rename in the same directory; file mode preserved (defaults to 0644 for new files, matching `WriteStarter`). On rename failure the original is left untouched.
- **Comments**: preserved by yaml.v3 round-trip; indentation is normalized to yaml.v3 defaults — comment preservation is the contract, byte-perfect formatting is not.

Public surface in `internal/yamled/yamled.go`:

```go
func MergeScan(path string, plan ScanPlan) error
func RenderScan(path string, plan ScanPlan) ([]byte, error)

type ScanPlan struct {
    DefaultURLs    []string
    InventedGroups []InventedGroup
}

type InventedGroup struct {
    Name string  // already slugified by the caller; conforms to ^[a-z][a-z0-9_-]*$
    Dir  string  // already canonical (with ~-substitution); ignored when Flat
    Flat bool    // flat-list render `<Name>: [...]` for forced -g groups
    URLs []string
}
```

`MergeScan` writes; `RenderScan` returns the bytes that `MergeScan` would write. Both share `mergeScanIntoTree` internally.

## `hop config scan` is deleted — NO alias

The standalone `hop config scan` cobra subcommand was **removed entirely** in change `260608-w2bj-unify-recursive-add` — `newConfigScanCmd()`, its `runConfigScan` entry point, the `scanLong` help string, and the `scanHeaderComment` / `emitScanSummary` helpers (scan-exclusive) are all gone. The walk capability now lives as `hop add -r`.

**No alias** — `hop config scan <anything>` returns cobra's `unknown command "scan" for "hop config"` and a non-zero exit. This is a deliberate hard break (user decision, made after being shown the back-compat tradeoff twice — contrast with `add`/`rm`, which DO survive as hidden `config` aliases). It is implemented via a `RunE` on the `config` parent command (`config.go::newConfigCmd`, `Args: cobra.ArbitraryArgs`): a bare `hop config` (no args) prints help; an unknown subcommand reaches the parent's RunE with non-empty args and returns `fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())`. Valid subcommands (`init`/`where`/`print` and the hidden `add`/`rm` aliases) dispatch to their own RunE before this fires. See [cli/subcommands § Migration: hidden config aliases](../cli/subcommands.md#migration-hidden-config-aliases).

The **shared helpers** in `config_scan.go` that `add` depends on are retained: `validateConfigDir`, `buildScanPlan`, `slugifyGroupName`, `matchesConvention`, `resolveInventedName`, `homeSubstitute`, `buildSkipParts`, `pluralize`, and the `scanPlanSummary` struct (which gained a `forcedGroup` field for `-g`). The new `addPrintHeader` / `emitAddSummary` (the write-default trailer spellings) live in `config_add.go`, replacing scan's deleted `scanHeaderComment` / `emitScanSummary`. `internal/scan` is **UNTOUCHED**.

Migration:

| Old (deleted) | New |
|---|---|
| `hop config scan <dir>` (old print default) | `hop add -r -p <dir>` |
| `hop config scan <dir> --write` | `hop add -r <dir>` |
| `hop config scan <dir>` (recursive write intent) | `hop add -r <dir>` |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success (any number of repos found, including zero) AND every forgiving single-dir no-op (non-git dir, worktree/bare/no-remote skip, already-registered URL) |
| 1 | `hop.yaml` missing **in print mode only** (write mode auto-inits — see [precondition](#hopyaml-precondition)); YAML write/merge/render failure; load error on existing `hop.yaml`; `git` not on PATH (lazy); `$HOME` unset; `EnsureGroup` write failure (`-g`) |
| 2 | Usage error (no `<dir>` arg, dir validation failure, `--depth < 1`) |

## Tool requirements

- `git` — required only when discovery actually finds a `.git` candidate (lazy). Missing → `hop: git is not installed.` exit 1.

No other external tools are required.

## Cross-references

- Bootstrap-then-populate workflow, `hop config init`'s post-write tip wording (now `hop add -r <dir>`), and **auto-init-on-write** (`EnsureSkeleton`, the `created:` announcement, the read-vs-write split, and clone's `yamled.EnsureGroup` step): [init-bootstrap](init-bootstrap.md)
- YAML schema and group regex `^[a-z][a-z0-9_-]*$` that slugify must conform to: [yaml-schema](yaml-schema.md)
- `internal/scan` package role and `Walk`/`ClassifyOne`/`Found`/`Skip`/`Options` public surface: [architecture/package-layout](../architecture/package-layout.md)
- Constitution I compliance: `internal/scan` invokes `git` only via `internal/proc.RunCapture`: [architecture/wrapper-boundaries](../architecture/wrapper-boundaries.md)
- `hop add <dir>` full inventory row (flags, exit-code matrix, hidden `hop config add` alias) and `hop rm [<name>]`: [cli/subcommands](../cli/subcommands.md)
