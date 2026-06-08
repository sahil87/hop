# Config Search Order

How `hop.yaml` is located on every invocation. Implemented in `src/internal/config/resolve.go`.

There is no search *order* anymore: the config lives at a **single fixed path**, `$HOME/.config/hop/hop.yaml`. The only environment input is `$HOME` (unavoidable, to build the path). No `$HOP_CONFIG`, no `$XDG_CONFIG_HOME` — both env-var overrides were removed in change `260605-xgmu-fix-config-location`, so the path is identical on macOS and Linux by construction. There is no caching — the path is re-derived and re-stat'd on every invocation (Constitution Principle II "No Database").

Two entry points share one `configPath()` helper:

- `Resolve() (string, error)` — used by every load path. Stats the fixed path; returns it if present, else a not-found error.
- `ResolveWriteTarget() (string, error)` — used by `hop config init` and `hop config where`. Returns the path that *would* be used regardless of file existence (no stat).

## `configPath()` helper (shared)

`configPath() (string, error)` (unexported) builds the single fixed location:

```go
home := os.Getenv("HOME")
if home == "" {
    return "", fmt.Errorf("hop: $HOME is not set; cannot locate config")
}
return filepath.Join(home, ".config", "hop", "hop.yaml"), nil
```

The literal `filepath.Join` construction is deliberate: `os.UserConfigDir()` was **rejected** because it resolves to `~/Library/Application Support` on macOS, which would defeat the cross-platform-identical goal. The only failure mode is `$HOME` unset.

## `Resolve()` semantics

- Calls `configPath()`, then `os.Stat`s the result.
- File exists → return the path, nil.
- `os.IsNotExist` → not-found error:
  ```
  hop: no hop.yaml found at <path>. Run 'hop add <dir>' to register a repo (creates the config), or 'hop config init' for a starter.
  ```
  where `<path>` is the resolved fixed path (e.g., `/Users/you/.config/hop/hop.yaml`). The message points at **both** bootstrap paths (refined in change `260605-44hm-auto-init-on-write`): `hop add` now auto-creates the config (so it's a true hint), and `hop config init` writes the annotated starter.
- Any other stat error → wrapped as `hop: stat <path>: <err>`.
- `$HOME` unset → propagates `configPath()`'s `hop: $HOME is not set; cannot locate config`.
- Sentinel `ErrNoConfig` is exported but the actual returned errors use `fmt.Errorf` with the exact messages above (callers don't currently `errors.Is` the sentinel).

**Read-vs-write split** (change `260605-44hm-auto-init-on-write`): `Resolve()` is the **read** seam — every read-command (`hop`, `hop ls`, `hop <name> where`, `hop config print`, and `hop add -p` print-mode dry-runs) errors on absence with the message above, because there's nothing to write. The **write-commands** (`hop add` at both breadths in write mode, `hop clone <url>`) take the `ResolveWriteTarget()` + `config.EnsureSkeleton()` path instead: they auto-create a minimal `repos: {}` skeleton (announced `created: <path>`) rather than erroring. (The former `hop config scan --write` write-command folded into `hop add -r` when scan was deleted — change `260608-w2bj-unify-recursive-add`.) The fixed-path property above is the safety precondition that makes this unambiguous. See [init-bootstrap § Auto-init-on-write](init-bootstrap.md#auto-init-on-write-change-260605-44hm-auto-init-on-write).

There is no `$HOP_CONFIG` set-but-missing hard error — that branch was deleted along with the env var. Setting `$HOP_CONFIG` or `$XDG_CONFIG_HOME` to anything (including a bad path) has no effect: only the fixed-path not-found error can fire.

## `ResolveWriteTarget()` semantics

- Returns `configPath()` directly — **no `os.Stat`** — so `init`/`where` get the path regardless of whether the file exists.
- Kept as a distinct exported function (wrapping `configPath()`) so the no-stat-vs-stat seam `init`/`where` rely on is preserved (intake Assumption 10).
- Errors only when `$HOME` is unset (propagated from `configPath()`):
  ```
  hop: $HOME is not set; cannot locate config
  ```

## No fallback to legacy paths

The previous v0.0.1 search order (`$REPOS_YAML`, `$XDG_CONFIG_HOME/repo/repos.yaml`, `$HOME/.config/repo/repos.yaml`) is **gone**, and so is the later `$HOP_CONFIG` → `$XDG_CONFIG_HOME/hop/hop.yaml` → `$HOME/.config/hop/hop.yaml` chain. There is exactly one path and no fallback chain. A user with a v0.0.1 `repos.yaml`, or anyone who relied on `$HOP_CONFIG` / a non-default `$XDG_CONFIG_HOME` to relocate the file, will see "no hop.yaml found at `$HOME/.config/hop/hop.yaml`" until they place (or symlink) their config at the fixed path.

## Design Decisions

1. **Single fixed path, zero env-var overrides** (change `260605-xgmu-fix-config-location`): `$HOP_CONFIG` and `$XDG_CONFIG_HOME` branching (and the `$HOP_CONFIG` set-but-missing hard error) were removed entirely. *Why*: env vars exported from `.zshrc` are unset in non-login / AI / CI shells, so the env-driven path made `hop` unusable in exactly those automation contexts. A literal home-relative path is environment-independent and identical across darwin/linux. Dotfile sync still works — symlink `~/.config/hop/hop.yaml` to a tracked file. This is a clean break (no migration shim), consistent with the prior `$REPOS_YAML` removal.
2. **Fixed-path absence is unambiguous → write-commands auto-init** (change `260605-44hm-auto-init-on-write`, depends on #1): once the path is fixed with no env overrides, "no file at the one known path" means exactly one thing — it doesn't exist yet. That removes the only justification for the write-commands' old require-`init`-first gate, so `hop add` / `scan --write` / `clone <url>` now auto-create the config via `EnsureSkeleton` instead of erroring. The auto-init change SHALL NOT have shipped before the fixed-path change (#1), because under env-driven resolution a failed resolve was ambiguous and auto-creating would have masked a misconfiguration. See [init-bootstrap § Auto-init-on-write](init-bootstrap.md#auto-init-on-write-change-260605-44hm-auto-init-on-write).

## Cross-references

- YAML schema and parsing: [yaml-schema](yaml-schema.md)
- Bootstrap behavior of `hop config init` + auto-init-on-write (`EnsureSkeleton`): [init-bootstrap](init-bootstrap.md)
