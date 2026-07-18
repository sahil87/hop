# hop — agent skill bundle

`hop` is a repo locator: it turns one config file (default `~/.config/hop/hop.yaml`) into a
personal directory of your git repos, so you can navigate to, run commands in, and
batch-update them from anywhere. The grammar is uniformly **`hop <selection> <action>`**,
selection-first. This bundle is the usage briefing for an agent driving an installed `hop`.

## When to use

- **Reach for hop** to locate, open, or batch-operate repos already registered in `hop.yaml`:
  resolve a repo's path, list the registry, clone a registered/ad-hoc URL, or run `git pull`/`push`/`sync` across a group.
- **Not for worktree lifecycle** (create/remove worktrees) — that is [`wt`](https://github.com/sahil87/wt).
  hop only *reads* worktrees (via `wt list --json`) to resolve a `<name>/<wt>` selection.
- **Not for unregistered, ad-hoc directories** — just `cd` there. hop's substring match is over `hop.yaml` names only.

## Capabilities map

Grammar: `hop <selection> <action>`. `<selection>` = a repo name (case-insensitive substring
→ fzf on ambiguity), a `repo/worktree`, a group name, or `--all` (every cloned repo).
`<action>` = a builtin verb, a batch verb, or (shell-only) any tool.

| Invocation | Does |
|---|---|
| `hop` | fzf picker over all repos; prints the selected absolute path to stdout. |
| `hop <name> where` | Resolve `<name>`, print its absolute path to stdout (the scriptable path-resolver). |
| `hop <name>` / `hop <name> cd` | cd into the repo — **shell-only** (needs the shim; binary exits 2). |
| `hop <name> open` | Open the repo in an app — delegates to `wt`'s interactive menu. |
| `hop <name> <tool> [args]` | Run any PATH binary / alias / function with cwd = the repo — **shell-only**. |
| `hop <sel> pull` / `push` / `sync` | Batch git over `<sel>` (repo / `repo/wt` / group / `--all`). `sync` = auto-commit dirty + `pull --rebase` + `push`. |
| `hop ls [--json] [--trees]` | List the registry. `--json` emits `{name,path,url,group}` per repo; `--trees` adds worktree state. |
| `hop clone [<name>\|<url>\|--all]` | Clone a registered repo, an ad-hoc URL (auto-registers), or every missing repo. |
| `hop add <dir> [-r] [-p] [-g <group>]` | Register on-disk repos (`-r` walks a tree, `-p` previews, `-g` forces a group). |
| `hop rm [<name>] [--dry-run] [--stale]` | Remove a repo from `hop.yaml`. `--dry-run` previews without writing. |
| `hop config init\|where\|print` | Bootstrap / locate / print `hop.yaml`. |
| `hop shell-init <zsh\|bash>` | Emit the shell integration (see below). |
| `hop update` | Self-update via Homebrew. |

## Composition patterns (principle №7)

- **The shim**: `eval "$(hop shell-init zsh)"` installs a `hop()` shell function (+ `h` alias)
  that runs cd / tool-form in the **parent shell**. The binary asks itself how to dispatch
  (`hop --shim-plan …`) and the shim acts on a fixed 3-keyword reply — it hard-codes no subcommand names.
- **`wt`**: `hop <name> open` and `hop <name>/<wt>` shell out to `wt` — it must be on `PATH` for any `/`-suffixed
  selection or `open`. Bare selections never invoke `wt`.
- **`fzf`**: the picker on `hop` (bare) and ambiguous/zero-match name resolution. Invoked lazily — unique matches skip it.
- **`git`**: under the `pull`/`push`/`sync` batch verbs (one 10-minute timeout per repo call).
- **Agent-side composition**: enumerate with `hop ls --json`, then drive other verbs by exact name;
  resolve a single path with `hop <name> where`. Both are scriptable and TTY-free.

## Output & exit-code contracts

- **stdout is data**: resolved paths, the `ls` table/JSON, `config print` bytes, the version string. Nothing else.
- **stderr is diagnostics**: per-repo `pull:`/`push:`/`sync:` status lines, `skip:`/`clone:` messages, errors, hints.
- **Exit codes** a caller branches on:

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Application error (no match, missing tool, write failure) or `errSilent` (hint already on stderr). |
| `2` | Usage error — includes the shell-only forms the binary can't honor (bare cd, tool-form). |
| `3` | **No TTY** for an interactive selection — name the repo or use `hop ls --json`. Distinct from 130. |
| `130` | fzf cancelled (Esc / Ctrl-C) — not reachable from a no-TTY caller. |

## Gotchas

- **cd and tool-form are shell-only.** `hop <name>`, `hop <name> cd`, and `hop <name> <tool>` change the parent
  shell's cwd or run in it — a bare binary can't. Without the shim the binary exits **2** with a hint pointing at
  `eval "$(hop shell-init zsh)"`. In scripts/CI: resolve with `command hop <name> where` and run the tool yourself.
- **No-TTY paths exit 3, not 130.** Bare `hop`, ambiguous/zero-match names, and `hop rm`/`hop clone` with no name
  reach fzf; with no terminal they fail fast with exit **3** and an actionable hint — they do not hang and are not
  "cancelled". A **unique substring match short-circuits before the TTY guard**, so an unambiguous name resolves with no terminal.
- **Substring match is on the repo NAME only** — not URL, not path, not group. `hop web` matches `webapp`.
  Cross-group name collisions render as `name [group]` in the picker.
- **hop is stateless** — every invocation re-reads `hop.yaml` and re-checks disk (no cache, no daemon), so retries are safe.
- **Set `HOP_WRAPPER=1`** (the shim does automatically) to suppress the "install the shim" hint text on shell-only forms; the exit code stays 2.
- **`hop rm <name>` performs no confirmation prompt** — use `--dry-run` to preview (`would remove: <url>`, writes nothing, exit 0).
