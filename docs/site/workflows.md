# hop workflows: one grammar, three jobs

hop has exactly one shape to learn:

```
hop <selection> <action>
```

The **selection** always comes first — a repo, a worktree, a group, or `--all`. The **action** is everything after it — a builtin verb (`cd`, `open`, `where`), a git batch verb (`pull`, `push`, `sync`), or *any* command, PATH binary, or shell alias. Learn the grammar once and every workflow below is just a substitution into those two slots.

If you haven't wired the shell shim yet, do the [install guide](install.md) first — navigation and running commands in a repo are shim-only (a binary can't change its parent shell's cwd).

## The three selection forms

### Singular — one repo

```sh
h web                      # substring match on the repo name → cd into webapp
hop webapp where           # print the absolute path
hop webapp cursor .        # run cursor inside webapp, then return you to where you were
hop webapp pull            # git pull a single repo
```

`h` is the single-letter alias for the `hop` shell function. The selection is matched by **substring on the repo name only** — not URL, not path, not group.

### Plural — a group or `--all`

A plural selection fans the git batch verbs across many repos:

```sh
hop --all pull             # pull every cloned repo in hop.yaml
hop default pull           # pull every cloned repo in the `default` group
hop work sync              # rebase-and-push every repo in the `work` group
hop --all push
```

A plural selection accepts **only** the batch verbs (`pull` / `push` / `sync`). `cd`, `open`, `where`, and arbitrary tools across N repos are refused — running an interactive tool across many repos makes no sense.

### Worktree — a `/<wt>` suffix on the selection

The selection slot takes an optional `/<wt-name>` suffix; every action inherits it:

```sh
h webapp/feat-x                 # cd into the feat-x worktree of webapp
hop webapp/feat-x where         # path of that worktree
hop webapp/feat-x git status    # run git in that worktree
hop webapp/main where           # the main checkout (same as bare `hop webapp where`)
```

The suffix resolves via `wt list --json`, so [`wt`](https://github.com/sahil87/wt) must be on `PATH` for any `/`-suffixed query. Bare `hop webapp` never invokes `wt`. Tab completion is slot-aware: `hop webapp/<TAB>` lists that repo's worktrees.

## Job 1 — navigate

```text
$ hop ls
webapp     /Users/sahil/code/sahil87/webapp
fab-kit    /Users/sahil/code/sahil87/fab-kit
wt         /Users/sahil/code/sahil87/wt
…

$ h web                  # substring match → cd into webapp
$ h webapp/feat-x        # cd into the feat-x worktree
$ hop ls --trees         # worktree status across every cloned repo
webapp    3 trees  (main, feat-x*, hotfix↑2)
hop       2 trees  (main, refactor-resolve)

$ hop                    # bare → fzf picker over all repos
```

In `hop ls --trees`, `*` marks a dirty worktree and `↑N` marks unpushed commits.

## Job 2 — run anything inside a repo

`hop <selection> <action>` runs *anything* with cwd set to that repo, then returns you to where you started. The selection is the first arg; the rest is the action — forwarded verbatim and run in your shell, so your aliases and functions resolve.

```text
$ pwd
/tmp/scratch
$ hop webapp git status
On branch main
nothing to commit, working tree clean
$ pwd
/tmp/scratch             # cwd unchanged
```

Useful variants:

```sh
hop dotfiles cursor .             # open dotfiles in Cursor (the trailing . matters — see Gotchas)
hop infra-tf terraform plan       # run terraform inside infra-tf
hop webapp p                      # run your shell alias `p` inside webapp
hop webapp open                   # delegate to wt's app menu (editor / terminal / cd here)
```

## Job 3 — batch git ops over groups

The batch verbs `pull`, `push`, and `sync` follow the same grammar, with a plural selection:

```sh
hop webapp pull            # single repo
hop default pull           # every cloned repo in the `default` group
hop --all pull             # every cloned repo in hop.yaml
hop work sync              # `git pull --rebase` then `git push` for every repo in `work`
```

Each run emits a per-repo `✓` / `✗` / `skip` line on stderr and a final `summary: pulled=N skipped=M failed=K`. `sync` skips the push when a rebase conflicts and prints a `git -C <path> rebase --continue` hint. Uncloned repos are silently skipped — run `hop --all clone` first if you want to materialize them.

## The grammar at a glance

| You type | hop does |
|----------|----------|
| `hop` | Bare fzf picker over all repos. |
| `hop <subcommand>` (`ls`, `clone`, `add`, `rm`, `config`, `update`, …) | Routes to the subcommand. |
| `hop <name>` | `cd` into the repo. |
| `hop <name> where` | Prints the absolute path. |
| `hop <name> open` | Delegates to wt's app menu. |
| `hop <name> <cmd> …` | Runs `<cmd> …` (command, PATH binary, or shell alias) with cwd = repo, in your shell. |
| `hop <name> pull` / `push` / `sync` | Git batch verb on the selection. |
| `hop <name>/<wt>` | Same as `hop <name>` but lands in the named worktree (via `wt list --json`); every action accepts the suffix. |
| `hop --all <verb>` / `hop <group> <verb>` | Plural selection — batch verb (`pull`/`push`/`sync` only) across every matched repo. |
| `hop ls --trees` | Per-repo worktree summary (`*` = dirty, `↑N` = unpushed). |

## The shim-vs-binary dispatch model

The `hop` shell function asks the binary how to dispatch each invocation (an internal `hop --shim-plan …` call) and gets back one of three answers:

1. **cd here** — change the shell's cwd (for `hop <name>` / `hop <name> cd`);
2. **run this in the parent shell** — cd into the repo, run your typed words, then return (for `hop <name> <cmd>`);
3. **pass through to the binary** — forward subcommands like `hop ls` straight to the binary.

The shim never `eval`s the binary's output — it runs the words you already typed, so there's no injection surface and no command rewriting. Because the binary owns the classification, the shim hard-codes no subcommand names and can't drift out of sync as the binary gains new subcommands.

### Scripting and CI

In scripts or CI that bypass the shim, invoke the binary directly with `command hop`:

```sh
cd "$(command hop webapp where)"   # resolve the path, then cd yourself
command hop --all pull             # batch ops work fine without the shim
```

`command hop <name> where` prints the path instead of `cd`-ing — the binary handles `where` directly, so it's the reliable building block for automation.

## Gotchas

- **`hop <name>`, `hop <name> <cmd>`, and `h <name>` need the shell shim.** A binary can't change its parent shell's cwd or run a command in it — the same Unix constraint `wt` hits with "Open here." Without the shim, the binary prints a hint pointing at `eval "$(hop shell-init zsh)"`, or use `cd "$(command hop <name> where)"`.
- **Running commands in a repo is shim-only.** The shim cds in and runs your action in the parent shell, so PATH binaries *and* your aliases/functions resolve. In scripts, resolve the path yourself with `command hop <name> where` and run the tool from there.
- **Substring match is on the repo name only.** Not URL, not path, not group. `hop web` matches `webapp` but not `git@github.com:org/webapp.git`. When two repos in different groups share a name, the picker shows `name [group]` to disambiguate.
- **No `--force` on the `push` / `sync` batch verbs.** Intentional — for nuanced single-repo cases reach for `hop <name> git push --force` (you get the full git output). The batch verbs stay safe by default.
- **`hop <name> cursor` / `code` need a trailing `.`** — e.g. `hop dotfiles cursor .`. Both editors take `[paths…]`; invoked with none, they restore the previously open folder instead of opening the cwd. The `.` says "open *this* directory." Tools that operate on cwd by default (`git status`, `terraform plan`, `ls`, `npm test`) don't need it.
- **The `<name>/<wt>` suffix needs `wt` on `PATH`.** hop shells out to `wt list --json` to resolve the worktree (no state cached in `hop.yaml` — worktrees are wt's domain). Bare `hop <name>` never invokes `wt`. Homebrew pulls `wt` in as a dependency; for non-brew installs, `brew install sahil87/tap/wt` or build from source.

## See also

- [Install guide](install.md) — install, shell integration, and first-run bootstrap.
- [`cli-surface.md`](https://github.com/sahil87/hop/blob/main/docs/specs/cli-surface.md) — the canonical CLI contract (every subcommand, exit codes, every behavioral scenario).
- [`config-resolution.md`](https://github.com/sahil87/hop/blob/main/docs/specs/config-resolution.md) — config search order and `hop.yaml` schema.
