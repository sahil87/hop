---
description: "Single fixed path `$HOME/.config/hop/hop.yaml` (only `$HOME` consulted; no `$HOP_CONFIG`/`$XDG_CONFIG_HOME`), `Resolve` stat-then-not-found vs. `ResolveWriteTarget` no-stat, the read-vs-write split (readers error, writers auto-init), no fallback to legacy `repos.yaml` paths"
type: memory
---
# Config Search Order

How `hop.yaml` is located on every invocation. Implemented in `src/internal/config/resolve.go`.

There is no search *order*: the config lives at a **single fixed path**, `$HOME/.config/hop/hop.yaml`. The only environment input is `$HOME` (unavoidable, to build the path). No `$HOP_CONFIG`, no `$XDG_CONFIG_HOME` (xgmu) — the path is identical on macOS and Linux by construction. There is no caching — the path is re-derived and re-stat'd on every invocation (Constitution Principle II "No Database").

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
  where `<path>` is the resolved fixed path (e.g., `/Users/you/.config/hop/hop.yaml`). The message points at **both** bootstrap paths (44hm): `hop add` auto-creates the config (so it's a true hint), and `hop config init` writes the annotated starter.
- Any other stat error → wrapped as `hop: stat <path>: <err>`.
- `$HOME` unset → propagates `configPath()`'s `hop: $HOME is not set; cannot locate config`.
- Sentinel `ErrNoConfig` is exported but the actual returned errors use `fmt.Errorf` with the exact messages above (callers don't currently `errors.Is` the sentinel).

**Read-vs-write split** (44hm): `Resolve()` is the **read** seam — every read-command (`hop`, `hop ls`, `hop <name> where`, `hop config print`, and `hop add -p` print-mode dry-runs) errors on absence with the message above, because there's nothing to write. The **write-commands** (`hop add` at both breadths in write mode, `hop clone <url>`) take the `ResolveWriteTarget()` + `config.EnsureSkeleton()` path instead: they auto-create a minimal `repos: {}` skeleton (announced `created: <path>`) rather than erroring. The fixed-path property above is the safety precondition that makes this unambiguous. See [init-bootstrap § Auto-init-on-write](/config/init-bootstrap.md#auto-init-on-write).

There is no `$HOP_CONFIG` set-but-missing hard error. Setting `$HOP_CONFIG` or `$XDG_CONFIG_HOME` to anything (including a bad path) has no effect: only the fixed-path not-found error can fire.

## `ResolveWriteTarget()` semantics

- Returns `configPath()` directly — **no `os.Stat`** — so `init`/`where` get the path regardless of whether the file exists.
- Kept as a distinct exported function (wrapping `configPath()`) so the no-stat-vs-stat seam `init`/`where` rely on is preserved.
- Errors only when `$HOME` is unset (propagated from `configPath()`):
  ```
  hop: $HOME is not set; cannot locate config
  ```

## No fallback to legacy paths

There is exactly one path and no fallback chain — no `$REPOS_YAML`, no `$XDG_CONFIG_HOME/repo/repos.yaml` / `$HOME/.config/repo/repos.yaml`, and no `$HOP_CONFIG` → `$XDG_CONFIG_HOME/hop/hop.yaml` chain. A user with a legacy `repos.yaml`, or anyone relying on `$HOP_CONFIG` / a non-default `$XDG_CONFIG_HOME` to relocate the file, will see "no hop.yaml found at `$HOME/.config/hop/hop.yaml`" until they place (or symlink) their config at the fixed path.

## Design Decisions

1. **Single fixed path, zero env-var overrides.** *Introduced by*: xgmu. There is no `$HOP_CONFIG` or `$XDG_CONFIG_HOME` branching and no set-but-missing hard error. *Why*: env vars exported from `.zshrc` are unset in non-login / AI / CI shells, so an env-driven path makes `hop` unusable in exactly those automation contexts. A literal home-relative path is environment-independent and identical across darwin/linux. Dotfile sync still works — symlink `~/.config/hop/hop.yaml` to a tracked file. A clean break, no migration shim.
2. **Fixed-path absence is unambiguous → write-commands auto-init.** *Introduced by*: 44hm (depends on #1). Once the path is fixed with no env overrides, "no file at the one known path" means exactly one thing — it doesn't exist yet. That removes the only justification for a require-`init`-first gate, so the write-commands (`hop add` at both breadths, `hop clone <url>`) auto-create the config via `EnsureSkeleton` instead of erroring. Auto-init depends on #1: under env-driven resolution a failed resolve is ambiguous and auto-creating would mask a misconfiguration. See [init-bootstrap § Auto-init-on-write](/config/init-bootstrap.md#auto-init-on-write).

## Cross-references

- YAML schema and parsing: [yaml-schema](/config/yaml-schema.md)
- Bootstrap behavior of `hop config init` + auto-init-on-write (`EnsureSkeleton`): [init-bootstrap](/config/init-bootstrap.md)
