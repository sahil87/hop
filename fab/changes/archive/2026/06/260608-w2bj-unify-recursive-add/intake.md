# Intake: Unify Recursive Repo Registration onto `hop add`

**Change**: 260608-w2bj-unify-recursive-add
**Created**: 2026-06-08
**Status**: Draft

## Origin

Initiated from a `/fab-discuss` session exploring onboarding friction. The user observed
that the current way to start using hop is `hop config scan`, and asked whether the
recursive option should move onto `add` (originally proposed as `hop add -R <foldername>`),
whether a `-g <groupname>` option made sense, and "anything else we can do to make
onboarding easier?" The user also asked whether `hop add` is an alias of `hop config add`.

> should we move this option to add instead? via hop add -R <foldername>? Or even a -g
> option for groupname. Thoughts? Anything else we can do to make onboarding easier?
> (Also is hop add an alias of some hop config add subcommand?)

**Interaction mode**: conversational, multi-turn. Key facts surfaced during discussion:

- **`hop add` is canonical, `hop config add` is the hidden alias** — not the other way
  around (established by changes `gyo0` + `260602-n1me`). Same for `hop rm` / `hop config rm`.
- **`add` and `config scan` are already a deliberate granularity pair** sharing all
  machinery (`validateConfigDir`, `buildScanPlan`, `yamled.MergeScan`): `add` calls
  `scan.ClassifyOne` (single dir), `scan` calls `scan.Walk` (DFS tree). `add` writes by
  default; `scan` prints by default, `--write` to merge.
- **`-R` was a *removed* flag** (the old "run in repo" tool-form precursor, deleted in
  `gyo0`). Reusing the capital `-R` for "recursive" would collide with that history — so
  the recursive flag is **`-r`/`--recursive`** (conventional Unix spelling).
- **Group assignment is fully automatic today** (convention → `default`, else
  invented slugified-parent-dir group). There is no way to force a group. Only
  `hop clone <url>` has a `--group` flag. This is a genuine functional gap.
- **Print-by-default is scan's biggest papercut** — a new user runs `hop config scan ~/code`,
  sees YAML scroll past, and is unsure anything happened. Folding recursion onto `add`
  (write-by-default) with an explicit `-p/--print` opt-in inverts this to the safer default.

**Decisions reached** (see Assumptions table for SRAD grades):

1. One **combined** change (not split into two), typed `refactor`.
2. Add `-r/--recursive`, `-p/--print`, `--depth N`, `-g/--group <name>` to `hop add`.
3. `-g`/`--group` **auto-creates** the group if missing (consistent with `add` auto-initing
   the config; mirrors `clone`'s fresh-config `EnsureGroup`).
4. **Delete `hop config scan` entirely — NO alias** (hard break; `unknown command` for any
   caller). The user explicitly rejected both a hidden alias and a deprecation notice after
   being shown the back-compat tradeoff twice.
5. Onboarding collapses to a single command: **`hop add -r <dir>`**.

## Why

**Problem.** Onboarding requires discovering and chaining two commands with a confusing
default. The documented path is `hop config init` → `hop config scan <dir>` → realize
nothing was written → `hop config scan <dir> --write`. The recursive populate tool
(`config scan`) is buried under the `config` namespace, prints by default (so a first run
appears to do nothing), and requires an additional `--write` flag to actually populate.
Meanwhile group assignment is entirely automatic with no override, so a user who wants their
repos in a named group has no way to express that at registration time.

**Consequence if unaddressed.** New users hit a multi-step, low-discoverability flow with a
silent-by-default surprise. The `add` (single) / `scan` (recursive) split forces users to
learn two commands and two opposite default behaviors (write vs. print) for what is
conceptually one operation: "register repos from disk into my config."

**Why this approach over alternatives.**
- *Unify onto `add` with breadth/sink as flags* (chosen) — one verb, one mental model:
  `hop add <dir>` (single, write), `hop add -r <dir>` (tree, write), `hop add -r -p <dir>`
  (tree, print). Write is the default at every breadth; `-p` is the explicit dry-run.
  This is **more** aligned with Constitution VI (Minimal Surface Area) — it *removes* a
  top-level concept rather than adding one.
- *Keep `add`/`scan` split, just improve scan discoverability* (rejected) — leaves the
  print-by-default papercut and the two-command learning curve intact.
- *Reintroduce `-R` for recursive* (rejected) — collides with the removed `-R` flag's prior
  meaning; confusing in git/grep history.
- *Hidden alias or deprecation notice for `config scan`* (rejected by user) — user wants the
  cleanest possible tree and is willing to accept the hard break.

**Constitutional alignment.**
- **VI (Minimal Surface Area)**: net reduction — one subcommand deleted, capability folded
  into flags on an existing command.
- **III (Convention Over Configuration)**: `-g/--group` is an *override* for when convention
  guesses wrong, not a new requirement. Default group behavior (auto-derive) is unchanged.
- **I (Security First)**: no new subprocess surface; all `git` calls still route through
  `internal/proc`. No change to the injection-safe argument-slice contract.

## What Changes

### 1. New flags on `hop add` (`src/cmd/hop/config_add.go`)

`hop add` gains four flags. The existing single-dir, write-by-default behavior is the
zero-flag baseline and is unchanged.

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `-r`, `--recursive` | bool | `false` | DFS-walk `<dir>` for git repos (the behavior currently in `config scan`) instead of classifying just `<dir>`. |
| `-p`, `--print` | bool | `false` | Render the merge plan to stdout instead of writing to `hop.yaml`. Valid at both breadths. |
| `--depth N` | int | `3` | Max DFS depth (only meaningful with `-r`; `N < 1` → exit 2). Carried over verbatim from `config scan`. |
| `-g`, `--group <name>` | string | `""` | Force all discovered repos into the named group, **auto-creating it** if absent. Valid at both breadths. |

Resulting grammar:

```
hop add <dir>                    # single dir, write          (unchanged baseline)
hop add -r <dir>                 # whole tree, write          (was: config scan <dir> --write)
hop add -r -p <dir>              # whole tree, print only     (was: config scan <dir>)
hop add -p <dir>                 # single dir, print only     (NEW: single-dir dry-run)
hop add -r --depth 2 <dir>       # bounded recursion
hop add -g vendor <dir>          # force group (any breadth)
hop add -r -g work ~/clients     # tree into a forced group
```

### 2. Recursive mode (`-r`) — fold in `scan.Walk`

When `-r` is set, `runAdd` calls `scan.Walk` (the existing depth-bounded, symlink-following,
(dev,inode)-dedup DFS in `src/internal/scan/scan.go`) instead of `scan.ClassifyOne`. All
existing scan semantics carry over unchanged:

- Repo classification (worktree / normal-repo / bare-repo / plain-dir, first-match-wins).
- Skip reporting (worktree, bare repo, no remote, no group name, already registered).
- Lazy `git` check, 5s per-invocation timeout, `internal/proc.RunCapture` routing.
- Group assignment via `buildScanPlan` (convention → `default`; else invented slugified group).
- `--write` semantics via `yamled.MergeScan` (atomic, comment-preserving).

The `Found` slice (one element from `ClassifyOne`, or many from `Walk`) feeds the **same**
`buildScanPlan` + write path regardless of breadth — this is already how `add` and `scan`
share machinery, so the fold is mechanical.

### 3. Print mode (`-p`) — sink selection

Print mode renders the in-memory plan to stdout (the bytes `--write` would produce) instead
of merging into `hop.yaml`. This is exactly today's `config scan` print-vs-write split,
relocated to the `-p` flag with the sink **inverted to write-by-default**:

- **No `-p`** (default): write to `hop.yaml` via `yamled.MergeScan` (auto-init skeleton if
  absent, announcing `created: <path>`). Status summary to stderr ending in `wrote: <path>`.
- **`-p`**: render to `cmd.OutOrStdout()`. No file touched, no auto-init. Status summary to
  stderr ending in `Run without --print to merge into <resolved-hop.yaml-path>.` (reworded
  from scan's `Run with --write to merge into ...`).

**Print-mode header string** (currently
`# hop config — generated by 'hop config scan <user-arg>' on <date> (UTC).` /
`# Run with --write to merge into <path>.`) is updated to reference the new spelling:

```
# hop config — generated by 'hop add -r -p <user-arg>' on <YYYY-MM-DD> (UTC).
# Run without --print to merge into <resolved-hop.yaml-path>.
```

For the **single-dir** print case (`hop add -p <dir>` without `-r`), the same header/render
applies to the one-repo plan — a genuinely new dry-run capability hop doesn't have today.

### 4. `-g/--group <name>` — forced group with auto-create

When `-g <name>` is supplied, every discovered repo is assigned to `<name>` instead of
running through the convention/invented-group logic in `buildScanPlan`. Behavior:

- If `<name>` does not exist in `hop.yaml`, **auto-create it** as an empty group `<name>: []`
  before assignment — via `yamled.EnsureGroup` (the same idempotent helper `hop clone` uses
  on a fresh config). Announce `created group: <name>` to stderr (new status line, colon
  form, matching the neighboring `created:` / `added:` / `wrote:` voice).
- If `<name>` already exists, append to it (no announcement).
- The flag forces the group for *all* discovered repos (single or recursive), overriding both
  the convention-match-to-`default` path and the invented-group path.
- Auto-create chosen over error-on-unknown (user decision) — consistent with `add` already
  auto-initing the config; the most forgiving behavior. (`clone` errors on a typo'd `--group`
  against a *pre-existing* config; `add -g` deliberately diverges toward auto-create because
  registration-from-disk is a bulk/onboarding operation where forcing a group into existence
  is the expected intent.)

### 5. Delete `hop config scan` — NO alias

- Remove the `config scan` cobra subcommand entirely: delete `newConfigScanCmd()` and its
  wiring in `config.go::newConfigCmd().AddCommand(...)`. (`src/cmd/hop/config_scan.go` —
  the *shared helpers* it hosts, `validateConfigDir` / `buildScanPlan` / `slugifyGroupName` /
  the plan-resolution helpers, are still used by `add` and MUST remain; only the cobra
  command factory and its `runConfigScan` entry point that are exclusive to the deleted
  command are removed. The walk itself lives in `internal/scan` and is untouched.)
- **No alias** — `hop config scan <anything>` returns cobra's `unknown command` error. This
  is a deliberate hard break (user decision, made after being shown the back-compat tradeoff).
- `help-dump` JSON (`help_dump.go` output): `config scan` drops off automatically once the
  cobra node is gone; `add` gains its new flags in the dumped tree. The published
  `help/hop.json` contract (consumed by shll.ai's scheduled pull) changes accordingly.
- `newConfigCmd()`'s `Short` summary (`"config helpers (init, where, scan, print)"`) drops
  `scan` → `"config helpers (init, where, print)"`.

### 6. Fix dangling references to the deleted `config scan`

Every internal pointer to `hop config scan` becomes a dangling reference once the command is
gone. Known sites (a full sweep happens at apply time):

- **`hop config init`'s post-write tip** (`src/cmd/hop/config.go`): currently
  `Edit the file to add your repos, or run `hop config scan <dir>` to populate from existing
  on-disk repos.` → repoint to `hop add -r <dir>`.
- The `Resolve()` not-found message already points at `hop add <dir>` (not `config scan`), so
  that one is fine — but verify during the sweep.
- Any other tip/error strings, help text, or comments referencing `config scan` (grep
  `config scan` and `configScan`/`ConfigScan` across `src/`).

### 7. README onboarding cleanup (`README.md`)

Collapse onboarding to a single `hop add -r <dir>` command. Specific stale references:

- **`README.md:15`** — the "Bootstrap from disk, not yaml-by-hand" feature bullet currently
  reads `hop config scan ~/code walks your existing clones ...`. Update to `hop add -r ~/code`.
- **`README.md:100-101`** — the onboarding block (`hop config init` / `hop config where`).
  Rework so the primary getting-started step is `hop add -r <dir>`; drop the `config init`
  prerequisite framing (auto-init means `add -r` works on a fresh machine with no prior init).
  `hop config where` may stay as a debug-aid mention if still relevant.
- **`README.md:109-110`** — the `hop config scan ~/code` preview / `--write` example block.
  Replace with `hop add -r ~/code` (write) and `hop add -r -p ~/code` (preview) examples.

## Affected Memory

- `config/scan` → **renamed to `config/add-register`**: (modify+rename) The whole document
  describes `hop config scan`. It must be rewritten around `hop add -r` — recursion is now a
  flag on `add`, print/write inverts to `-p`-opt-in, `config scan` is deleted with no alias.
  The "`hop add`: the single-dir sibling" section folds into the main narrative (they're now
  one command at two breadths). The file is **renamed** `config/scan.md` →
  `config/add-register.md` (user decision) so the filename reflects the new topic
  (registering repos via `hop add`, not "scanning"). Update `docs/memory/index.md`'s `config`
  domain row to point at the renamed file.
- `cli/subcommands`: (modify) Rewrite the `hop add <dir>` inventory row to cover the new
  `-r`/`-p`/`--depth`/`-g` flags; **remove** the `hop config scan <dir>` row entirely; update
  the "Migration: hidden config aliases" section (scan is *deleted*, not aliased, unlike
  add/rm); update the `hop config init` row's tip wording; reflect the `config` parent
  `Short` summary change.
- `config/init-bootstrap`: (modify) The `config init` post-write tip and the auto-init-on-write
  table reference `config scan --write`; update to `hop add -r`. The "which write-commands
  auto-init" set is unchanged in substance (recursive add still auto-inits on write) but the
  command *spelling* changes.
- `architecture/package-layout`: (modify, if it documents the `config scan` command surface)
  — verify `internal/scan`'s `Walk`/`ClassifyOne`/`Found` exported-seam description still holds
  (it does — the package is untouched), but any reference to the `config scan` *command* as the
  `Walk` caller updates to `hop add -r`.

## Impact

- **Code**: `src/cmd/hop/config_add.go` (new flags, recursive + print branching, `-g` forced
  group), `src/cmd/hop/config_scan.go` (delete the cobra command + `runConfigScan`; keep shared
  helpers), `src/cmd/hop/config.go` (drop `scan` wiring + `Short` summary; fix `init` tip),
  `src/cmd/hop/help_dump.go` (no code change — tree updates automatically), `src/internal/scan/`
  (untouched), `src/internal/yamled/` (untouched — `EnsureGroup` already exists for `-g`).
- **Tests**: every `config scan` test (`config_scan_test.go` and any integration tests invoking
  `hop config scan`) must be migrated to `hop add -r` / `hop add -r -p` / `hop add -r -g`, or
  deleted if redundant with new `add` tests. New tests for `-g` auto-create and single-dir `-p`
  dry-run. Per Constitution "Test Integrity": tests conform to the (new) spec, not vice versa.
- **Docs**: `README.md` (3 sites), `docs/memory/config/scan.md`, `docs/memory/cli/subcommands.md`,
  `docs/memory/config/init-bootstrap.md`.
- **Published contract**: `help/hop.json` (shll.ai scheduled pull) — `config scan` vanishes,
  `add` gains flags. Downstream-visible; acceptable per user decision.
- **Breaking change**: any existing script or muscle-memory invocation of `hop config scan`
  hard-breaks with `unknown command`. This is intentional and accepted.

## Open Questions

- Exact reworded README onboarding prose — needs the surrounding README structure read at
  apply time to match voice/format. *(Not a blocking decision — voice-matching is a routine
  apply-time concern.)*

*(Both prior open questions resolved during `/fab-clarify`-style conversation: (1) `hop add -p
<dir>` single-dir dry-run is **supported** — `-p` does not require `-r`; (2) `config/scan.md`
is **renamed** to `config/add-register.md`. See Assumptions #11 and #12.)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | One combined `refactor` change (not split into A/B). | Discussed — user explicitly chose "One combined change" over my two-change recommendation. | S:98 R:70 A:90 D:95 |
| 2 | Certain | Delete `hop config scan` with NO alias (hard break). | Discussed — user rejected hidden alias and deprecation notice; confirmed "remove it" after being shown the tradeoff twice. | S:98 R:30 A:85 D:95 |
| 3 | Certain | Recursive flag is `-r`/`--recursive`, not `-R`. | Discussed — `-R` was a removed flag with different prior meaning; user agreed via the AskUserQuestion preview. Conventional Unix spelling. | S:95 R:75 A:95 D:90 |
| 4 | Certain | `-g`/`--group` auto-creates the group if missing. | Discussed — user explicitly selected "Auto-create the group" over "Error on unknown group". | S:98 R:65 A:90 D:95 |
| 5 | Certain | `-p`/`--print` opt-in; write is the default at every breadth. | Discussed — user proposed `hop add -r -p/--print` for print; inverts scan's print-by-default papercut. | S:95 R:70 A:90 D:90 |
| 6 | Confident | Add `-r`, `-p`, `--depth`, `-g` all to `hop add`. | Discussed — the four flags are the agreed surface; `--depth` carries over from scan unchanged. | S:90 R:65 A:90 D:85 |
| 7 | Confident | `config scan`'s shared helpers (`validateConfigDir`, `buildScanPlan`, `slugifyGroupName`) stay; only the cobra command + `runConfigScan` are deleted. | Codebase: `add` already depends on these helpers (per scan.md); deleting them would break `add`. Mechanical. | S:85 R:60 A:92 D:88 |
| 8 | Confident | README onboarding collapses to `hop add -r <dir>`; fix refs at lines 15, 100-101, 109-110. | Discussed — user named the exact goal and I grepped the exact line numbers. | S:92 R:80 A:88 D:85 |
| 9 | Confident | `config init` post-write tip repoints from `config scan` to `hop add -r`. | Discussed — flagged as a dangling reference; user agreed to the sweep. Codebase-confirmed the tip exists. | S:88 R:75 A:90 D:85 |
| 10 | Confident | Reuse `yamled.EnsureGroup` for `-g` auto-create (don't write a new helper). | Codebase: `clone` already uses `EnsureGroup` for the identical "create group on fresh config" need; reuse over reinvent (Constitution IV). | S:80 R:60 A:88 D:82 |
| 11 | Certain | `hop add -p <dir>` (single-dir, no `-r`) is a supported dry-run; `-p` does not require `-r`. | Clarified — user confirmed. `-p` means "render plan, don't write" at any breadth; no `-p`-requires-`-r` carve-out. | S:95 R:70 A:90 D:90 |
| 12 | Certain | `config/scan.md` memory file is **renamed** to `config/add-register.md` (content rewritten around `hop add -r`). | Clarified — user chose rename over keep-name/defer. Filename reflects the new topic; recorded as a planned hydrate task. | S:95 R:80 A:90 D:90 |

12 assumptions (7 certain, 5 confident, 0 tentative, 0 unresolved).
