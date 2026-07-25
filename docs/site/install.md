# Installing and setting up hop

A complete walkthrough from "nothing installed" to "`h web<TAB>` cds my shell into webapp." Five steps: install the binary, wire the shell shim, bootstrap `hop.yaml` from your existing clones, sync that config across machines, and keep hop up to date.

For the day-to-day grammar once you're set up, see the [workflows deep-dive](workflows.md).

## TL;DR

```sh
curl -fsSL https://shll.ai/install | sh -s -- hop   # install hop (+ shll) via Homebrew
shll shell-setup                                    # wire the shell shim into your rc file
hop add -r ~/code                                   # walk your code dir, build hop.yaml from git remotes
```

Then open a new terminal — or `exec zsh` (bash: `exec bash`) — so the freshly written rc file loads; the `hop` function and `h` alias only exist in shells started after `shll shell-setup`.

That's the whole happy path. The rest of this page unpacks each step — and covers from-source installs, previewing the bootstrap, config syncing, and updates.

## 1. Install the binary

### Via shll (macOS and Linux)

```sh
curl -fsSL https://shll.ai/install | sh -s -- hop
```

Installs hop (plus the shll meta-CLI) via Homebrew under the hood, handling tap trust automatically. To install the entire shll toolkit instead, run `curl -fsSL https://shll.ai/install | sh` — see [shll.ai](https://shll.ai) for the complete install story.

[`wt`](https://github.com/sahil87/wt) is a sibling tool and is **not** installed automatically. hop shells out to `wt list --json` to resolve the `<name>/<wt>` worktree suffix, so having `wt` on `PATH` is what makes worktree navigation (`h webapp/feat-x`) and `hop ls --trees` work — install it with `shll install wt` (or the full-toolkit bootstrap above). Bare `hop webapp` queries never touch `wt`.

### From source

```sh
git clone https://github.com/sahil87/hop.git
cd hop
just install
```

`just install` builds the binary and copies it to `~/.local/bin/hop`. Make sure that directory is on your `$PATH` (`export PATH="$HOME/.local/bin:$PATH"` in your rc file if it isn't). The build follows the thin-justfile pattern — `just install` delegates to a script under `scripts/`, so there's no hidden build state to manage.

A from-source install does **not** pull in `wt`. If you want worktree navigation, install it separately via [shll.ai](https://shll.ai) (`shll install wt`, or `curl -fsSL https://shll.ai/install | sh -s -- wt` if you don't have `shll`), or build it from source the same way.

## 2. Wire the shell shim

This is the step that turns hop from a path-printer into a navigator. A binary cannot change its parent shell's current directory, nor run a command in it — that's a Unix constraint, not a hop limitation. The shim is a tiny shell function that bridges the gap.

If you installed via shll (step 1's recommended path), wire it with one command:

```sh
shll shell-setup
```

It's idempotent — re-running is a no-op — and it wires every installed shll tool's shell integration and completions (hop, `wt`, and friends) into your rc file in one shot, so there's no per-tool `eval` line to manage. See [shll.ai](https://shll.ai) for details and variants.

Either way you wire it — via `shll shell-setup` or the manual line below — it's an rc-file edit, so it takes effect in new shells only: open a new terminal or `exec zsh` before trying `h`.

### Installed from source? Wire the shim manually

A from-source install doesn't come with `shll`, so add hop's line to your shell's rc file yourself:

```sh
eval "$(hop shell-init zsh)"   # add to ~/.zshrc
eval "$(hop shell-init bash)"  # add to ~/.bashrc
```

### What the shim installs

Either way, `hop shell-init <shell>` prints (and the `eval` — run directly or via `shll shell-setup`'s composed line — installs) three things:

- **the `hop` shell function** — wraps the binary and acts on its dispatch decision (see below);
- **the `h` alias** — a single-letter shorthand for the `hop` function, so `h web<TAB>` works;
- **tab completion** — slot-aware: `hop <TAB>` offers subcommands and repo names, `hop webapp <TAB>` offers verbs and tools, `hop webapp/<TAB>` offers that repo's worktree names.

### How dispatch works (why the shim is safe)

For each invocation, the `hop` function asks the binary how to dispatch (an internal `hop --shim-plan …` call) and gets back exactly one of three answers:

1. **cd here** — for `hop <name>` / `hop <name> cd`, the function changes your shell's cwd;
2. **run this in the parent shell** — for `hop <name> <cmd>`, the function cds into the repo, runs your already-typed words in your shell (so your aliases and functions resolve), then returns you to where you started;
3. **pass through to the binary** — for subcommands like `hop ls` or `hop add`, the function just forwards to the binary.

The shim never `eval`s the binary's output — it runs the words you already typed. Because the binary owns the classification, the shim hard-codes no subcommand names and can't drift out of sync with the binary as new subcommands are added.

If you ever run hop without the shim installed, navigation prints a hint pointing you back to `eval "$(hop shell-init zsh)"`, with `cd "$(command hop <name> where)"` as the manual workaround.

## 3. First run: bootstrap `hop.yaml` from disk

You don't write `hop.yaml` by hand. If you already have repos cloned somewhere, point hop at them and it builds the config for you:

```sh
hop add -r ~/code          # walk ~/code, read each repo's git remote, populate hop.yaml
hop add -r -p ~/code       # preview only: print what it would write, change nothing
```

`hop add -r` writes by default and auto-creates the config on a fresh machine — there's no separate "init" step. Use `-p` (preview) first if you want to see the plan before committing to it.

The walk defaults to depth 3; override with `--depth N`. For each git repo it finds, it inspects the `origin` remote and **auto-derives a group**:

- repos whose on-disk path matches the `<code_root>/<org>/<name>` convention land in the `default` group;
- repos in non-convention layouts get a group named after their parent directory.

Pass `-g <name>` to force everything into one named group instead (auto-created if it doesn't exist). Worktrees, submodules, bare repos, and repos with no remote are skipped.

Prefer to start from a hand-edited template? `hop config init` writes an annotated `hop.yaml` you can fill in yourself.

## 4. Where `hop.yaml` lives, and syncing it across machines

The config lives at `~/.config/hop/hop.yaml`. Confirm the resolved path any time with:

```sh
hop config where
```

hop keeps **no** database or cache — every invocation re-reads this YAML and re-checks the disk. That's what makes the dotfiles pattern work: keep `hop.yaml` in your dotfiles repo and symlink `~/.config/hop/hop.yaml` to it. Your entire repo directory then follows you between laptops, and a fresh machine is one `git clone` of your dotfiles away from a populated hop.

## 5. Keeping hop up to date

```sh
hop update
```

When hop was installed via Homebrew, `hop update` self-upgrades through brew. When it was installed from source or a release tarball, it prints a hint (re-run `just install`, or grab the latest release) and exits without invoking brew — so it never fights your install method.

## Next steps

You're set up — in a fresh shell, `hop ls` lists everything the bootstrap registered, and `h <partial>` (say, `h web`) cds you straight into a repo. Head to the [workflows deep-dive](workflows.md) for the one grammar (`hop <selection> <action>`), navigation, running commands inside any repo, and batch git ops across groups.

For the canonical command contract and config schema, see the source-of-truth specs: [`cli-surface.md`](https://github.com/sahil87/hop/blob/main/docs/specs/cli-surface.md) and [`config-resolution.md`](https://github.com/sahil87/hop/blob/main/docs/specs/config-resolution.md).
