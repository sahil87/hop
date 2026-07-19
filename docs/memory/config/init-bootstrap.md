---
description: "`hop config init` write target, mode 0644, embedded grouped-form starter; post-write tip text (`hop add -r <dir>`); **auto-init-on-write** (`EnsureSkeleton` minimal `repos: {}` skeleton for `hop add` write mode / `clone <url>`, `created:` announcement, read-vs-write split)"
type: memory
---
# Config Init / Bootstrap

How `hop config init` and `hop config where` behave, plus **auto-init-on-write** (the write-commands self-bootstrap a missing config). Implemented in `src/cmd/hop/config.go`; the actual file write is in `src/internal/config/config.go::WriteStarter` (explicit `init`) and `src/internal/config/config.go::EnsureSkeleton` (auto-init). Starter content is embedded from `src/internal/config/starter.yaml`.

## `hop config init`

1. Calls `config.ResolveWriteTarget()` ([search-order](/config/search-order.md)) — does NOT trigger the missing-file hard error.
2. Calls `config.WriteStarter(target)`:
   - If target exists → returns:
     ```
     hop config init: <path> already exists. Delete it first to recreate it.
     ```
     Exit 1. Existing file is untouched.
   - Creates parent dir via `os.MkdirAll(dir, 0o755)` if absent.
   - Writes `starterContent` (embedded via `//go:embed starter.yaml`) with file mode **0644**.
3. Stdout: `Created <path>`.
4. Stderr (two lines):
   ```
   Edit the file to add your repos, or run `hop add -r <dir>` to populate from existing on-disk repos.
   Tip: to sync this config across machines, keep it in your dotfiles and symlink ~/.config/hop/hop.yaml to it.
   ```
   The first line surfaces `hop add -r` for onboarding discoverability — without it, the recursive bootstrap is invisible to new users (w2bj). The `Tip:` line gives symlink-based sync guidance: since the config lives at a single fixed path (no `$HOP_CONFIG` override — xgmu), dotfile sync is achieved by symlinking that path to a tracked file.
5. Exit 0.

The `0644` mode is intentional: the file contains repo paths and public git URLs — no credentials. Treating it as sensitive (0600) would be theater.

## Embedded starter content

Stored verbatim at `src/internal/config/starter.yaml` and pulled in via `//go:embed`. Self-bootstrapping — points at this repo so a fresh user can `hop` (fzf shows one entry) or `hop clone hop` immediately:

```yaml
# hop config — locator and operations registry.
# Edit to add repos. Tip: to sync this config across machines, keep it in your
# dotfiles and symlink ~/.config/hop/hop.yaml to it.
#
# Two ways to add a repo:
#   1. Append a URL to a flat group (default) — convention applies:
#      path = <config.code_root>/<org-from-url>/<name-from-url>
#   2. Use a named group with explicit `dir:` to override convention.

config:
  code_root: ~/code

repos:
  default:
    - git@github.com:sahil87/hop.git    # the locator tool itself

  # Example: vendor group with explicit dir override.
  # vendor:
  #   dir: ~/vendor
  #   urls:
  #     - git@github.com:some-vendor/their-tool.git
```

`config.StarterContent() []byte` exposes the embedded bytes for tests that compare exact contents.

The starter parses cleanly under the schema validator (verified by `TestStarterParses` in `config_test.go`).

## Auto-init-on-write

The **write-commands** auto-create a minimal `hop.yaml` when it is absent — `init` is not a precondition for them (44hm). The commands that auto-init:

| Command | Auto-inits when | Source |
|---|---|---|
| `hop add <dir>` / `hop add -r <dir>` (and hidden alias `hop config add`) | in **write mode** (default; the recursive `-r` breadth still auto-inits) — **print mode** (`-p`, any breadth) never touches the file, so there's nothing to bootstrap | `config_add.go::runAdd` (via `resolveAddConfig`) |
| `hop clone <url>` | always (a fresh file is needed for path resolution) | `clone.go::cloneURL` |

### The minimal skeleton (`config.EnsureSkeleton`)

```go
func EnsureSkeleton(path string) (created bool, err error)
```

- **Content**: exactly the bytes `repos: {}\n` (named constant `skeletonContent` in `config.go`). No `config:` block, no `code_root`, no header comment. `code_root` defaults to `~` when the `config:` block is absent (see [search-order](/config/search-order.md) / [yaml-schema](/config/yaml-schema.md)), so the bare skeleton is fully functional — convention repos still land at `<code_root>/<org>/<name>`.
- **Create path**: when the file is absent, `os.MkdirAll(filepath.Dir(path), 0o755)` then `os.WriteFile(path, skeletonContent, 0o644)`, returning `(true, nil)`. Same 0644 mode rationale as the starter (repo paths + public URLs, no credentials).
- **No-op path**: when the file already exists it is left **byte-for-byte unchanged** and returns `(false, nil)` — it never overwrites (the inverse trigger of `WriteStarter`: absence creates, presence is the no-op). This makes auto-init **idempotent** — a second write-command run does not re-announce `created:`.
- **Stat errors**: a stat error other than not-exist (e.g. a permission error) is returned, not swallowed (`hop: stat <path>: <err>`).

### `created:` stderr announcement

Auto-init is **announced, not silent**: when `EnsureSkeleton` returns `created==true`, the calling command writes `created: <path>` to stderr (colon form, no command-name prefix — matches the neighboring `added:` / `wrote:` / `removed:` lines in the same functions). This preserves the project's "no surprises" posture: the user knows a new file appeared at `~/.config/hop/hop.yaml`.

### `hop clone <url>` ensure-group step

A fresh `repos: {}` skeleton has **no groups** at all, so `clone`'s target group (default `default`, or `--group <name>`) won't exist yet and `yamled.AppendURL` would return `ErrGroupNotFound`. So `cloneURL` calls `yamled.EnsureGroup(path, group)` — which idempotently adds `<group>: []` under `repos` (synthesizing the `repos` mapping if absent) — **only when `created==true`** (i.e. when *it* just created the skeleton). On a **pre-existing** config it does NOT call `EnsureGroup`, so a typo'd `--group <nonexistent>` still hits `findGroup==nil → error` — `AppendURL`'s `ErrGroupNotFound` typo-catching contract is preserved unchanged. See [cli/subcommands § Ad-hoc URL clone](/cli/subcommands.md#ad-hoc-url-clone) and [add-register § -g forced group](/config/add-register.md#-g-forced-group-auto-create) (which reuses the same `yamled.EnsureGroup` helper). Note: `hop clone --no-add <url>` on a fresh machine **still creates** the config (the skeleton + target group are needed for path resolution); `--no-add` suppresses only the URL write-back, not the file's creation.

### Read-vs-write split

**Read-commands do NOT auto-init.** `hop`, `hop ls`, `hop <name> where`, and `hop config print` (and `hop add -p` *print*-mode dry-runs, any breadth) keep calling `config.Resolve()` and erroring on absence — they have nothing to write, so silently conjuring an empty config just to then report "no repos" is worse UX than a clear error. The not-found message names both bootstrap paths (44hm):

```
hop: no hop.yaml found at <path>. Run 'hop add <dir>' to register a repo (creates the config), or 'hop config init' for a starter.
```

`hop rm` / `hop config rm` are **also not** in the auto-init set — `rm` has nothing to register on a fresh machine, so it has its own gate message (`Run 'hop config init' first, then re-run rm.`, `config_rm.go`). See [search-order § Resolve() semantics](/config/search-order.md#resolve-semantics).

### `init` vs. auto-init: which writes what

| | `hop config init` | Auto-init (`EnsureSkeleton`) |
|---|---|---|
| Content | embedded **annotated starter** (`starter.yaml`: comments, `code_root: ~/code`, the `hop.git` self-bootstrap seed) | **bare skeleton** `repos: {}\n` only |
| Trigger | explicit user command | side effect of a write-command on a fresh machine |
| Overwrite? | refuses (errors `... already exists`) | refuses (silent no-op, `created=false`) |
| Announcement | stdout `Created <path>` + tip lines | stderr `created: <path>` |

### Design Decisions

1. **Auto-init writes a minimal skeleton, NOT the annotated starter.** *Introduced by*: 44hm. *Why*: the starter seeds `git@github.com:sahil87/hop.git` to give a bare-`init` user *something* to `hop` at. But a user running `hop add <dir>` / `scan --write` / `clone <url>` already carries their own intent — injecting the unrelated `hop.git` self-bootstrap repo they didn't ask for would be surprising. So auto-init writes `repos: {}` and the command applies its operation on top; the result contains exactly what the user asked for. *Rejected*: (a) seeding an empty `default:` group into the skeleton — contradicts truly-minimal and doesn't generalize to `clone --group <other>` (clone's `EnsureGroup` handles any group uniformly); (c) reusing the full starter — injects the unwanted seed.
2. **Only write-commands auto-init; readers keep erroring.** *Introduced by*: 44hm. Writers have something to write, so create-on-absence is natural and intent-preserving. Readers have nothing to write — a clear not-found error (now pointing at `hop add`) beats a conjured empty config that immediately reports "no repos".
3. **Dependency on the fixed-path property.** *Introduced by*: xgmu. Auto-init is only *safe* because the config lives at exactly one fixed path with no env overrides — so "no file at the one known path" has exactly one meaning: it doesn't exist yet. Under env-driven resolution a failed resolve is ambiguous (forgot to init? typo'd `$HOP_CONFIG`?), and auto-creating would silently mask a misconfiguration. The unambiguous-absence precondition is what makes auto-init sound. See [search-order § Design Decisions](/config/search-order.md#design-decisions).

## `hop config where`

Prints `config.ResolveWriteTarget()` to stdout. Exit 0 unless `$HOME` is unset (the only case `ResolveWriteTarget()` can error). Never errors on missing file — it's a debug aid, not a load.

Named `where` for voice-fit consistency with the locator's `hop where`. Both `init` and `where` are exempt from the standard "load `hop.yaml` first" flow; they run cleanly even when no config exists yet.

## Cross-references

- Bootstrap-then-populate workflow (`hop config init` followed by `hop add -r <dir>`, or just `hop add -r <dir>` directly on a fresh machine): [add-register](/config/add-register.md)
- Search order shared by `init`, `where`, and `add`'s precondition check; the refined `Resolve()` not-found message and the read-vs-write split: [search-order](/config/search-order.md)
- Auto-init wiring per write-command + exit-code changes: [cli/subcommands](/cli/subcommands.md) (`hop add`, `hop clone <url>` rows)
- `yamled.EnsureGroup` (clone's ensure-target-group helper, also used by `hop add -g`) and the `hop add` write-mode precondition: [add-register](/config/add-register.md)
