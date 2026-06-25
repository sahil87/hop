---
description: "`src/cmd/hop/` + `src/internal/<pkg>/`, cobra wiring, pre-cobra `--shim-plan` classifier (`shim_plan.go`), reoriented batch verbs (`batch_verb.go`), `help_dump.go` cobra-tree producer, conventions, gyo0 line-reduction"
type: memory
---
# Package Layout

How the Go source tree is organized for the `hop` binary. Module path is `github.com/sahil87/hop`. The module is rooted at `src/go.mod`, not the repo root.

## Tree

```
src/
├── go.mod                        # module github.com/sahil87/hop, go 1.22; deps cobra, yaml.v3, golang.org/x/term
├── go.sum
├── cmd/hop/                      # one cobra entrypoint (renamed from cmd/repo/)
│   ├── main.go                   # entrypoint + translateExit (incl. errNoTTY→3) + extractShimPlan (pre-cobra --shim-plan dispatch)
│   ├── root.go                   # newRootCmd, rootLong help text, AddCommand wiring, runRoot (selection-first grammar) + runPluralSelection
│   ├── shim_plan.go              # hidden --shim-plan classifier — runShimPlan/classifySingular/classifyPlural/emitCD/emitRunInParent, protocol keyword consts, batchVerbs set, isKnownSubcommand/isConfiguredGroupName
│   ├── batch_verb.go             # runBatchVerb — selection-first entry for the pull/push/sync action tokens; resolveTargets → existing pull/push/sync runners
│   ├── resolve.go                # shared loadRepos/resolveOne/resolveByName/buildPickerLines + resolveTargets/resolveMode/hasConfiguredGroup (name-or-group resolver for the batch verbs)
│   ├── repo_completion.go        # completeRepoNames, completeWorktreeCandidates, completeCloneArg (cobra ValidArgsFunctions)
│   ├── clone.go, ls.go
│   ├── pull.go                   # pullSingle/pullBatch/pullOne + lastNonEmptyLine (runner logic; cobra factory removed in gyo0)
│   ├── push.go                   # pushSingle/pushBatch/pushOne (runner logic; cobra factory removed in gyo0)
│   ├── sync.go                   # syncSingle/syncBatch/syncOne + mentionsConflict + defaultSyncCommitMessage (runner logic; cobra factory removed in gyo0)
│   ├── open.go                   # runOpen — execs `wt open <path>` (PASSTHROUGH passthrough; shim owns the cd-handoff)
│   ├── shell_init.go             # posixInit (shared zsh+bash protocol-interpreter shim, hard-codes zero subcommand names) — a SINGLE self-contained hop() (PASSTHROUGH body inlined; no _hop_passthrough sibling), exports HOP_WRAPPER=1; + cobra GenZshCompletion / GenBashCompletionV2 at runtime
│   ├── tty.go                    # isTTY seam (term.IsTerminal on stdin), errNoTTY sentinel, noTTYHint stderr line (change 1x1u — agent/non-interactive support)
│   ├── config.go                 # config parent + nested init/where/scan/print factories; also wires the HIDDEN config add/config rm aliases
│   ├── config_scan.go            # shared plan-building helpers for `hop add`: validateConfigDir, buildScanPlan, slugify, conflict resolution, scanPlanSummary (the `config scan` cobra command was deleted in w2bj; helpers retained)
│   ├── config_add.go             # canonical `hop add <dir>` (newAddCmd) + hidden alias `hop config add` (newConfigAddCmd), both backed by the shared runAdd(cmd, cmdName, arg, opts) — flags -r/-p/--depth/-g; single-dir (scan.ClassifyOne) or recursive (scan.Walk) discovery + buildScanPlan/buildForcedGroupPlan + MergeScan/RenderScan; addPrintHeader/emitAddSummary
│   ├── config_rm.go              # canonical `hop rm [<name>]` (newRmCmd) + hidden alias `hop config rm` (newConfigRmCmd), both backed by the shared runRm(cmd, cmdName, stale, name) — fzf picker (pickOne seam) + path-column map-back, OR resolveByName for the positional path, then the shared removeRepo → yamled.RemoveURL
│   ├── help_dump.go              # hidden `hop help-dump` producer — Doc/Node structs, cobra-tree walk, buildHelpDoc(cmd.Root())
│   ├── *_test.go                 # adjacent unit tests (incl. shim_plan_test.go — classifier CD/RUN_IN_PARENT/PASSTHROUGH/plural-guard tests)
│   ├── integration_test.go       # builds the binary and exercises it end-to-end
│   └── testutil_test.go          # shared test helpers
└── internal/
    ├── config/                   # YAML schema, search order, embedded starter
    │   ├── config.go             # yaml.Node-based loader, group validation, URL uniqueness
    │   ├── resolve.go            # single fixed config path ($HOME/.config/hop/hop.yaml)
    │   ├── starter.yaml          # //go:embed (grouped form)
    │   ├── *_test.go
    │   └── testdata/             # valid + invalid fixtures (mixed shapes, bad names, dup URLs, etc.)
    ├── repos/                    # in-memory Repo model + match
    │   ├── repos.go              # FromConfig, MatchOne, ExpandDir, DeriveName, DeriveOrg
    │   └── repos_test.go
    ├── yamled/                   # comment-preserving YAML node-level edits
    │   ├── yamled.go             # AppendURL, RemoveURL, MergeScan, RenderScan, ScanPlan, InventedGroup, ErrGroupNotFound, ErrURLNotFound, atomic write
    │   └── yamled_test.go
    ├── scan/                     # DFS walk + repo classification for `hop add` (single-dir + recursive `-r`)
    │   ├── scan.go               # Walk, ClassifyOne, Found, Skip, Options, GitRunner; closed Reason enum; (dev,inode) loop dedup
    │   └── scan_test.go
    ├── fzf/                      # fzf wrapper
    │   ├── fzf.go
    │   └── fzf_test.go
    ├── proc/                     # centralized exec.CommandContext
    │   ├── proc.go               # Run, RunCapture, RunInteractive, RunForeground, ExitCode, ErrNotFound
    │   └── proc_test.go
    └── update/                   # self-update via Homebrew
        ├── update.go             # Run(version), brew detect/index/info/upgrade
        └── update_test.go
```

## Conventions

| Convention | Value |
|---|---|
| Module path | `github.com/sahil87/hop` |
| `go.mod` location | `src/go.mod` (not repo root — mirrors `fab-kit/src/go/wt`) |
| Go version | `1.22` |
| CLI framework | `github.com/spf13/cobra` v1.8.1 |
| YAML library | `gopkg.in/yaml.v3` |
| TTY detection | `golang.org/x/term` v0.27.0 (+ indirect `golang.org/x/sys` v0.28.0) — added in change `1x1u`; pins mirror the sibling `idea`. The `go` directive was deliberately kept at `1.22` (term v0.27.0 builds clean against it; no bump). |
| Tests | Adjacent to source (`config.go` + `config_test.go`) |
| Test fixtures | `testdata/` next to the tests that use them (per-package, not centralized) |
| `internal/<pkg>/` shape | Flat — no nested sub-packages |

## Cobra wiring

Each subcommand is exposed via a `func newXxxCmd() *cobra.Command` factory in its own file. `root.go::newRootCmd()` constructs the root and calls `AddCommand(newAddCmd(), newRmCmd(), newCloneCmd(), newLsCmd(), newShellInitCmd(), newConfigCmd(), newUpdateCmd(), newHelpDumpCmd())`. That is **seven user-facing top-level subcommands** (`add`, `rm`, `clone`, `ls`, `shell-init`, `config`, `update`) plus the hidden `help-dump`. **`pull`/`push`/`sync` are no longer subcommands** (change `gyo0`) — their `newPullCmd`/`newPushCmd`/`newSyncCmd` factories were removed; they are now action tokens dispatched by `runRoot` → `runBatchVerb` (see [batch_verb.go](#batch_verbgo--reoriented-batch-verbs) below). The registry-edit verbs `add` / `rm` are the canonical top-level commands; `config add` / `config rm` survive as hidden aliases sharing the same `runAdd` / `runRm` bodies (wired under `config` in `config.go`, not here) — see [cli/subcommands § Migration: hidden config aliases](/cli/subcommands.md#migration-hidden-config-aliases). `main.go::main()`:

1. Builds `rootCmd := newRootCmd()`.
2. Sets `rootCmd.Version = version` (the package-level `var version = "dev"`, overridden via `-ldflags "-X main.version=…"` at build time — see [build/local](/build/local.md)).
3. Sets `rootForCompletion = rootCmd` (a package-level var used by `shell-init` to call `GenZshCompletion` / `GenBashCompletionV2` without threading rootCmd through the factory).
4. Inspects `os.Args` for the hidden `--shim-plan` flag via `extractShimPlan`; if present, calls `os.Exit(runShimPlan(os.Stdout, os.Stderr, rest))` — classifying the user's argv and emitting the 3-keyword protocol **before cobra parses argv** (the action token after a selection, e.g. `git pull`, is an arbitrary child command line cobra must never parse). See [shim_plan.go](#shim_plango--the---shim-plan-classifier) below.
5. Otherwise calls `rootCmd.Execute()`. Errors are mapped to exit codes via `translateExit`.

`rootCmd` sets `SilenceUsage = true` and `SilenceErrors = true` so we control all stderr/exit emission via `translateExit`. The root uses `cobra.ArbitraryArgs` (no positional cap) and a `--all` bool flag; its `RunE` calls `runRoot(cmd, args, all)`, which implements the selection-first grammar (bare picker, plural selection, `where`/`cd`/`open` verbs, `pull`/`push`/`sync` action tokens, tool-form hint).

### Why pre-Execute argv inspection for `--shim-plan`

Cobra's flag parser would try to dispatch the action token after a selection (`hop <name> git status`) as a subcommand or its args, which fails for arbitrary child command lines. Handling `--shim-plan` before `rootCmd.Execute()` lets `runShimPlan` classify the full argv and resolve a path without cobra ever touching the action token. The detection is a single function (`extractShimPlan` in `shim_plan.go`), and the classifier itself is unit-tested in `shim_plan_test.go`. (This is the same rationale that previously justified the removed `-R` pre-Execute inspection — change `gyo0` replaced `extractDashR`/`runDashR` with `extractShimPlan`/`runShimPlan`.)

## `help_dump.go` — help-tree producer

New file (change `jr5f`). Houses the hidden `hop help-dump` subcommand and its pure-Go producer; `newHelpDumpCmd()` is wired into `newRootCmd()`'s `AddCommand(...)` alongside the other factories. The file follows the package's conventions: a `newXxxCmd()` factory, structured cobra accessors, adjacent `help_dump_test.go`.

The producer **walks the live cobra tree** rather than regex-parsing `-h` output: `buildHelpDoc(cmd.Root())` (called from `RunE` so it gets the wired root with `Version` set and all subcommands attached), then `buildNode` recurses via `cmd.Commands()`. `shouldSkipChild` prunes `completion`, `help`, any `Hidden` command (including `help-dump` itself), and `IsAdditionalHelpTopicCommand()` topics. It is **pure Go** — `encoding/json` + cobra only, **no subprocess** (no `internal/proc`, no `os/exec`) and no `time.Now()` — so it builds on all four targets and stays deterministic/testable. The emitted JSON contract, field semantics, and the shll.ai pull flow that consumes it are documented in [cli/subcommands § help-dump contract](/cli/subcommands.md#hop-help-dump--json-help-tree-contract) and [build/release-pipeline § help reference](/build/release-pipeline.md#help-reference-shllai).

Note the `--shim-plan` classifier (`shim_plan.go`, handled pre-cobra in `main.go`) and the tool-form / batch-verb action tokens live **outside** the cobra tree, so the walk never sees them as nodes — they are described only via the root node's `text` (which carries `rootLong`). This is intentional, consistent with why `--shim-plan` uses pre-Execute argv inspection rather than a cobra flag/subcommand (see above). With `pull`/`push`/`sync` reoriented to action tokens (change `gyo0`), the dump's top-level node list shrank to the seven user-facing subcommands.

## `shim_plan.go` — the `--shim-plan` classifier

New file (change `gyo0`). Houses the hidden `--shim-plan` dispatch core: `extractShimPlan` (pre-cobra detection of the flag in `os.Args`), `runShimPlan` (top-level classifier), `classifySingular` / `classifyPlural` (per-selection-shape logic), `emitCD` / `emitRunInParent` (resolve a path and print the plan), `shimResolveErr` (mirrors `translateExit`'s code policy), and the protocol vocabulary as named constants (`planCD` / `planRunInParent` / `planPassthrough` / `shimPlanFlag` / `allSelectionFlag`) plus the `batchVerbs` set. `isKnownSubcommand` walks the **live** `newRootCmd().Commands()` — the single source of truth — so the shim hard-codes zero subcommand names; `isConfiguredGroupName` reuses `hasConfiguredGroup` (resolve.go) for the singular-vs-plural decision. The classifier NEVER execs the user's action — it only resolves a path and emits the fixed vocabulary (Constitution I — no injection surface). See [cli/subcommands § Shim ↔ binary protocol](/cli/subcommands.md#shim--binary---shim-plan-protocol) for the full classification contract.

## `batch_verb.go` — reoriented batch verbs

New file (change `gyo0`). Houses `runBatchVerb(cmd, verb, selection, all)` — the selection-first entry point for the `pull`/`push`/`sync` action tokens. It resolves the selection via `resolveTargets` (`resolve.go`) and dispatches into the **existing** single/batch runners in `pull.go` / `push.go` / `sync.go` (`pullSingle`/`pullBatch`, `pushSingle`/`pushBatch`, `syncSingle`/`syncBatch`) — the runner logic, per-repo summary lines, and exit-code policy are unchanged from change `xj3k`; only the entry point moved from a cobra subcommand factory to this action-token dispatcher. Called from `root.go::runRoot` (singular `$2` ∈ {pull,push,sync}) and `root.go::runPluralSelection` (plural selection), and reached via the `--shim-plan` classifier's `PASSTHROUGH` plan. `sync` uses the fixed `defaultSyncCommitMessage` — the old `-m`/`--message` flag was dropped with the cobra factory.

## `internal/yamled`

New package introduced by this change. Owns node-level YAML edits — comment-preserving append into a group's URL list. See [wrapper-boundaries](/architecture/wrapper-boundaries.md) for why it's a separate package from `internal/config`.

API:

```go
func AppendURL(path, group, url string) error
func RemoveURL(path, group, url string) error
var ErrGroupNotFound = errors.New("yamled: group not found")
var ErrURLNotFound   = errors.New("yamled: url not found")
```

`AppendURL` reads the file as a `*yaml.Node` tree, navigates `repos.<group>`, appends a new scalar to either the sequence body (flat group) or the `urls:` child sequence (map-shaped group), then marshals and atomically writes back via temp file + rename. Comments are preserved by the yaml.v3 round-trip; **indentation is normalized to yaml.v3 defaults** (this is a deliberate design choice, not a guarantee — comment preservation is the contract, byte-perfect formatting is not).

`RemoveURL` is the mirror of `AppendURL` (added in change `260602-n1me-config-add-rm-folders`, consumed by the canonical `hop rm` and its hidden `hop config rm` alias, via the shared `removeRepo` helper): same tree round-trip + `atomicWrite`, but it locates the matching URL scalar (via the `urlsSequenceOf` / `indexOfScalar` helpers, handling both the flat-list and `urls:`-map shapes) and drops it from the group's URL sequence. Removing a group's last URL **leaves the empty group node in place** (the group key is never deleted), keeping it a valid `hop clone --group` target — see [config/yaml-schema](/config/yaml-schema.md#empty--placeholder-groups).

Errors are wrapped fmt.Errorf strings; missing-group is additionally wrapped via `%w` with `ErrGroupNotFound` so callers can detect via `errors.Is`. `RemoveURL` also wraps `ErrURLNotFound` when the group exists but the URL is absent (and reuses `ErrGroupNotFound` when the group itself is missing); both not-found cases are no-ops that leave the file byte-for-byte unchanged, so `hop rm` / `hop config rm` (via `removeRepo`) can surface them as a forgiving message + exit 0.

## `internal/scan`

Owns the directory walk and repo classification for `hop add` (single-dir via `ClassifyOne`, recursive via `Walk` under `-r`). UI-free: knows how to recognize git working trees (vs worktrees, submodules, bare repos, no-remote repos) and how to follow symlinks safely with `(dev, inode)` loop dedup; does NOT know about groups, slugify, conflict resolution, YAML rendering, or stderr UX (those live in the CLI / yamled layers). The package is **untouched** by the `config scan` → `hop add -r` unification (change `260608-w2bj-unify-recursive-add` was a CLI-layer dispatch change only).

API:

```go
func Walk(ctx context.Context, root string, opts Options) ([]Found, []Skip, error)
func ClassifyOne(ctx context.Context, dir string, opts Options) (found Found, skipReason string, isRepo bool, err error)

type Found struct { Path, URL string }              // canonical path + remote URL
type Skip struct { Path, Reason string }            // closed reason set
type Options struct { Depth int; GitRunner GitRunner }
type GitRunner func(ctx, dir, args ...string) ([]byte, error)

const (
    ReasonNoRemote  = "no remote"
    ReasonBareRepo  = "bare repo"
    ReasonWorktree  = "worktree"
    ReasonSubmodule = "submodule"  // reserved; never emitted by Walk (no-descent invariant suffices)
)
```

`Walk` performs a stack-based DFS, classifies each candidate via first-match-wins rules, and registers found repos by invoking `git remote` + `git remote get-url` through `Options.GitRunner` (production binds `internal/proc.RunCapture`). It is the discovery engine behind `hop add -r`. Tests inject a fake `GitRunner` so no real `git` subprocess spawns. Discovery order is DFS lexical (deterministic for reproducible test fixtures and slug-tie tiebreaking). See [config/add-register](/config/add-register.md) for the classification rules and the submodule-handling rationale.

`ClassifyOne` (added in change `260602-n1me-config-add-rm-folders`, consumed by `cmd/hop/config_add.go`'s shared `runAdd` body) is the **single-dir entry point** for `hop add` (and its hidden `hop config add` alias): it classifies one already-canonical directory (no recursion, `opts.Depth` ignored) and, for a normal repo, inspects its remote — reusing the *exact* unexported `classifyDir` + `inspectRepo` logic `Walk` applies per directory. It returns exactly one of: `isRepo=true` with `Found` populated (normal repo with a usable remote); `isRepo=false` with `skipReason` set to a `Reason*` constant (worktree / bare repo / no-remote); or `isRepo=false` with `skipReason==""` (a plain non-git dir). `err` is non-nil only on a fatal `git` failure (wraps `proc.ErrNotFound` for `errors.Is` matching). **Exported-seam design note**: `ClassifyOne` is the *smallest* exported surface that gives the CLI what it needs — exporting `classifyDir` raw was rejected because it would leak the private `dirClass` enum and skip the remote-inspection step (`inspectRepo`) the CLI requires. The CLI canonicalizes `dir` (via the shared `validateConfigDir`) before calling, and `ClassifyOne` stays UI-free like the rest of the package.

## Code size impact of the grammar + shim refactor (change `gyo0`)

The selection-first refactor was a **net reduction** despite adding two files (`shim_plan.go`, `batch_verb.go`). Measured `src/` delta: **−729 lines** (643 insertions, 1372 deletions). The deletions outweigh the additions because the refactor removed more than it added:

- `main.go`: 155 → 81 lines (`extractDashR` / `runDashR` and the `-R` argv-inspection block deleted).
- `shell_init.go`: 199 → 152 lines (the hard-coded subcommand `case` list, `_hop_dispatch`, the `-R` rewrite, and the `hi` alias deleted; replaced by the small 3-keyword protocol interpreter + `_hop_passthrough`).
- `pull.go` / `push.go` / `sync.go`: cobra factories (`newPullCmd`/`newPushCmd`/`newSyncCmd`) removed; runner logic retained.
- `dashr_test.go`: deleted; replaced by `shim_plan_test.go`.

The new files carry only the classifier (`shim_plan.go`) and the action-token dispatcher (`batch_verb.go`, which reuses existing runners rather than reimplementing them) — both small.

### Design Decision: subcommand list lives only in cobra (change `gyo0`)

The shim hard-codes **zero** subcommand names. The binary's `isKnownSubcommand` (`shim_plan.go`) walks the live `newRootCmd().Commands()`, so the subcommand list has exactly one source of truth (cobra registration). This is the structural fix for the "stale shim" bug class — an already-open interactive shell can no longer drift out of sync with the binary's subcommand set, because the shim branches only on the fixed 3-keyword protocol, never on subcommand names. Future changes that add/remove/promote subcommands need no shim change.

### Design Decision: `posixInit` is a single self-contained `hop()` — partial-capture immunity (change `1x1u`)

`posixInit` now emits **one** top-level shell function, `hop()`, with the PASSTHROUGH cd side-channel body (the `WT_CD_FILE` mktemp/`-s`/`cd`/`rm -f` dance) **inlined directly** into its `PASSTHROUGH)` case arm (`shell_init.go`). The previously-standalone `_hop_passthrough()` sibling function was removed.

**Why this is a fix, not a refactor.** This addresses a NEW facet of the "stale shim" bug class — not name-drift (fixed above), but **partial capture**. Claude Code's shell-snapshot mechanism captures shell functions per-function; it captured `hop()` but NOT its sibling `_hop_passthrough()`. Every PASSTHROUGH command (`where`, `ls`, `add`, `rm`, `clone`, `config`, `pull`/`push`/`sync`, `--help`, `--version`, …) then died with `_hop_passthrough: command not found` — even though the binary, config, and resolution logic were correct the whole time. A dispatcher that depends on a separately-defined top-level sibling is fragile to per-function snapshotting. Inlining is the only form **provably immune** to partial capture: there is no second definition object for a snapshotter to drop, and it matches the file's existing "logic-free interpreter" framing. (Nesting `_hop_passthrough` inside `hop()` was rejected — a nested function is still a separate definition object a snapshotter could in principle split.) The inlined body keeps the `WT_CD_FILE` semantics verbatim, so the unified cd side-channel (`open`/`clone <url>` handoff) is unchanged. The `__complete*` arm, `CD`/`RUN_IN_PARENT` arms, defensive `*)` fallback, and the `h()` alias are all unchanged. This is the second structural fix to the "stale shim" bug class (the first — shim/binary name-drift — was change `gyo0`'s zero-hard-coded-subcommands shim); see [cli/agent-non-interactive-usage](/cli/agent-non-interactive-usage.md) for how this ties into the broader agent contract.

### `tty.go` — TTY seam + no-TTY sentinel (change `1x1u`)

New file. Houses the package-level `isTTY` seam (swappable in tests via `withIsTTY` / a `TestMain` default of `true` in `testutil_test.go`, since the test binary runs with stdin redirected), the `errNoTTY` sentinel, and the `noTTYHint` stderr constant. The seam mirrors `idea`'s `internal/idea.IsTTY` and hop's own package-level seam idiom (`pickResolve`, `pickOne`, `listWorktrees`). It **probes `/dev/tty`** (opens it and runs `term.IsTerminal` on the result) — **not `os.Stdin`** — because fzf opens the controlling terminal directly for its picker UI and reads the candidate list hop pipes in from stdin separately (`internal/fzf.Pick` feeds a `strings.Reader` as fzf's stdin). Keying on `os.Stdin` would mis-report `hop </dev/null` (or any stdin redirect) at a real terminal as "no TTY" even though fzf could still run via `/dev/tty`; an agent/CI run with no controlling terminal cannot open `/dev/tty`, so the probe still correctly returns false there. (This is the PR-review correction in change `1x1u` — the initial implementation keyed on `os.Stdin.Fd()`; the Copilot review flagged the stdin-redirect false positive and it was fixed to the `/dev/tty` probe before merge.) Consumed by `resolve.go::resolveByName` (before `pickResolve`) and `config_rm.go::pickRepo` (before `pickOne`); `errNoTTY` maps to exit 3 in `main.go::translateExit` and `shim_plan.go::shimResolveErr`. See [cli/match-resolution](/cli/match-resolution.md) and [cli/agent-non-interactive-usage](/cli/agent-non-interactive-usage.md).

## Cross-references

- CLI grammar, `--shim-plan` protocol, shim shape: [cli/subcommands](/cli/subcommands.md)
- Agent / non-interactive contract (self-contained shim, `--json`, exit 3, `HOP_WRAPPER`): [cli/agent-non-interactive-usage](/cli/agent-non-interactive-usage.md)
- Match resolution + the TTY guard on fzf paths: [cli/match-resolution](/cli/match-resolution.md)
- Wrapper boundaries (`internal/proc`, `internal/fzf`, `internal/yamled`, `internal/scan` separation): [wrapper-boundaries](/architecture/wrapper-boundaries.md)
- `hop add` behavior (single-dir + recursive `-r`), classification rules, group assignment: [config/add-register](/config/add-register.md)
- Build pipeline: [build/local](/build/local.md)
