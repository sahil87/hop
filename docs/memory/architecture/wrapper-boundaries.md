---
description: "`internal/proc` security choke point (Run/RunCapture/RunInteractive/RunForeground), `internal/fzf` wrapper, `internal/yamled` comment-preserving YAML edits, `internal/scan` git invocation routing"
type: memory
---
# Wrapper Boundaries

How `hop` wraps external tools. Enforces Constitution Principle I (Security First) and Principle IV (Wrap, Don't Reinvent).

## `internal/proc` — the security choke point

All subprocess invocations in production code MUST go through `src/internal/proc/proc.go`. No production package outside `internal/proc/` may import `os/exec` directly. Verified by audit:

```
grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/
# → matches restricted to src/internal/proc/

grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/
# → zero matches (only exec.CommandContext is permitted)
```

Test files (`*_test.go`) MAY use `os/exec` directly — to spawn the built binary in integration tests, or to set up local git fixtures (e.g., `git init --bare` for ad-hoc URL clone tests). The audits scope to non-test code.

### API

| Symbol | Signature |
|---|---|
| `Run(ctx, name, args...) ([]byte, error)` | Non-interactive. Captures stdout to bytes; stderr passes through to parent. |
| `RunCapture(ctx, dir, name, args...) ([]byte, error)` | `Run` with an explicit `cmd.Dir`. Captures stdout, stderr passes through. Used by `internal/scan` for `git remote` / `git remote get-url` invocations scoped to a discovered repo's working tree (cmd.Dir is preferred over `git -C` so the subprocess sees the canonical cwd directly). |
| `RunInteractive(ctx, stdin io.Reader, name, args...) (string, error)` | Pipes stdin, captures stdout to string; stderr passes through. Used for fzf. |
| `RunForeground(ctx, dir, name, args...) (int, error)` | Runs a child with `cmd.Dir = dir` and stdin/stdout/stderr **inherited** from the parent. Returns the child's exit code on success (error nil); returns `(-1, ErrNotFound)` if the binary is missing; returns `(-1, err)` for other I/O / exec failures. The subprocess always inherits the parent's environment. When `dir` is `""`, the subprocess inherits the parent's working directory. Used by `hop <name> open` (with `dir = ""` — the verb forwards the path to wt as a positional arg rather than chdir'ing) and `internal/update`'s `brew upgrade`. Tool-form (`hop <name> <tool>`) runs in the parent shell via the shim's `RUN_IN_PARENT` plan, not through the binary — it has no `RunForeground` call site (gyo0). |
| `var ErrNotFound` | Sentinel returned when the binary is not on PATH. Callers use `errors.Is(err, proc.ErrNotFound)` to produce install-hint messages. |
| `ExitCode(err) (int, bool)` | Helper to extract the child's exit code from an `*exec.ExitError` without callers needing to import `os/exec`. |

All three runner functions use `exec.CommandContext(ctx, name, args...)` — never `exec.Command`, never shell strings. Callers supply the `context.Context` (with timeout for non-interactive ops; `context.Background()` for fzf and `wt open` since the user is at the keyboard / interacting with a menu).

## `internal/fzf` — fzf wrapper

`Pick(ctx, lines []string, query string) (string, error)`:

- Joins `lines` with `\n` and pipes via stdin to `fzf`.
- Argv built by `buildArgs(query)`: `--query <q>` (omitted when empty), then `--select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'`.
- All exec goes through `proc.RunInteractive` — no direct `os/exec`.
- A package-level `var runInteractive = proc.RunInteractive` provides a test seam for asserting argv composition without spawning fzf.
- Errors propagate directly; callers `errors.Is(err, proc.ErrNotFound)` to detect missing fzf.

Why a dedicated package: the invocation is non-trivial (multiple flags, stdin piping, query prefill) and used by 5+ subcommands. Worth one file.

## `internal/update` — Homebrew self-update

`Run(currentVersion string, out, errOut io.Writer) error`:

- Detects whether the binary was installed via Homebrew by walking `os.Executable()` through `filepath.EvalSymlinks` and checking for `/Cellar/` in the resolved path. Non-brew installs print a manual-update hint to `out` and return nil (exit 0).
- Refreshes the brew index (`brew update --quiet`, 30s timeout via `proc.Run`).
- Queries the latest tap formula version (`brew info --json=v2 sahil87/tap/hop`, parses `formulae[0].versions.stable`).
- Compares against `currentVersion` after stripping any leading `v` (binary reports `v0.0.3`, brew reports `0.0.3`).
- On mismatch, runs `brew upgrade sahil87/tap/hop` with a 120s timeout via `proc.RunForeground` so brew's progress streams through.
- All `brew` invocations route through `internal/proc` (Constitution Principle I).

Stream routing — `out` and `errOut` receive **only the wrapper messages this package emits** ("Current version:", "Already up to date.", error hints). Subprocess stdout/stderr from `brew update`, `brew info`, and `brew upgrade` is intentionally NOT routed through these writers — `internal/proc` owns subprocess streams (`proc.Run` pipes child stderr to the parent's `os.Stderr`; `proc.RunForeground` inherits all three streams). The split is deliberate: subprocess streams are tty-aware (brew prints colored progress); wrapper messages are small and may be redirected for tests or embedding. Production callers pass `os.Stdout` / `os.Stderr` to keep both consistent.

Error contract — when `brew` is missing on PATH, `Run` writes `hop update: brew not found on PATH.` to `errOut` and returns `proc.ErrNotFound`. The cobra wrapper in `cmd/hop/update.go` catches this with `errors.Is` and converts to `errSilent` so `translateExit` does not also print the underlying "binary not found on PATH" message.

Formula name: **`sahil87/tap/hop` (fully qualified)** to dodge a name collision with the Homebrew core `hop` cask (an HWP document viewer).

## `internal/yamled` — comment-preserving YAML write-back

`AppendURL(path, group, url string) error`:

- Reads the file, parses to `*yaml.Node`, navigates `repos.<group>`, appends a new scalar to either the sequence body (flat group) or the `urls:` child sequence (map-shaped group), marshals, and writes back via temp file + `os.Rename` (atomic on the same filesystem).
- Comments are preserved by the yaml.v3 round-trip. **Indentation is normalized to yaml.v3 defaults** — comment preservation is the contract, byte-perfect formatting is not.
- `ErrGroupNotFound` is a sentinel wrapped via `%w` when the named group is absent. Detect via `errors.Is(err, yamled.ErrGroupNotFound)`.

Why a dedicated package separate from `internal/config`: `config` validates and consumes; `yamled` produces a node tree, navigates, mutates, writes. Different responsibilities — `config` is the schema validator; `yamled` is a node-level mutator. Either can be tested independently.

## `internal/scan` — directory walk + repo classification

`Walk(ctx, root, opts) ([]Found, []Skip, error)`:

- Stack-based DFS with `(path, depth)` frames. Depth-bounded; symlinks followed with `(device, inode)` loop dedup (`syscall.Stat_t` keys). Each `Found.Path` is `filepath.EvalSymlinks`-resolved (canonical).
- Classifies each candidate dir via first-match-wins (`classifyDir`): worktree (`.git` is a regular file) → bare repo (HEAD + config + objects/, no `.git`) → normal repo (`.git` is a directory) → plain dir (recurse). `ReasonSubmodule` is part of the public Skip enum but never emitted — the no-descent invariant ("never enqueue children of a registered repo") makes nested `.git` dirs unreachable through DFS.
- All `git` invocations route through `Options.GitRunner`, which production binds to `internal/proc.RunCapture` (Constitution Principle I). Tests inject a fake `GitRunner` so no real `git` subprocess spawns. Each invocation gets a 5-second `context.WithTimeout`.
- The package is **UI-free**: knows about repos and skips, knows nothing about groups, slugify, conflict resolution, YAML, or stderr UX. The CLI layer (`cmd/hop/config_add.go`'s `runAdd`, with shared plan-building helpers in `cmd/hop/config_scan.go`) handles those concerns.

Why a dedicated package: discovery is non-trivial (DFS + inode dedup + classifier + git invocation), benefits from isolated unit tests with an injected `GitRunner`, and slots cleanly alongside `internal/yamled` and `internal/update` as a per-feature internal package. See [config/add-register](/config/add-register.md) for the per-rule details.

## What is NOT wrapped

Per Constitution Principle IV ("Wrap, Don't Reinvent") — wrap external tools, but don't over-package:

| External call | Where | Why no wrapper package |
|---|---|---|
| `git clone`, `git pull`, `git pull --rebase`, `git push` | `cmd/hop/clone.go`, `cmd/hop/pull.go`, `cmd/hop/sync.go` each call `proc.RunCapture(ctx, path, "git", ...)` (or `proc.Run` for clone) inline | Each call site is one line; `internal/proc.RunCapture` already enforces the cmd.Dir + argv-slice contract. A dedicated `internal/git/` package would be a thin pass-through that adds an indirection without containing logic. Promote later if a verb composes more git operations (e.g., a `status`-then-`fetch` flow that benefits from a single function boundary). |
| YAML parsing | `internal/config/config.go` calls `yaml.Unmarshal` directly into `*yaml.Node` | `gopkg.in/yaml.v3` already is the wrapper. |
| tool-form child exec | **none — the shim runs it** | Tool-form runs in the parent shell via the shim's `RUN_IN_PARENT` plan (`cd -- <path>; shift; "$@"`) — there is no Go-side exec to wrap; the binary only classifies and resolves a path (gyo0). |
| `wt open` invocation | `cmd/hop/open.go::runOpen` calls `proc.RunForeground(ctx, "", "wt", "open", repo.Path)` inline | Single operation; the binary is a transparent passthrough — the cd-handoff and env setup live in the shim, not the binary. A dedicated `internal/wt/` package would have nothing to encapsulate (one `RunForeground` call). |
| `wt list --json` invocation | `cmd/hop/wt_list.go::defaultListWorktrees` calls `proc.RunCapture(ctx, repoPath, "wt", "list", "--json")` and unmarshals into `[]WtEntry`. Used by `resolve.go::resolveWorktreePath` (single-worktree path resolution for the `<name>/<wt>` grammar suffix), `ls.go::runLsTrees` (fan-out for `hop ls --trees`), and `repo_completion.go` (both the post-slash `completeWorktreeCandidates` and the root-only pre-slash eager branch in `completeRepoNames` — odle). | Three distinct features, four call sites — same "promote later" rationale as `internal/git/`. Logic is a thin `RunCapture` + JSON unmarshal; a dedicated `internal/wt/` package would add an indirection without containing logic. The helper lives in `cmd/hop/` as a package-level `var listWorktrees = defaultListWorktrees` seam (mirroring `internal/fzf/fzf.go::runInteractive`) so tests inject fakes without spawning a real `wt`. Promote to `internal/wt/` if a fourth distinct feature emerges (e.g. a verb that needs `wt list --json` to drive new logic, not another consumer of the same single-shot-and-filter pattern). |

## `wt` integration (`cmd/hop/open.go` + shim)

`hop <name> open` delegates to `wt open <path>` for app detection, menu selection, and launching. wt is a hard runtime dependency declared as a Homebrew formula `depends_on "sahil87/tap/wt"` in `.github/formula-template.rb` (which the release workflow rewrites into `Formula/hop.rb` at tag time).

The binary is a transparent passthrough: it resolves the repo and `exec`s `wt open <path>` via `proc.RunForeground(ctx, "", "wt", "open", repo.Path)` with stdio fully inherited. **All env-var orchestration lives in the shell shim**, not the binary. This keeps the binary trivial (~20 lines) and respects the rule that interactive subprocesses can't multiplex stdout with a return-value channel.

**Unified `WT_CD_FILE` cd-channel (gyo0)**: `open` is classified as `PASSTHROUGH` by `--shim-plan`, and the shim's **inlined PASSTHROUGH arm** (inlined into `hop()` for partial-capture immunity — 1x1u; see [package-layout § single self-contained hop()](/architecture/package-layout.md#posixinit-is-a-single-self-contained-hop--partial-capture-immunity)) exports `WT_CD_FILE` on **every** passthrough invocation (not just `open`). One env var crosses the shim→wt boundary (the binary is a passive carrier — it's set on the shim's `command hop "$@"` invocation prefix-style, hop's process inherits it, and wt sees it via the parent env):

| Env var | Lifecycle | Purpose |
|---|---|---|
| `WT_CD_FILE` | the PASSTHROUGH arm creates a temp file via `mktemp -t hop-cd.XXXXXX`, exports its path on the `command hop "$@"` line, reads the file after the command exits, removes it via `rm -f` | wt writes the resolved repo path to this file iff the user picks "Open here"; for any other menu choice (editors, terminals, file managers) wt leaves the file empty. The shim reads the file with `[[ -s "$cdfile" ]]` and `cd -- "$target"` if non-empty (and the command exited 0). The **same channel** is reused by `hop clone <url>`, whose `writeCDTarget` writes the landed path to `WT_CD_FILE` when set. |

**Wrapper env vars**: the shim does NOT export `WT_WRAPPER=1` — the cd side-channel is only `WT_CD_FILE` (gyo0). It DOES export the hop-specific `HOP_WRAPPER=1` (1x1u), which the binary reads to suppress its shell-only hints (NOT a wt-boundary concern — see [cli/subcommands § HOP_WRAPPER hint suppression](/cli/subcommands.md#hop_wrapper--shell-only-hint-suppression) and [cli/agent-non-interactive-usage](/cli/agent-non-interactive-usage.md)). (If wt prints its "Open here requires the shell wrapper" hint in some configs, that is wt's own surface, not something hop suppresses.)

**Why temp file (not stdout capture)**: wt's app menu is interactive and renders to stdout. Capturing stdout with `$(...)` would swallow the menu and leave the user staring at a blank prompt while wt blocks on stdin. The temp file is a side-channel that keeps wt's stdio fully connected to the user's terminal. This single channel serves all three path-handoffs (gyo0): `where` prints to stdout for scripts (file stays empty, no cd); `open` and `clone <url>` route their cd-target through `WT_CD_FILE`.

**Path-arg, not chdir**: hop passes the resolved repo path to wt as a positional arg (`wt open <path>`) rather than chdir'ing first. wt has a "path-first" branch that opens the app menu when called with an existing-directory arg; chdir'ing into a main-repo cwd would instead trigger wt's worktree-selection menu (which is wrong for hop's use case — hop wants to open the directory, not pick a worktree underneath it).

**Why no `internal/wt` wrapper package**: the binary's call site is one `proc.RunForeground` line; there's nothing to encapsulate. The env-orchestration that *would* warrant a wrapper lives in the shim (shell, not Go). If hop grows additional wt-delegating verbs that share Go-side env construction (none planned), promote then.

**Direct binary invocation** (no shim, e.g., `/path/to/hop webapp open`): the binary still execs `wt open <path>` correctly. wt's interactive menu reaches the user's terminal. Picking "Open here" with no `WT_CD_FILE` set falls through to wt's own `cd -- '<path>'` printout + `wt shell-setup` install hint — that's wt's contract, surfacing transparently.

## `wt list --json` integration (`cmd/hop/wt_list.go`)

`wt list --json` is the second wt subcommand hop invokes from Go (alongside `wt open`). It supports the `hop <name>/<wt-name>` grammar suffix and the `hop ls --trees` flag (7eab). The invocation is a thin `proc.RunCapture` + JSON unmarshal — no env-var orchestration, no interactive stdio, no cd-handoff (those concerns belong to `open.go` and the shim).

### Contract

```go
const wtListTimeout = 5 * time.Second

type WtEntry struct {
    Name      string `json:"name"`
    Branch    string `json:"branch"`
    Path      string `json:"path"`
    IsMain    bool   `json:"is_main"`
    IsCurrent bool   `json:"is_current"`
    Dirty     bool   `json:"dirty"`
    Unpushed  int    `json:"unpushed"`
}

var listWorktrees = defaultListWorktrees

func defaultListWorktrees(ctx context.Context, repoPath string) ([]WtEntry, error) {
    ctx, cancel := context.WithTimeout(ctx, wtListTimeout)
    defer cancel()
    out, err := proc.RunCapture(ctx, repoPath, "wt", "list", "--json")
    if err != nil {
        return nil, err
    }
    return unmarshalWtEntries(out)
}
```

| Aspect | Value / rationale |
|---|---|
| Subprocess routing | `proc.RunCapture(ctx, repoPath, "wt", "list", "--json")` — Constitution Principle I (no direct `os/exec` outside `internal/proc/`). `cmd.Dir = repoPath` runs wt in the resolved repo's main checkout so wt discovers worktrees via `.git/worktrees/`. |
| Per-call timeout | 5 seconds, set via `context.WithTimeout` inside `defaultListWorktrees`. Matches `internal/scan`'s precedent for `git remote` invocations. wt list is a local op (reads `.git/worktrees/`) with no network round-trip, so 5s is generous. |
| JSON unmarshal | `encoding/json` default — `[]WtEntry` with `json` tags matching wt's documented schema. Unknown fields are silently ignored (no `DisallowUnknownFields`) so future wt schema additions don't break hop. Unmarshal failures are wrapped with `fmt.Errorf("wt list: %w", err)` so callers can route the error through the `hop: wt list: <err>` stderr line without further wrapping. |
| Test seam | Package-level `var listWorktrees = defaultListWorktrees` in `wt_list.go` — exactly the seam pattern used by `internal/fzf/fzf.go::runInteractive`. Tests swap `listWorktrees` to inject canned `[]WtEntry` or trigger error paths without needing a real `wt` binary on PATH. |
| Error surfaces | `proc.ErrNotFound` (wt missing on PATH — callers match via `errors.Is` to produce `wtMissingHint`); JSON unmarshal errors wrapped with `wt list:` prefix; any other subprocess error returned verbatim. All four wt-list error wordings (uncloned-with-`/`, wt missing, malformed JSON, no-such-worktree) are formatted by the callers (`resolveWorktreePath` and `runLsTrees`), not by `defaultListWorktrees` itself. |

### Call sites

- `resolve.go::resolveWorktreePath` — single-worktree lookup invoked by `resolveByName`'s `/`-suffix branch. Resolves `hop <name>/<wt> where`, `hop <name>/<wt> open`, the `CD`/`RUN_IN_PARENT` plans for `hop <name>/<wt>` (bare cd and tool-form), and the `pull`/`push`/`sync` batch verbs on a `<name>/<wt>` selection.
- `ls.go::runLsTrees` — fans `wt list --json` across every cloned repo in `hop.yaml` source order for the `hop ls --trees` flag. Per-row failures degrade gracefully as inline `(wt list failed: <err>)` rows; the FIRST `proc.ErrNotFound` aborts the run with `wtMissingHint`.

### Why no `internal/wt/` package

Two call sites is below the threshold for a dedicated wrapper package — same "promote later" rationale as `internal/git/`'s non-creation (which has more call sites than this and still stays inline). The helper is a thin `RunCapture` + JSON unmarshal; extracting to `internal/wt/` would add an indirection without containing logic. The `cmd/hop/wt_list.go` location keeps the `WtEntry` type next to its only consumers (`resolveWorktreePath` and `runLsTrees`) and keeps the test seam in the same package as the tests that swap it. Promote to `internal/wt/` if a third call site emerges (e.g., a future `hop wt-status`-shaped verb).

### Why no env-var orchestration

Unlike `wt open` (whose cd-target rides the shim's unified `WT_CD_FILE` channel for the "Open here" handoff), `wt list --json` is a pure read — no shell-mutation side channel, no interactive stdio, no orchestration needed. The hop side is just `RunCapture` and unmarshal; no shim changes were required for this surface.

## Composability primitives

The composability primitives under the selection-first grammar (gyo0):

- **`hop <name> where`** — path resolver. Stdin/stdout-friendly: `cd "$(hop webapp where)"` works as a shell composition (and is the scriptable fallback for callers that bypass the shim). Reached via the `PASSTHROUGH` plan so it stays a real binary command printing to stdout. The selection-first grammar puts the repo first (there is no top-level `hop where <name>`).
- **Tool-form `hop <name> <tool>...`** — exec-in-context, owned by the **shell**, not the binary (gyo0). The `--shim-plan` classifier returns `RUN_IN_PARENT\n<path>` and the shim runs `cd -- <path>; shift; "$@"`, executing the user's already-parsed words in the parent shell — the binary never execs the child. Because the shim runs the user's literal words, aliases and shell functions resolve (a binary exec could not honor them).

The `pull`/`push`/`sync` batch verbs (action tokens; gyo0) compose a third primitive — `resolveTargets` (`cmd/hop/resolve.go`) — which wraps `resolveByName` with an `--all`/name-or-group rule order so the verbs can take a single repo, a worktree, a group, or `--all` from the same selection slot (`runBatchVerb` in `batch_verb.go`). Future batch-friendly verbs build on `resolveTargets` rather than each re-implementing the rule order. Path resolution stays on `where` (binary) and the `CD`/`RUN_IN_PARENT` plans (shim).

## Security guarantees

1. **`exec.CommandContext` everywhere** — kernel never sees a shell string; argv is an explicit slice.
2. **User input passes as args, not shell tokens** — repo names from `hop.yaml` reach `git clone` via `proc.Run("git", "clone", url, path)`; fzf queries reach fzf as `--query <q>` (a single arg) and the candidate list is on stdin.
3. **`--shim-plan` is not an injection surface** (gyo0) — the classifier never execs the user's action; it emits only the fixed 3-keyword vocabulary plus a resolved path used as a quoted `cd` operand. The shim runs the user's **already-parsed** `"$@"` (`RUN_IN_PARENT`) and **never `eval`s** binary stdout, so binary output is never re-parsed as code.
4. **No `sh -c`, no `bash -c`, no command-string interpolation anywhere in production code.**
5. **Atomic file writes** — `internal/yamled` uses temp file + rename in the same directory, preserving the original on rename failure.
