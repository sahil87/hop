# CLI Surface

> Canonical contract for what the `hop` binary exposes to users.
> Source of truth for argument parsing, exit codes, stdout/stderr conventions, and help text.

## Subcommand Inventory

| Subcommand | Args | Behavior summary | Exit codes |
|---|---|---|---|
| `hop` | (none) | fzf picker over all repos; print selected absolute path on stdout | 0 selected, 130 cancelled |
| `hop <name>` | `<name>` | Binary form: print bare-name hint to stderr, exit 2 (1-arg dispatch is shell-only — shorthand for `hop <name> cd`). Shell-function form (after `eval`): `--shim-plan` classifies it as `CD\n<path>` and the shim cds there. `<name>` accepts an optional `/<wt-name>` suffix (`hop <name>/<wt>`) that resolves through `wt list --json` to a worktree path — see "Match Resolution Algorithm" below. | Binary: 2. Shell function: 0 success, 1 no match, 2 empty LHS/RHS in `/`-suffixed query |
| `hop <name> cd` | `<name> cd` | Binary form: print `cd` hint to stderr, exit 2 (cd is shell-only). Shell-function form: `--shim-plan` classifies it as `CD\n<path>` and the shim cds there. | Binary: 2. Shell function: 0 success, 1 no match |
| `hop <name> where` | `<name> where` | Resolve `<name>` and print absolute path on stdout. Replaces v0.x's top-level `hop where <name>` subcommand (removed). `<name>` accepts the `/<wt-name>` suffix — `hop <name>/<wt> where` prints the worktree's absolute path. | 0 selected, 1 no match / worktree not found / wt missing / wt list failure, 2 empty LHS/RHS, 130 cancelled |
| `hop <name> open` | `<name> open` | Resolve `<name>`, then exec `wt open <path>` (positional path arg, no chdir, stdio fully inherited). wt presents its interactive app menu directly to the user's terminal. The cd-handoff for "Open here" lives in the shim's unified `WT_CD_FILE` side-channel: the shim's `_hop_passthrough` exports `WT_CD_FILE` pointing at a temp file on the binary invocation, and cds there if wt wrote a path. The binary is a transparent passthrough — emits no stdout. `open` is classified PASSTHROUGH by `--shim-plan`. wt is a Homebrew formula dependency (`depends_on "sahil87/tap/wt"`). `<name>` accepts the `/<wt-name>` suffix — wt opens its app menu targeting the worktree's path. | wt's exit code on completion; 1 if wt is missing or resolution fails; 130 fzf cancelled |
| `hop <name> <tool> [args...]` | (shim, tool-form) | Tool-form is native grammar: any action token that is not a builtin verb (`cd`/`where`/`open`) or batch verb (`pull`/`push`/`sync`) is classified `RUN_IN_PARENT\n<path>` by `--shim-plan`. The shim cds to `<path>`, then runs the user's literal `<tool> [args...]` in the parent shell — so PATH binaries, aliases, and functions all resolve. The binary itself does NOT exec the tool (no shell-injection surface — Constitution I); invoked directly it errors with the tool-form hint. `<name>` accepts the `/<wt-name>` suffix (e.g., `hop webapp/feat-x cursor .`). | tool's exit code (in the parent shell); 2 if invoked directly on the binary; 1 if `<name>` fails to resolve |
| `hop clone [<name>] \| --all` | optional `<name>` or `--all` | Clone single (resolved) or all missing repos | 0 success, 1 path conflict, non-zero on git failure |
| `hop clone <url>` | 1 (URL form, detected by `looksLikeURL`) | Ad-hoc clone with auto-registration. Flags: `--group`, `--no-add`, `--no-cd`, `--name`. Prints the landed path on stdout AND (when `WT_CD_FILE` is set by the shim) writes it to that side-channel so the parent shell cds on success. | 0 success, 1 missing group / path conflict / git failure |
| `hop <selection> pull` | `<selection> pull` | Action token (no longer a subcommand). Wraps `git pull` over a single repo/worktree (substring `Name` match), every cloned repo in a named group (exact group match), or `--all`. Per-call 10-minute `cloneTimeout` via `proc.RunCapture`. stdout empty; per-repo `pull: <name> ✓ <last-line>` / `pull: <name> ✗ <err>` and `skip: <name> not cloned` go to stderr; batch summary `summary: pulled=N skipped=M failed=K`. `git` missing emits `gitMissingHint` once and aborts the batch. Classified PASSTHROUGH by `--shim-plan` (the binary owns the fan-out). | 0 success / batch with `failed == 0`; 1 single-repo not-cloned, single failure, batch failed > 0, git missing, fzf missing; 2 usage error; 130 fzf cancelled |
| `hop <selection> push` | `<selection> push` | Action token. Wraps `git push` over a single repo/worktree, a named group, or `--all`. Same resolution rules as `pull` — delegates to the shared `resolveTargets` resolver and `runBatch` helper. stdout empty; per-repo `push: <name> ✓ <last-line>` / `push: <name> ✗ <err>` and `skip: <name> not cloned` go to stderr; batch summary `summary: pushed=N skipped=M failed=K`. No `--force`, no `--set-upstream` — Constitution III; reach for `hop <name> git push --force` for nuanced single-repo cases. | 0 success / batch with `failed == 0`; 1 single-repo not-cloned, single failure, batch failed > 0, git missing, fzf missing; 2 usage error; 130 fzf cancelled |
| `hop <selection> sync` | `<selection> sync` | Action token. Auto-commits a dirty tree (fixed default message `chore: sync via hop` — no `-m` override in the reoriented form), then `git pull --rebase` then `git push` per target. Same resolution rules as `pull`. Two independent 10-minute timeouts per repo. Rebase `CONFLICT` emits a `resolve manually with: git -C <path> rebase --continue` hint and skips push; push failure emits `sync: <name> ✗ push failed: <err>`. Batch summary `summary: synced=N skipped=M failed=K`. | Exit codes match `pull`/`push`. |
| `hop --all <verb>` / `hop <group> <verb>` | plural selection + batch verb | Plural selection: runs the batch verb (`pull`/`push`/`sync`) across every matched repo. `hop --all pull` replaces the former `hop pull --all`. A plural selection accepts ONLY the batch verbs — `cd`, `open`, tool-form, and a bare plural (no action) are refused with exit 2 (running an interactive action across many repos is not supported). | Exit codes match the batch verb; 2 when a non-batch action or no action is given on a plural selection |
| `hop ls` | (none); `--trees` boolean flag | Default: print all repos as `name<spaces>path` columns. With `--trees`: fan `wt list --json` across configured cloned repos in YAML source order and emit per-row worktree summaries (`name<spaces>{N} tree(s)  (<wt-list>)` where each wt is `name[*][↑N]`). Non-cloned repos surface `(not cloned)` without invoking wt. Per-row `wt list` failures degrade as inline `(wt list failed: <err>)`; first `wt`-missing aborts the run with `hop: wt: not found on PATH.` | 0 success; 1 wt missing during `--trees` |
| `hop rm [<name>]` | optional `<name>`; `--stale` boolean flag; `--dry-run` boolean flag | Remove a registered repo from `hop.yaml`. No positional → fzf picker (single-select) over registered repos; a `<name>` resolves via the shared match-or-fzf algorithm and removes directly (no picker, no on-disk check, no prompt). `--stale` pre-filters the picker to repos whose resolved path is missing from disk (cannot be combined with `<name>`). **`--dry-run`** resolves the target through the same path as a live removal but **writes nothing** — it previews `would remove: <url>` + `dry-run: no changes written` to stderr (or the forgiving `Nothing to remove.` when the URL/group is absent) and exits 0, leaving `hop.yaml` byte-for-byte unchanged (principle №5: a destructive write's `--dry-run` shares the real code path via `yamled.WouldRemoveURL`, the read-only half of `yamled.RemoveURL`). Status lines (`removed:`/`wrote:` on a live run, `would remove:`/`dry-run:` on a preview) go to stderr. `hop config rm [--stale] [--dry-run]` is a hidden picker-only alias. | 0 success / forgiving no-op (nothing to remove, nothing stale, not-found, successful dry-run preview incl. the forgiving not-found); 1 fzf missing / missing `hop.yaml` / write failure / dry-run preview failure (unreadable `hop.yaml`); 2 `--stale` combined with a name; 3 no TTY for the picker (live or dry-run); 130 fzf cancelled |
| `hop shell-init <shell>` | `zsh` or `bash` (required) | Emit shell function wrapper + cobra-generated completion to stdout | 0 success, 2 unsupported shell |
| `hop config init` | (none) | Bootstrap a starter `hop.yaml` at the resolved location | 0 written, 1 file exists, 2 write error |
| `hop config where` | (none) | Print the resolved config path on stdout. Renamed from v0.0.1's `config path`. | 0 resolved, 1 unresolvable |
| `hop config print` | (none) | Print the resolved `hop.yaml` contents to stdout (raw bytes, comment-preserving). | 0 success, 1 unresolvable / read error |
| `hop config scan <dir>` | exactly 1 (directory) | Walk `<dir>` (default `--depth 3`), discover git repos via stat + `git remote`, and emit a merged `hop.yaml` to stdout (default) or merge in place via `--write` (atomic, comment-preserving). Auto-derives groups: convention-match repos go to `default`; non-convention repos land in invented map-shaped groups keyed off the parent dir basename. | 0 success (incl. zero repos found); 1 missing `hop.yaml` / git missing / write failure; 2 usage error (missing arg, dir validation, `--depth < 1`) |
| `hop update` | (none) | Self-update the `hop` binary via Homebrew. No-op (with hint) when the binary was not installed via brew. | 0 success, 1 brew failure |
| `hop -h \| --help \| help` | (none) | Print help text on stdout | 0 |
| `hop -v \| --version` | (none) | Print version string on stdout | 0 |

> `hop path` (v0.0.1) and `hop config path` (v0.0.1) were removed without aliases. The top-level `hop where <name>` and `hop cd <name>` subcommands were removed in the v0.x repo-verb grammar flip — use `hop <name> where` and `hop <name> cd` (or the bare `hop <name>` shorthand) instead. `hop config where` survives unchanged (different namespace).
>
> **Grammar + shim refactor (gyo0)**: the grammar is now uniformly `hop <selection> <action>`. The `-R` flag (and its `extractDashR`/`runDashR` argv inspection) and the `hi` alias were removed; tool-form is native grammar dispatched via the shim's `RUN_IN_PARENT` plan. `pull`/`push`/`sync` are no longer cobra subcommands — they are action tokens after a selection (`hop <name> pull`), with plural fan-out via `hop --all pull` / `hop <group> pull` (replacing the former `hop pull --all`). Subcommand classification moved out of the shim into the binary's hidden `--shim-plan` flag, which emits a fixed 3-keyword protocol (`CD` / `RUN_IN_PARENT` / `PASSTHROUGH`); the shim hard-codes zero subcommand names.

### Match Resolution Algorithm

Used by `hop` (the `--shim-plan` `CD`/`RUN_IN_PARENT` path), `hop <name> where`, `hop <name> cd`, the batch verbs, and `hop clone`.

0. **Worktree-suffix pre-step**: if `<name>` contains a `/`, split on the **first** `/` (repo names from `hop.yaml` are URL basenames and never contain `/`, so first-split is unambiguous even when wt worktree names themselves contain `/`). Empty LHS → exit 2 with `hop: empty repo name before '/'`; empty RHS → exit 2 with `hop: empty worktree name after '/'`. Otherwise, recurse on the LHS (steps 1-5 below) to resolve a repo, then run the worktree-resolution sub-step (described below).
1. Build the list of all known repos from `hop.yaml`. Each entry has `(Name, Group, Dir, URL, Path)`. The list preserves YAML source order (groups in `cfg.Groups` order, URLs within each group in source order).
2. If `<name>` is non-empty: filter by case-insensitive substring match on `Name` (not Path, not URL, not Group).
3. If exactly **1 match**: return it directly without invoking fzf.
4. Otherwise (0 matches OR 2+ matches): invoke fzf with these flags, piping the **full repo list** (not the filtered subset) on stdin so the user can clear the query inside fzf to browse all repos:
   ```
   fzf --query <name> --select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'
   ```
   The `--select-1` flag makes fzf auto-select if its filter narrows to exactly 1.
5. If `<name>` is empty: invoke fzf without `--query` (full picker).

#### Worktree-resolution sub-step

When the pre-step splits on `/`, after the LHS resolves to a `*repos.Repo`:

1. **Cloned-state guard**: if the resolved repo's `.git` does NOT exist on disk, exit 1 with `hop: '<name>' is not cloned. Try: hop clone <name>` BEFORE invoking wt. This guard applies ONLY to `/`-suffixed queries; bare queries retain their existing permissive behavior of resolving registry paths even when the repo isn't cloned.
2. Invoke `wt list --json` in the repo's main checkout via `internal/proc.RunCapture` with a 5-second per-call timeout. Parse into `[]WtEntry` where each entry has `{Name, Branch, Path, IsMain, IsCurrent, Dirty, Unpushed}` (unknown JSON fields are silently ignored for forward-compat).
3. Find the entry whose `Name` equals the RHS exactly (case-sensitive — mirrors the case-sensitive group-name match in `resolveTargets`). `hop <name>/main` naturally resolves to the main checkout because wt's `is_main: true` entry carries that path; no special-case in hop.
4. Return a shallow copy of the LHS-resolved repo with `Path` replaced by the worktree's absolute path; all other fields (`Name`, `Group`, `URL`, `Dir`) preserved.

Error surfaces (all exit code 1 unless noted; pre-formatted stderr lines):

- `wt` missing on PATH → `hop: wt: not found on PATH.` (same wording as `hop <name> open`)
- `wt list --json` non-zero exit or malformed JSON → `hop: wt list: <err>` (no silent fallback to the main path — unparseable wt output is a real failure)
- No matching worktree → `hop: worktree '<wt>' not found in '<repo>'. Try: wt list (in <repo-path>) or hop ls --trees`

#### Group disambiguation in the picker

When two or more repos share the same `Name` across different groups, the displayed first column is `<name> [<group>]` rather than just `<name>`. When a name is unique across groups, no suffix is added. Two URLs in the *same* group whose derived `Name` collides still render an identical first column (intra-group collisions are out of scope; cross-group collisions are handled).

### Stdout / stderr Conventions

- **stdout**: resolved absolute paths (`hop` bare picker, `hop <name> where`), the `hop ls` table, version string, config path (`hop config where`), shell integration (`hop shell-init <shell>`), help text, "Created <path>" message from `hop config init`, the landed path from `hop clone <url>` (also mirrored to `WT_CD_FILE` for cd-on-success). The `--shim-plan` classifier emits exactly one plan line-group on stdout: `CD\n<path>`, `RUN_IN_PARENT\n<path>`, or `PASSTHROUGH`. Tool-form runs in the parent shell (the shim's `RUN_IN_PARENT` arm), so its output is the tool's own — not hop-owned.
- **stderr**: status messages (`clone: <url> → <path>`, `skip: <reason>`), error messages, hints. The `hop config init` post-write tip also goes to stderr. The `--shim-plan` usage errors (plural-selection guard) go to stderr with exit 2.
- The `hop <name> cd`, bare `hop <name>`, and tool-form (`hop <name> <tool>`) binary-form exit-2 hints go to **stderr**.

### Behavioral Scenarios (GIVEN/WHEN/THEN)

#### Bare picker

> **GIVEN** `hop.yaml` lists 3 repos
> **WHEN** I run `hop` with no arguments
> **THEN** fzf opens with all 3 repos visible
> **AND** selecting one prints its absolute path to stdout
> **AND** exit code is 0

#### Bare-name 1-arg form (binary)

> **GIVEN** the user invokes the binary directly (no shim)
> **WHEN** they run `hop webapp`
> **THEN** the binary writes the bare-name hint (`hop: bare-name dispatch is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop "<name>" where`) to stderr
> **AND** stdout is empty
> **AND** exit code is 2

#### Unique substring match (`hop <name> where`)

> **GIVEN** `hop.yaml` has exactly one repo named `webapp`
> **WHEN** I run `hop webapp where`
> **THEN** fzf is NOT invoked
> **AND** stdout is the absolute path to that repo
> **AND** exit code is 0

#### Ambiguous substring match (`hop <name> where`)

> **GIVEN** `hop.yaml` has repos `webapp` and `webapp-shared`
> **WHEN** I run `hop webapp where`
> **THEN** fzf opens with both candidates filtered (`--query webapp`)
> **AND** if the user picks one, exit code 0
> **AND** if the user cancels (Esc), exit code 130

#### Zero substring match (`hop <name> where`)

> **GIVEN** `hop.yaml` has repos `alpha`, `beta`, `gamma`
> **WHEN** I run `hop zzz where`
> **THEN** fzf opens with `--query zzz` and zero filtered candidates
> **AND** the user can clear the query inside fzf to see all repos and pick one
> **AND** if the user cancels, exit code 130

#### Group disambiguation in picker

> **GIVEN** `hop.yaml` has a repo named `tools` in group `default` and another named `tools` in group `vendor`
> **WHEN** I run `hop` (bare)
> **THEN** fzf shows two rows: `tools [default]` and `tools [vendor]`
> **AND** the path column (the unique key for match-back) distinguishes them

#### `hop <name> cd` binary form

> **GIVEN** the user has NOT run `eval "$(hop shell-init zsh)"`
> **WHEN** they run `hop <name> cd`
> **THEN** the binary prints to stderr: `hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" where)"`
> **AND** exit code is 2

#### `hop <name> cd` shell-function form

> **GIVEN** the user has run `eval "$(hop shell-init zsh)"`
> **WHEN** they run `hop <name> cd`
> **THEN** `--shim-plan` classifies it as `CD\n<resolved-path>`
> **AND** the shim runs `cd -- <resolved-path>`
> **AND** the parent shell's working directory is changed

#### `hop <name> <tool>` binary form (tool-form attempt)

> **GIVEN** the user invokes the binary directly (no shim)
> **WHEN** they run `hop webapp cursor`
> **THEN** the binary prints the tool-form hint to stderr: `hop: 'cursor' is not a hop verb (cd, where, open, pull, push, sync). Tool-form runs in your shell — install the shim: eval "$(hop shell-init zsh)"`
> **AND** exit code is 2 (tool-form runs in the parent shell via the shim, so the binary cannot honor it)

#### The `--shim-plan` protocol

The shim is a logic-free interpreter of a fixed 3-keyword protocol emitted by the binary's hidden `--shim-plan` flag. The shim hard-codes zero subcommand names — the list lives only in cobra, so shim/binary name-drift is structurally impossible.

`hop()` calls `plan="$(command hop --shim-plan "$@")" || return $?` and branches the first line over:

- `CD\n<path>` → `cd -- <path>` (bare `hop <name>`, `hop <name> cd`).
- `RUN_IN_PARENT\n<path>` → `cd -- <path>; shift; "$@"` — runs the user's already-parsed words in the parent shell (tool-form: PATH binaries, aliases, functions all resolve). **Security (Constitution I)**: `"$@"` are the user's typed words, never an `eval` of binary output; `<path>` is used only as a quoted `cd` operand — no shell-injection surface.
- `PASSTHROUGH` → `_hop_passthrough "$@"` → `command hop "$@"`. The binary owns it: `add`, `rm`, `clone`, `ls`, `config`, `update`, `shell-init`, `where`, `open`, `pull`/`push`/`sync`, `--help`/`-h`/`--version`/`completion`/`__complete*`. `_hop_passthrough` provides the unified `WT_CD_FILE` cd side-channel (collapsing the former where/open/clone handoffs into one).

`__complete*` is forwarded directly to the binary (NOT through `--shim-plan`, which would classify the completion request instead of answering it). `h <name>` (single-letter alias) behaves identically. The `hi` alias and the `-R` flag were removed; `command hop` is the raw escape hatch.

Classification (first match wins):

1. No args → `CD` (bare picker resolves a path).
2. `$1` is `__complete*` → `PASSTHROUGH` (defense-in-depth).
3. `$1` is a known cobra subcommand, or a flag other than `--all` → `PASSTHROUGH`.
4. `$1 == --all` → plural selection (action = `$2..`).
5. `$1` is an exact configured group name → plural selection (action = `$2..`).
6. Otherwise `$1` is a singular repo/worktree selection (action = `$2..`):
   - no action / `cd` → `CD\n<path>`.
   - `where` / `open` / `pull` / `push` / `sync` → `PASSTHROUGH` (the binary owns them).
   - any other token → `RUN_IN_PARENT\n<path>` (tool-form).

> **GIVEN** `hop.yaml` resolves `webapp` to `~/code/sahil87/webapp`, shim installed
> **WHEN** I run `hop webapp git status`
> **THEN** `--shim-plan` emits `RUN_IN_PARENT\n~/code/sahil87/webapp`
> **AND** the shim cds there, shifts off `webapp`, and runs `git status` in the parent shell
> **AND** the parent shell's cwd is now `~/code/sahil87/webapp`

> **GIVEN** an arbitrary tool with its own flags
> **WHEN** I run `hop webapp jq '.foo' file.json`
> **THEN** the shim runs the user's literal words `jq '.foo' file.json` (no re-parsing) in the parent shell

> **GIVEN** `<name>` matches no repo
> **WHEN** I run `hop nope echo hi`
> **THEN** `--shim-plan` fails resolution, prints the match-or-fzf no-candidate stderr, and the shim's `|| return $?` propagates exit 1

> **GIVEN** `cursor` is a shell alias/function and `dotfiles` resolves uniquely
> **WHEN** I run `hop dotfiles cursor .`
> **THEN** the shim cds into dotfiles and runs `cursor .` in the parent shell — so aliases/functions resolve (the former `-R` path could not run shell functions)

> **GIVEN** the user invokes the binary directly without the shim
> **WHEN** they run `/usr/local/bin/hop --shim-plan webapp git status`
> **THEN** the binary emits `RUN_IN_PARENT\n~/code/sahil87/webapp` and exits 0 (classification only — it never execs the tool)

#### Plural selection and the interactive guard

> **GIVEN** shim installed
> **WHEN** I run `hop --all pull`
> **THEN** `--shim-plan` emits `PASSTHROUGH` and `command hop --all pull` runs `git pull` across every cloned repo (replacing the former `hop pull --all`)

> **WHEN** I run `hop <group> sync`
> **THEN** sync runs across every cloned repo in `<group>`

> **WHEN** I run `hop --all code .` (interactive action on a plural selection)
> **THEN** `--shim-plan` prints a usage error to stderr and exits 2 — running an interactive action across many repos is refused

> **WHEN** I run `hop --all` (plural selection, no action)
> **THEN** exit 2 with a usage error — a plural selection has no single directory to cd into

#### `hop clone <name>` (registered repo)

> **GIVEN** `<name>` resolves to `(name=foo, path=~/code/foo, url=git@github.com:user/foo.git)` and `~/code/foo` does not exist
> **WHEN** I run `hop clone foo`
> **THEN** stderr shows `clone: git@github.com:user/foo.git → ~/code/foo`
> **AND** `git clone git@github.com:user/foo.git ~/code/foo` runs (10-minute timeout)
> **AND** exit code matches git's exit code

> **GIVEN** the same resolution, but `~/code/foo/.git` already exists
> **WHEN** I run `hop clone foo`
> **THEN** stderr shows `skip: already cloned at ~/code/foo`
> **AND** exit code is 0

> **GIVEN** the same resolution, but `~/code/foo` exists and is NOT a git repo
> **WHEN** I run `hop clone foo`
> **THEN** stderr shows `hop clone: ~/code/foo exists but is not a git repo`
> **AND** exit code is 1

#### `hop clone --all`

> **GIVEN** `hop.yaml` has 5 repos, 2 already cloned
> **WHEN** I run `hop clone --all`
> **THEN** stderr shows `clone:` lines for the 3 missing and `skip:` lines for the 2 cloned
> **AND** the final stderr line is `summary: cloned=3 skipped=2 failed=0`
> **AND** exit code is 0 if `failed == 0`, else non-zero

#### `hop clone <url>` — ad-hoc URL clone with auto-registration

`hop clone` distinguishes URL form from name form via `looksLikeURL`: the argument contains `://` OR (`@` AND `:`). On URL form:

1. Resolve the target group (`--group <name>`, default `default`). Missing group → exit 1 with `hop: no '<group>' group in <config-path>. ...`.
2. Compute landing path:
   - Map-shaped group with `dir:` set: `<dir>/<name>`.
   - Flat group: `<code_root>/<org-from-url>/<name-from-url>` (the `org` segment is dropped if the URL has none).
   - `--name <override>` replaces the URL-derived name.
3. Classify on-disk state and act:
   - **Missing path** → `git clone <url> <path>`, then (unless `--no-add`) append URL to `hop.yaml` via `internal/yamled.AppendURL`. Print landed path to stdout (unless `--no-cd`).
   - **Already cloned** (`<path>/.git` exists) → emit `skip: already cloned at <path>` to stderr; still appends YAML and prints path (registers an existing checkout).
   - **Path exists, not a git repo** → emit `hop clone: <path> exists but is not a git repo`; exit 1; no YAML write, no stdout.
4. URL already in target group's `urls` list → emit `skip: <url> already registered in '<group>'` to stderr; no YAML write; still print path (unless `--no-cd`) so the shim can `cd` to it.

The YAML write is **comment-preserving and atomic** (temp file + rename via `internal/yamled`); see [architecture.md](architecture.md#internalyamled).

> **GIVEN** `hop.yaml` has a `default` flat group, `code_root = ~/code`, and `~/code/sahil87/loom` does not exist
> **WHEN** I run `hop clone git@github.com:sahil87/loom.git`
> **THEN** `git clone` runs into `~/code/sahil87/loom`
> **AND** the URL is appended to the `default` group in `hop.yaml` (comments preserved, atomic write)
> **AND** stdout is `~/code/sahil87/loom` (consumed by the shim's `cd`)
> **AND** exit code is 0

> **GIVEN** the same setup, plus `--group vendor` and a map-shaped `vendor: { dir: ~/vendor, urls: [...] }` group
> **WHEN** I run `hop clone --group vendor git@github.com:other/tool.git`
> **THEN** the landing path is `~/vendor/tool`
> **AND** the URL is appended to `vendor.urls` in `hop.yaml`

> **GIVEN** `--no-add` is passed
> **WHEN** I run `hop clone --no-add <url>`
> **THEN** the clone proceeds but `hop.yaml` is NOT modified

> **GIVEN** `--no-cd` is passed
> **WHEN** I run `hop clone --no-cd <url>` (under the shim or not)
> **THEN** stdout suppresses the landed path, so the shim does not `cd`

> **GIVEN** `--name foo`
> **WHEN** I run `hop clone --name foo git@github.com:user/bar.git`
> **THEN** the landing path uses `foo`, not the URL-derived `bar`

#### `hop ls`

> **GIVEN** `hop.yaml` has 3 repos across 2 groups (preserving source order: group A then group B)
> **WHEN** I run `hop ls`
> **THEN** stdout shows 3 rows in YAML source order, each `name<spaces>path`, aligned (column-style)
> **AND** exit code is 0
> **AND** an empty `hop.yaml` produces no output (still exit 0)

#### `hop rm <name> --dry-run` — preview a removal without writing

> **GIVEN** `hop.yaml` registers `hop` and `wt` in the `default` group
> **WHEN** I run `hop rm wt --dry-run`
> **THEN** `wt` is resolved via the same match-or-fzf path a live `hop rm wt` uses
> **AND** stderr shows `would remove: git@github.com:sahil87/wt.git` followed by `dry-run: no changes written`
> **AND** no `removed:` / `wrote:` line is emitted
> **AND** `hop.yaml` is byte-for-byte unchanged on disk
> **AND** exit code is 0

> **GIVEN** the same `hop.yaml`
> **WHEN** I run `hop rm --dry-run` (no name — the picker path) and select `wt`
> **THEN** the picked entry is previewed the same way (`would remove:` + `dry-run:`), `hop.yaml` is untouched, exit 0

> **GIVEN** a resolved repo whose URL is not present in its group
> **WHEN** the dry-run runs
> **THEN** stderr shows the forgiving `... not found in <path>. Nothing to remove.` (same wording as the live path's not-found no-op) and exit code is 0 — the preview shares `yamled.RemoveURL`'s locate contract via `yamled.WouldRemoveURL`

#### `hop shell-init <shell>`

> **WHEN** I run `hop shell-init zsh`
> **THEN** stdout contains the shared `posixInit` prefix defining `hop()`, `_hop_passthrough()`, `h()` (a logic-free interpreter of the `--shim-plan` protocol — no `_hop_dispatch`, no `hi`)
> **AND** stdout contains the cobra-generated `_hop` completion function (appended at runtime via `rootCmd.GenZshCompletion`)
> **AND** stdout contains `compdef _hop h` so the `h` alias shares the completion
> **AND** running `eval "$(hop shell-init zsh)"` in a zsh shell defines `hop` as a function (verifiable via `whence -w hop`)
> **AND** exit code is 0

> **WHEN** I run `hop shell-init bash`
> **THEN** stdout contains the same shared `posixInit` prefix (works in both shells — uses `[[ ]]`, `${@:N}`, `local`)
> **AND** stdout contains the cobra-generated `__start_hop` bash completion function (via `rootCmd.GenBashCompletionV2`)
> **AND** stdout contains `complete -o default -F __start_hop h hi` so the aliases share the completion
> **AND** exit code is 0

> **WHEN** I run `hop shell-init` with no shell argument
> **THEN** stderr shows `hop shell-init: missing shell. Supported: zsh, bash`
> **AND** exit code is 2

> **WHEN** I run `hop shell-init fish`
> **THEN** stderr shows `hop shell-init: unsupported shell 'fish'. Supported: zsh, bash`
> **AND** exit code is 2

#### `hop --version` / `-v`

> **WHEN** I run `hop --version` or `hop -v`
> **THEN** stdout is a single line containing the version string (e.g., `v0.1.0` or `v0.1.0-2-gabc123` for dev builds from `git describe`)
> **AND** exit code is 0

> **NOTE**: Cobra also auto-wires a `hop version` subcommand from `rootCmd.Version`; this still works (no effort spent suppressing it).

#### `hop update`

`hop update` self-upgrades the binary via Homebrew. It MUST detect whether the binary was installed via brew (by walking `os.Executable` through `EvalSymlinks` and checking for `/Cellar/` in the resolved path); when it wasn't, it MUST exit 0 after printing a hint pointing at the manual install command — the binary cannot upgrade what it didn't install.

The brew formula is referenced as `sahil87/tap/hop` (fully qualified) to disambiguate from the Homebrew core `hop` cask (an HWP document viewer) that would otherwise shadow the formula.

Version comparison MUST normalize the leading `v` — the binary reports versions with the `v` prefix (e.g. `v0.0.3` from the build's `git describe` ldflag), while `brew info --json=v2` reports the bare form (`0.0.3`). The comparison uses the bare form on both sides.

> **GIVEN** the binary was installed via Homebrew and the tap formula is at the same version
> **WHEN** I run `hop update`
> **THEN** stdout shows `Current version: v<X>`, then `Checking for updates...`, then `Already up to date (v<X>).`
> **AND** exit code is 0
> **AND** `brew upgrade` is NOT invoked

> **GIVEN** the binary was installed via Homebrew and the tap has a newer version
> **WHEN** I run `hop update`
> **THEN** stdout shows `Updating v<old> → v<new>...` followed by `brew upgrade` output
> **AND** on success, stdout ends with `Updated to v<new>.`
> **AND** exit code is 0

> **GIVEN** the binary was NOT installed via Homebrew (e.g. `just local-install`, manual `go install`, or downloaded tarball)
> **WHEN** I run `hop update`
> **THEN** stdout shows `hop v<X> was not installed via Homebrew.` followed by a manual-update hint pointing at `brew install sahil87/tap/hop`
> **AND** `brew` is NOT invoked
> **AND** exit code is 0

> **GIVEN** `brew update` or `brew info` fails (network error, brew not on PATH, etc.)
> **WHEN** I run `hop update`
> **THEN** stderr shows the failure reason
> **AND** exit code is 1

#### `hop config scan <dir>` — populate `hop.yaml` from on-disk repos

`hop config scan` walks `<dir>` (default `--depth 3`, inclusive), discovers git repositories via stat + `git remote`, derives groups from the on-disk layout (convention-match → `default`; non-convention → invented map-shaped group keyed off the parent dir basename), and emits a merged `hop.yaml` to stdout (default) or merges in place via `--write` (atomic, comment-preserving). All `git` invocations route through `internal/proc.RunCapture` with a 5-second per-call `context.WithTimeout`. Walk symlinks are followed with `(dev, inode)` loop dedup. Implementation: `src/cmd/hop/config.go::newConfigScanCmd` + helpers in `src/cmd/hop/config_scan.go`; the walker lives in `src/internal/scan/` and the YAML merge in `src/internal/yamled/MergeScan` + `RenderScan`.

> **GIVEN** `hop.yaml` has `code_root: ~/code` and `~/code/sahil87/hop/.git` exists with `git remote get-url origin` returning `git@github.com:sahil87/hop.git`
> **WHEN** I run `hop config scan ~/code`
> **THEN** the URL lands in the `default` flat group in the rendered YAML
> **AND** stderr summarizes `matched convention (default): 1`
> **AND** exit code is 0

> **GIVEN** the same `hop.yaml` and a non-convention repo at `~/vendor/forks/tool/.git` with URL `git@github.com:other/tool.git`
> **WHEN** I run `hop config scan ~/vendor`
> **THEN** the rendered YAML contains an invented `forks:` group with `dir: ~/vendor/forks` and the URL under `urls:`
> **AND** stderr summarizes `invented groups: 1 (forks)`

> **GIVEN** `~/work` is a symlink to `~/Volumes/Mac/work` (a real directory containing repos)
> **WHEN** I run `hop config scan ~/work`
> **THEN** `EvalSymlinks` resolves the argument and the walk proceeds against the canonical target
> **AND** each `Found.Path` is the canonical (resolved) path

> **GIVEN** `~/code/a/b/c/d/.git` exists at depth 4 from `~/code`
> **WHEN** I run `hop config scan ~/code --depth 3`
> **THEN** that repo is NOT in the rendered YAML (depth bound is inclusive at 3)

> **GIVEN** `~/code/scratch/.git` exists and `git remote` returns empty
> **WHEN** I run `hop config scan ~/code`
> **THEN** the repo is skipped with reason `no remote`
> **AND** stderr's skipped breakdown counts it
> **AND** the URL is NOT rendered into the YAML

> **GIVEN** no `hop.yaml` exists at the resolved path (and `$HOP_CONFIG` is unset)
> **WHEN** I run `hop config scan ~/code`
> **THEN** stderr shows `hop config scan: no hop.yaml found at <ResolveWriteTarget>.` followed by `Run 'hop config init' first, then re-run scan.`
> **AND** exit code is 1
> **AND** no walk is performed (no `git` invocations)

### External Tool Availability

External tools (`fzf`, `git`, `<cmd>` for `-R`) are checked **lazily** — only when the subcommand actually needs them. Subcommands that resolve without an external tool MUST NOT preemptively check or fail.

| Tool | Required by | Behavior if missing |
|---|---|---|
| `fzf` | `hop` (bare picker), `hop <name> where` (ambiguous), `hop <name> <tool>` (ambiguous selection), `hop clone <name>` (ambiguous) | Print to stderr: `hop: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).` Exit 1. |
| `git` | `hop clone` (any form); `hop <selection> pull`/`push`/`sync`; `hop config scan <dir>` (only when the walk finds a `.git` candidate — empty trees succeed without `git`) | Print to stderr: `hop: git is not installed.` Exit 1. |
| `<tool>` | `hop <name> <tool>` (tool-form, run in the parent shell via the shim's `RUN_IN_PARENT` plan) | The shim runs the user's literal `<tool>` words; a missing tool surfaces via the shell's own "command not found" (not a hop error). The binary never execs the tool. |
| `wt` | `hop <name> open` (any form); `hop <name>/<wt> ...` (any `/`-suffixed form, via `resolveByName`); `hop ls --trees` (lazy — first invocation only) | Print to stderr: `hop: wt: not found on PATH.` Exit 1. Mitigated: wt is declared as a Homebrew formula dependency (`depends_on "sahil87/tap/wt"`). |
| `brew` | `hop update` (when installed via brew) | Print to stderr: `hop update: brew not found on PATH.` Exit 1. |

Subcommands that don't need a tool MUST work without it. Examples:
- `hop foo where` (when `foo` is a unique substring match) does not invoke fzf — works without `fzf` installed.
- `hop ls` does not invoke any external tool.
- `hop shell-init zsh` and `hop shell-init bash` do not invoke any external tool — emit stdout text only.
- `hop config init` and `hop config where` do not invoke any external tool.

### Help Text

`hop -h | --help | help` emits help text to stdout. Cobra renders the help; the `Usage:` table and `Notes:` block come from `rootLong` in `src/cmd/hop/root.go`. Top-level structure mirrors the inventory table above.

The `Usage:` block enumerates (in this order): `hop`, `hop <name>`, `hop <name>/<wt>`, `hop <name> cd`, `hop <name> where`, `hop <name> open`, `hop <name> git pull`, `hop <name> code .`, `hop <name> p`, `hop <name> pull`, `hop <name> push`, `hop <name> sync`, `hop <group> pull`, `hop --all pull`, `hop --all sync`, `hop clone <name>`, `hop clone <url>`, `hop clone --all`, `hop clone`, `hop ls`, `hop add <dir>`, `hop rm [<name>]`, `hop shell-init <shell>`, `hop config init`, `hop config where`, `hop config print`, `hop config scan <dir>`, `hop update`, `hop -h | --help`, `hop -v | --version`.

The `Notes:` block in `rootLong` documents:
- `hop <name>` and `hop <name> cd` require shell integration (a binary can't change its parent shell's cwd). Without it, use `cd "$(hop <name> where)"`.
- Tool-form (`hop <name> <tool> ...`) and `hop <name> open`'s "Open here" choice run in the parent shell via the shim.
- `pull`/`push`/`sync` accept a repo, a worktree, a group, or `--all` as the selection. A plural selection (`--all` or a group) accepts only `pull`/`push`/`sync`.
- On ambiguous or no-match queries, fzf opens prefilled with the user's query.
- Config lives at `~/.config/hop/hop.yaml`.

### Cobra Wiring

- `rootCmd` is defined in `src/cmd/hop/root.go::newRootCmd()`.
- Each subcommand has its own file under `src/cmd/hop/` with a `func newXxxCmd() *cobra.Command` factory.
- `main.go::main()`:
  1. Builds `rootCmd := newRootCmd()`.
  2. Sets `rootCmd.Version = version` (the package-level `var version = "dev"`, overridden via `-ldflags "-X main.version=…"` at build time).
  3. Captures `rootForCompletion = rootCmd` so `shell-init` can call `GenZshCompletion` / `GenBashCompletionV2` without threading `rootCmd` through factories.
  4. Inspects `os.Args` for `--shim-plan` via `extractShimPlan` (pre-cobra). If present, calls `runShimPlan(os.Stdout, os.Stderr, rest)` and `os.Exit(code)` — bypassing cobra entirely (the action token after the selection is an arbitrary child command line, not a hop flag).
  5. Otherwise calls `rootCmd.Execute()`. Errors are mapped to exit codes via `translateExit`.
- `rootCmd.SilenceUsage = true` and `rootCmd.SilenceErrors = true` — `translateExit` is the sole stderr/exit path.
- The selection-first behavior (`hop` with no args, `hop <name>` 1-arg, `hop <selection> <action>` 2+-arg) is implemented via `rootCmd.RunE` (with `cobra.ArbitraryArgs`) in `runRoot`. The `where`-verb branch dispatches to `resolveAndPrint`; `open` to `runOpen`; `pull`/`push`/`sync` to `runBatchVerb`; the bare-name, `cd`-verb, and tool-form branches each return `&errExitCode{code: 2, msg: ...}` with the appropriate hint. Plural selection (`--all` or an exact group) dispatches to `runPluralSelection`.

#### Why `--shim-plan` bypasses cobra

The action token after the selection (e.g. `git pull`, `code .`, `jq '.foo' file.json`) is an arbitrary child command line, not a hop flag or subcommand. Cobra's parser would try to interpret it as hop's own flags/subcommand. Pre-Execute argv inspection (`extractShimPlan`) strips the `--shim-plan` flag and hands the user's original argv to `runShimPlan`, which classifies it and emits the fixed 3-keyword protocol without cobra parsing the action. Tested in `shim_plan_test.go`.

### Exit Code Conventions

Defined centrally in `main.go::translateExit`:

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Application error (no match, missing tool, file already exists, write error, child resolution error, etc.); also `errSilent` (caller already wrote stderr) |
| 2 | Usage error (`cd` binary form, tool-form binary form, `shell-init` missing/unsupported shell, plural-selection guard) |
| 130 | User cancelled — fzf Esc / Ctrl-C (`errFzfCancelled`) |

The `--shim-plan` classifier bypasses cobra entirely and uses `os.Exit` directly with the classification exit code (0 for a successful plan, 2 for the plural-selection guard, 1 for resolution errors, 130 for fzf cancellation).

### Design Decisions

1. **The `cd` verb at $2 is shell-only; the binary errors with a hint.** A binary cannot change its parent shell's `cwd`; the function wrapper (emitted by `hop shell-init zsh`) does. The binary's role is to print a hint pointing at the shim install and `hop <name> where`, so users discover the shell integration. Generalizes to: every form that needs the shim errors in the binary; every form the binary can fulfill works in both layers.
2. **Bare-name dispatch (`hop <name>` 1 arg) is shorthand for `hop <name> cd` (Option B2).** Both are shell-only — the binary errors with a hint. This enforces the invariant that any `hop <subform>` either errors in the binary or works in both layers — never two different effects sharing one syntax. The pre-flip behavior (binary printed the path; shim cd'd) was the asymmetry this change eliminates.
3. **`fzf` is invoked lazily, not preflighted.** Subcommands that don't need fzf (`hop ls`, `hop shell-init zsh`, `hop config *`, exact-match resolutions) work without it installed. This matters for minimal environments and CI.
4. **`--shim-plan` bypasses cobra rather than using `cobra.Command{DisableFlagParsing: true}`.** Pre-Execute argv inspection is a single small function (`extractShimPlan`); the alternative would entangle every flag-parsing path with action-token-aware logic. Unit tests cover the classification without spawning the binary (`shim_plan_test.go`). This replaced the former `-R` flag + `extractDashR` argv split (gyo0): tool-form is now native grammar dispatched via the shim's `RUN_IN_PARENT` plan, not a binary `-R` exec.
5. **Match algorithm is substring-on-`Name` only.** Not Path, not URL, not Group. Simple, predictable, matches the bash original. Group disambiguation is a display-time concern only (`buildPickerLines` adds `[<group>]` suffix when names collide across groups).
6. **The `where` verb is the explicit path-printer.** Used as `hop <name> where` (top-level repo-verb form) and `hop config where` (config namespace). The top-level `where` *subcommand* (v0.x's `hop where <name>`) was removed — `hop <name> where` is the replacement; the verb survives, the subcommand position does not. Both answer "where does this resolve to?" The v0.0.1 names (`path`, `config path`) lacked voice-fit with the new binary name and were renamed without aliases (no migration path; the rename was a clean break for v0.x).
7. **`hop clone <url>` infers form from argument shape.** `looksLikeURL` (contains `://` OR (`@` AND `:`)) splits URL form from name form. This keeps `clone` to one verb rather than `clone-url` / `clone-name`. URLs of registered repos still go through name form via `hop clone <name>` — there's no ambiguity because the URL form requires an actual URL shape.
8. **Auto-registration on `hop clone <url>` is opt-out, not opt-in.** The default behavior for an ad-hoc URL clone is "I want this in my registry"; `--no-add` is the escape valve. This matches the dominant use case (try a new repo → keep it). The YAML write is comment-preserving (via `internal/yamled`) so registration doesn't trash hand-curated comments.
9. **`hop update` is a top-level subcommand, not `hop config update` or a flag.** Per Constitution Principle VI, new top-level subcommands need explicit justification. Self-update is a binary-state operation, not config-state — it doesn't fit under `config`, and overloading a flag on the root (e.g. `hop --update`) muddles the bare-form's "print path" semantics. It also matches the convention every Homebrew-installed CLI uses (`fab-kit update`, `gh extension upgrade`). The implementation lives in `internal/update` and routes all subprocess invocations through `internal/proc` per Constitution Principle I (no direct `os/exec` outside `internal/proc`).
10. **Grammar is uniformly `hop <selection> <action>` (gyo0).** `<selection>` = repo / `repo/worktree` / group / `--all`; `<action>` = a builtin verb (`cd`/`where`/`open`), a batch verb (`pull`/`push`/`sync`), a PATH binary, or a shell alias/function. Subcommand classification moved out of the shim into the binary's hidden `--shim-plan` flag, which emits a fixed 3-keyword protocol (`CD`/`RUN_IN_PARENT`/`PASSTHROUGH`). The shim hard-codes ZERO subcommand names — the permanent fix for the stale-shim drift bug. Tool-form is native grammar: a non-verb action classifies as `RUN_IN_PARENT\n<path>` and the shim runs the user's literal words in the parent shell (so aliases/functions resolve — the former `-R` path could not). The trade-off: scripts/CI bypassing the shim use `hop <name> where` for path resolution and run tools themselves.
11. **No `eval` of binary output (Constitution I).** The shim runs the user's already-parsed `"$@"`, and the binary emits only the fixed vocabulary plus a path used as a quoted `cd` operand. There is no re-parsing of binary stdout as code — no shell-injection surface. `eval`-ing binary output was explicitly rejected for this reason.
12. **`open` keeps the `WT_CD_FILE` temp-file side-channel, unified into the PASSTHROUGH arm.** wt's app menu is interactive (full stdio mid-flow) and the "Open here" cd-target arrives only after wt exits — this does not fit the classify-then-act 3-keyword shape, so `open` is classified `PASSTHROUGH`. The shim's `_hop_passthrough` exports `WT_CD_FILE` on every passthrough; `wt open` (and `clone <url>`) write their cd-target there and the shim cds afterward. This collapses the former three handoffs (where=stdout, open=WT_CD_FILE, clone=conditional-stdout) into one channel without a 4th protocol keyword.
13. **`pull`/`push`/`sync` are action tokens, not cobra subcommands (gyo0).** `hop <name> pull` (reoriented from `hop pull <name>`). Plural fan-out via `hop --all pull` / `hop <group> pull` (replacing `hop pull --all`). They classify as `PASSTHROUGH` (the binary owns the resolution + per-repo summary + exit-code policy via `runBatchVerb` → `resolveTargets`/`runBatch`). A plural selection refuses non-batch actions (the interactive guard): `cd`/`open`/tool-form across N repos is nonsensical, so only `pull`/`push`/`sync` are permitted; everything else errors exit 2.
