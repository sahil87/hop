# hop

> Part of [@sahil87's open source toolkit](https://shll.ai) — see all projects there.

[![Latest release](https://img.shields.io/github/v/release/sahil87/hop)](https://github.com/sahil87/hop/releases) [![Downloads](https://img.shields.io/github/downloads/sahil87/hop/total)](https://github.com/sahil87/hop/releases) [![Stars](https://img.shields.io/github/stars/sahil87/hop?style=social)](https://github.com/sahil87/hop/stargazers)

A small Go CLI that turns one config file (`hop.yaml`) into a personal directory of all your git repos — navigate, clone, run commands, and batch-update them from any directory.

## Install

```sh
curl -fsSL https://shll.ai/install | sh -s -- hop
```

Installs hop (plus the shll meta-CLI) via Homebrew, handling tap trust automatically. To install the entire sahil87 toolkit instead:

```sh
curl -fsSL https://shll.ai/install | sh
```

## Why hop?

- **One config, every machine** — `hop.yaml` lives at `~/.config/hop/hop.yaml` and lists every repo you care about (with groups). Keep it in your dotfiles and symlink that path to it, and your repo directory follows you between laptops.
- **Substring navigation** — `h web<TAB>` or `hop web` matches `webapp` and `cd`s your shell straight there. No more `cd ~/code/sahil87/webapp`.
- **Run anything inside a repo, from anywhere** — `hop dotfiles cursor .` opens your dotfiles in Cursor without changing your cwd. Works for any tool, PATH binary, or shell alias: `hop webapp git status`, `hop infra-tf terraform plan`, `hop loom npm test`.
- **Batch git ops over groups** — `hop --all pull` pulls every cloned repo. `hop work sync` rebases-and-pushes every repo in the `work` group. Group-level fan-out built in.
- **Bootstrap from disk, not yaml-by-hand** — `hop add -r ~/code` walks your existing clones, reads `git remote`, and populates `hop.yaml` for you. Comment-preserving merges, idempotent re-runs.
- **Plays nicely with [`wt`](https://github.com/sahil87/wt)** — `hop <name> open` delegates to wt's app menu, so you get the same "open in editor / terminal / file manager / cd here" experience for every repo in the registry. The `hop <name>/<wt-name>` suffix lands you straight inside a worktree (`h webapp/feat-x` cds you there, `hop webapp/feat-x git status` runs git in it), and `hop ls --trees` shows worktree state across every repo at a glance.

## The mental model

**One grammar:** `hop <selection> <action>`. The selection comes first — always. Then an action: a builtin verb (`cd`, `open`, `where`), a git batch verb (`pull`, `push`, `sync`), or *any* command, PATH binary, or shell alias.

> **Colored half** = what you type · **grey half** = what the `h` alias or tab completion fills in.

**General form** — `hop <selection> <action>`:

![hop](https://img.shields.io/badge/h-op-9ca3af?labelColor=1f6feb&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![repo](https://img.shields.io/badge/re-po--name-9ca3af?labelColor=7c3aed&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![cmd](https://img.shields.io/badge/command-059669?style=for-the-badge)

```sh
h  web<TAB>     cursor .            # → hop webapp cursor .
h  dot<TAB>     ls -la              # → hop dotfiles ls -la
h  web<TAB>     git status         # → hop webapp git status
h  web<TAB>     pull               # → hop webapp pull (git batch verb)
```

The action is whatever follows the selection — a builtin verb, a git verb, a PATH binary, or even a shell alias you've defined (`hop webapp p`). The shim runs it with cwd set to the repo, then returns you to where you started.

**Plural form** — the selection can be a group or `--all`, fanning the git batch verbs across many repos:

![hop](https://img.shields.io/badge/h-op-9ca3af?labelColor=1f6feb&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![sel](https://img.shields.io/badge/-%2D%2Dall%20%2F%20group-9ca3af?labelColor=7c3aed&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![git](https://img.shields.io/badge/pull%20%2F%20push%20%2F%20sync-d97706?style=for-the-badge)

```sh
hop  --all   pull                   # pull every cloned repo
hop  work    sync                   # 'work' is a group → sync each repo in it
hop  --all   push
```

A plural selection accepts only the batch verbs (`pull`/`push`/`sync`) — `cd`, `open`, `where`, and arbitrary tools across many repos are refused (running an interactive tool across N repos makes no sense).

**Worktree form** — the selection slot accepts an optional `/<wt-name>` suffix. Everything else is unchanged; every action inherits it:

![hop](https://img.shields.io/badge/h-op-9ca3af?labelColor=1f6feb&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![repo](https://img.shields.io/badge/re-po--name-9ca3af?labelColor=7c3aed&style=for-the-badge) ![sep](https://img.shields.io/badge/-%2F-lightgrey?style=for-the-badge) ![wt](https://img.shields.io/badge/wo-rktree-9ca3af?labelColor=0d9488&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![cmd](https://img.shields.io/badge/command-059669?style=for-the-badge)

```sh
h  web<TAB>             # multi-worktree repo → TAB surfaces webapp, webapp/feat-x, webapp/hotfix
h  web<TAB>/fe<TAB>     git status              # → hop webapp/feat-x git status
h  web<TAB>/<TAB>       cursor .                # → TAB after / lists worktrees of webapp
hop  web<TAB>/main      where                   # → main-worktree path (same as bare `hop webapp where`)
```

The `/<wt-name>` resolves via `wt list --json`, so wt must be on `PATH` for any `/`-suffixed query — bare `hop webapp` is unaffected. The full grammar (subcommands, verbs, flags) is in [Grammar at a glance](#grammar-at-a-glance) below.

## Other ways to install

To upgrade later, run `hop update` — self-upgrades via Homebrew. When `hop` was installed from source or a release tarball, it prints a hint and exits without invoking brew.

### From source

```sh
git clone https://github.com/sahil87/hop.git
cd hop
just install
```

Builds the binary and copies it to `~/.local/bin/hop`. Make sure that directory is on your `$PATH`.

> **Fuller guide:** [Installing and setting up hop](docs/site/install.md) — Homebrew & from-source, shell integration in depth, and first-run bootstrap.

## Shell integration

The shell shim is what makes `hop <name>` actually `cd` your shell. Install it once:

```sh
eval "$(hop shell-init zsh)"   # in ~/.zshrc
eval "$(hop shell-init bash)"  # in ~/.bashrc
```

This installs the `hop` shell function, the `h` alias, and tab completion. The shell function asks the binary how to dispatch each invocation (cd, run-in-parent-shell, or pass through to the binary) and acts on the answer — so navigation and running commands/aliases in a repo's directory work, none of which a bare binary can do (changing the parent shell's cwd is a Unix constraint, not a hop limitation). See [Gotchas](#gotchas) for the shim-vs-binary details.

> 💡 Have other sahil87 tools? [`shll shell-install`](https://github.com/sahil87/shll#shll-shell-install--wire-the-rc-file-recommended) handles all of their shell integrations and autocompletions at once.

## First run

If you already have repos cloned somewhere, point hop at them — this creates `hop.yaml` for you and registers everything it finds:

```sh
hop add -r ~/code                   # walk ~/code and populate hop.yaml (comments preserved)
hop add -r -p ~/code                # preview only: print what it would write, change nothing
```

`hop add -r` writes by default and auto-creates the config on a fresh machine — there's no separate setup step. The file lives at `~/.config/hop/hop.yaml` (run `hop config where` to confirm). To sync it across machines, keep it in your dotfiles and symlink that path to it.

The walk (default depth 3, `--depth N` to override) inspects each git repo's `origin` remote and auto-derives groups: repos whose on-disk path matches the `<code_root>/<org>/<name>` convention land in `default`; repos in non-convention layouts get a group named after their parent directory. Add `-g <name>` to force everything into a named group instead (auto-created if it doesn't exist). Worktrees, submodules, bare repos, and repos with no remote are skipped.

Prefer to start from a hand-edited starter instead? `hop config init` writes an annotated `hop.yaml` you can fill in by hand.

## Quick tour

Three things hop does. Each is a complete mental model on its own.

### 1. Navigate

```text
$ hop ls
prompt-pantry  /Users/sahil/code/sahil-weaver/prompt-pantry
webapp         /Users/sahil/code/sahil87/webapp
fab-kit        /Users/sahil/code/sahil87/fab-kit
wt             /Users/sahil/code/sahil87/wt
idea           /Users/sahil/code/sahil87/idea
…

$ hop webapp where
/Users/sahil/code/sahil87/webapp

$ h web                       # substring match → cd into webapp
$ pwd
/Users/sahil/code/sahil87/webapp

$ h webapp/feat-x             # cd into the feat-x worktree of webapp (via wt list --json)
$ pwd
/Users/sahil/code/sahil87/webapp.worktrees/feat-x

$ hop ls --trees              # one-shot worktree status across every cloned repo
webapp    3 trees  (main, feat-x*, hotfix↑2)
dotfiles  1 tree   (main)
hop       2 trees  (main, refactor-resolve)
loom      (not cloned)

$ hop                         # bare → fzf picker over all repos, prints selection
```

`h` is the single-letter alias for the `hop` shell function. To invoke the binary directly (bypassing the shim — e.g. for scripting), use `command hop <name> where`, which prints the path instead of `cd`-ing.

The `/<wt-name>` suffix is optional, completes after `<TAB>` (so `h webapp/<TAB>` lists worktrees), and every action inherits it — `hop webapp/feat-x where`, `hop webapp/feat-x open`, `hop webapp/feat-x git status`, all work. Bare `hop webapp` is unchanged. `wt` must be on `PATH` for any `/<wt>`-suffixed query.

### 2. Run anything inside a repo

`hop <selection> <action>` runs *anything* with cwd set to that repo, then returns you to where you started. The selection is the first arg; the rest is the action — a command, PATH binary, or shell alias — forwarded verbatim and run in your shell (so your aliases and functions resolve).

```text
$ pwd
/tmp/scratch

$ hop webapp git status
On branch main
Your branch is up to date with 'origin/main'.

nothing to commit, working tree clean

$ pwd
/tmp/scratch                  # cwd unchanged
```

A few useful variants:

```sh
hop dotfiles cursor .                # open dotfiles in Cursor (the trailing . matters — see Gotchas)
hop infra-tf terraform plan          # run terraform inside infra-tf
hop webapp p                         # run your shell alias `p` inside webapp
hop webapp open                      # delegates to wt's app menu (editor / terminal / cd here)
```

### 3. Batch git ops

The git batch verbs `pull`, `push`, and `sync` follow the same `hop <selection> <action>` grammar. The selection can be a single repo, a group, or `--all`:

```sh
hop webapp pull                # pull a single repo
hop default pull               # pull every cloned repo in the `default` group
hop --all pull                 # pull every cloned repo in hop.yaml
hop work sync                  # `git pull --rebase` then `git push` for every repo in `work`
hop --all sync                 # same, every repo
```

Each command emits a per-repo `✓` / `✗` / `skip` line on stderr and a final `summary: pulled=N skipped=M failed=K`. `sync` skips the push when rebase hits a conflict and prints a `git -C <path> rebase --continue` hint. Uncloned repos are silently skipped — `hop --all clone` first if you want to materialize them. A plural selection (`--all` or a group) accepts only these batch verbs.

## Grammar at a glance

The rule is one shape: **`hop <selection> <action>`.** The first positional is either a subcommand or a selection (repo / worktree / group / `--all`); everything after a selection is the action.

| You type | hop does |
|----------|-----------|
| `hop` | Bare fzf picker over all repos. |
| `hop <subcommand>` (`ls`, `clone`, `add`, `rm`, `config`, `update`, …) | Routes to the subcommand. |
| `hop <name>` | `cd` into the repo. |
| `hop <name> cd` | Same — explicit verb form. |
| `hop <name> where` | Prints the absolute path. |
| `hop <name> open` | Delegates to wt's app menu. |
| `hop <name> <cmd> ...` | Runs `<cmd> ...` (command, PATH binary, or shell alias) with cwd = repo, in your shell. |
| `hop <name> pull` / `push` / `sync` | Git batch verb on the selection (repo, worktree, or group). |
| `hop <name>/<wt>` | Same as `hop <name>` but lands in the named worktree (resolved via `wt list --json`). All actions above accept the suffix — `<name>/<wt> where`, `<name>/<wt> open`, `<name>/<wt> git status`, etc. |
| `hop --all <verb>` / `hop <group> <verb>` | Plural selection — runs the batch verb (`pull`/`push`/`sync` only) across every matched repo. |
| `hop ls --trees` | Per-repo worktree summary across the registry (`*` = dirty, `↑N` = unpushed commits). |

Tab completion knows which slot you're in: `hop <TAB>` offers subcommands + repo names; `hop webapp <TAB>` offers verbs + tools; `hop webapp/<TAB>` offers worktree names for that repo. When `hop <prefix><TAB>` uniquely resolves to a cloned repo with 2+ worktrees, completion surfaces both the bare repo and `<repo>/<wt>` candidates inline — press Space to commit the main checkout, or pick a worktree from the menu.

Under the hood, the shell function asks the binary how to dispatch (`hop --shim-plan …`, an internal call) and gets back one of three answers — cd here, run this in the parent shell, or pass through to the binary. The shim never `eval`s binary output; it runs your already-typed words. Because the binary owns the classification, the shim hard-codes no subcommand names and can't drift out of sync with the binary.

> **Deep-dive:** [hop workflows](docs/site/workflows.md) — the three jobs, worktree workflows, the shim-vs-binary dispatch model, and gotchas, with worked examples.

## Config schema

`hop.yaml` is grouped by named sections under `repos:`, with optional global `config:` fields:

```yaml
config:
  code_root: ~/code   # optional; defaults to ~

repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git

  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:some-vendor/their-tool.git
```

A flat list (`default` above) uses convention: each URL lands at `<code_root>/<org>/<name>`. A map with `dir:` (`vendor` above) overrides convention: each URL lands at `<dir>/<name>`, with `org` ignored. Group names match `^[a-z][a-z0-9_-]*$`.

## Gotchas

- **`hop <name>`, `hop <name> <cmd>`, and `h <name>` need the shell shim.** A binary can't change its parent shell's cwd, nor run a command in it — that's the same Unix constraint wt hits with its "Open here" menu option. Without the shim, the binary prints a hint pointing at `eval "$(hop shell-init zsh)"` or the workaround `cd "$(command hop <name> where)"`.
- **Running commands in a repo is shim-only.** The shim cd's into the repo and runs your action in the parent shell — so PATH binaries *and* your shell aliases/functions resolve. In scripts and CI that bypass the shim, resolve the path yourself with `command hop <name> where` (handled directly by the binary) and run the tool from there.
- **Substring match is on the repo name only.** Not URL, not path, not group. `hop web` matches `webapp` but not the URL `git@github.com:org/webapp.git`. When two repos in different groups share a name, the picker shows `name [group]` to disambiguate.
- **No `--force` on the `push` / `sync` batch verbs.** Intentional — for nuanced single-repo cases, reach for `hop <name> git push --force` and you'll get the full git output. The batch verbs stay safe by default.
- **`hop <name> cursor` / `code` need a trailing `.`** — e.g. `hop dotfiles cursor .`. Not a hop quirk: both editors take `[paths...]` as positional args and, when invoked with none, restore the previously open folder instead of opening the cwd. The `.` is what tells them "open *this* directory." Tools that operate on cwd by default (`git status`, `terraform plan`, `ls`, `npm test`) don't need it.
- **The `<name>/<wt>` suffix needs `wt` on `PATH`.** Hop shells out to `wt list --json` to resolve the worktree name (no state cached in `hop.yaml` — worktrees are wt's domain). Bare `hop <name>` queries never invoke wt. The Homebrew formula pulls wt in as a dependency; for non-brew installs, `brew install sahil87/tap/wt` or build from source.

## Reference

- `hop --help` — full subcommand listing (rendered online at [shll.ai/tools/hop/commands](https://shll.ai/tools/hop/commands/))
- [Install guide](docs/site/install.md) — install, shell integration, and first-run bootstrap in depth
- [Workflows deep-dive](docs/site/workflows.md) — the grammar, daily workflows, the shim model, and gotchas
- [`docs/specs/cli-surface.md`](https://github.com/sahil87/hop/blob/main/docs/specs/cli-surface.md) — canonical CLI contract (every subcommand, exit codes, stdout/stderr conventions, every behavioral scenario)
- [`docs/specs/config-resolution.md`](https://github.com/sahil87/hop/blob/main/docs/specs/config-resolution.md) — config search order and `hop.yaml` schema
