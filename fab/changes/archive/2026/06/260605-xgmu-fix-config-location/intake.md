# Intake: Fix config location, remove env-var overrides

**Change**: 260605-xgmu-fix-config-location
**Created**: 2026-06-05
**Status**: Draft

## Origin

> refactor: fix hop.yaml location to ~/.config/hop/hop.yaml, remove $HOP_CONFIG and $XDG_CONFIG_HOME overrides

Initiated conversationally via `/fab-discuss`. The user surfaced a concrete pain point: `$HOP_CONFIG` lives in `.zshrc`, and AI workflows (and any non-login, non-interactive environment) don't source `.zshrc`. In those environments `$HOP_CONFIG` is unset, so `hop` either falls through to a path with no config or hard-errors with "no hop.yaml found" — making `hop` unusable in exactly the automation contexts the user cares about.

Key decisions reached in discussion:
- The dotfile-sync workflow that `$HOP_CONFIG` was meant to enable **does not actually depend on the env var** — a synced file can just as easily live at `~/.config/hop/hop.yaml` (via symlink, or by having a dotfile manager own that path). So removing the override costs nothing on the sync front.
- The user explicitly chose the **fully-fixed** variant: consult **zero** env vars for the path. Not just dropping `$HOP_CONFIG` while keeping `$XDG_CONFIG_HOME` — both go. The only env input remaining is `$HOME` (unavoidable, to build the path).
- The user explicitly chose to **keep the nested XDG-style path** (`~/.config/hop/hop.yaml`), not flatten to `~/.config/hop.yaml`. This avoids migrating the user's existing file and is standard XDG app layout.
- Rationale for "zero env vars" over "keep XDG": the goal is one path, always, immune to any environment difference between a login shell and an AI/CI environment. `$XDG_CONFIG_HOME` is rarely set on macOS but *can* differ between environments, which would reintroduce the exact class of problem being eliminated. A literal path is also identical across darwin/linux without OS-derivation logic (notably, this means *not* using Go's `os.UserConfigDir()`, which resolves to `~/Library/Application Support` on macOS).

This change is **Change A** of a two-change sequence discussed together. **Change B** (`auto-init on write-commands`) depends on this change: auto-creating a config on absence is only safe once "no file at the one known path" has exactly one meaning (it doesn't exist yet) — with no env override, there is no typo'd-path ambiguity for auto-init to mask. Change B is drafted separately.

## Why

**The problem.** `hop`'s config resolution today consults two environment variables before its fixed fallback (`src/internal/config/resolve.go`):

1. `$HOP_CONFIG` (highest precedence; hard-errors if set but the file is missing)
2. `$XDG_CONFIG_HOME/hop/hop.yaml` (if `$XDG_CONFIG_HOME` is set)
3. `$HOME/.config/hop/hop.yaml` (fixed fallback)

The user points `$HOP_CONFIG` at a dotfile-synced location. That env var is exported from `.zshrc`. AI/automation environments (and any non-interactive shell) don't source `.zshrc`, so `$HOP_CONFIG` is unset there, candidate 3's file doesn't exist (the real config is elsewhere), and every `hop` invocation fails. The config is *present on disk* — hop just can't find it because the locator depends on shell state the automation context lacks.

**Consequence of not fixing.** `hop` remains unusable in AI workflows, CI, and any environment that doesn't replicate the user's interactive shell setup. The user has to special-case env export per environment — exactly the brittle, location-is-not-fixed problem at hand.

**Why this approach over alternatives.** Three options were weighed in discussion:

1. *Symlink, no code change* — keep `$HOP_CONFIG`, symlink `~/.config/hop/hop.yaml` at the dotfile, stop relying on the env var. Rejected as the long-term answer: it leaves the env-var precedence in place (still a footgun, still divergent between environments) and depends on every environment having the symlink set up.
2. *Demote `$HOP_CONFIG` to lowest precedence* — fixed path wins by default, env var as explicit opt-out. Rejected: still env-sensitive; alters the documented hard-error semantics (Design Decision #2 in `config-resolution.md`) in a confusing half-way manner.
3. *Hardcode the location, remove the env vars entirely* — **chosen.** Simplest mental model ("config is always at `~/.config/hop/hop.yaml`"), environment-independent, identical across platforms. The cost (loses env-based config relocation) is illusory because dotfile sync works via symlink at the fixed path.

This aligns with Constitution Principle III (Convention Over Configuration): the XDG path *is* the convention; `$HOP_CONFIG` was the escape hatch. Removing it makes hop more convention-driven, not less. It mirrors the prior clean break that retired `$REPOS_YAML` (Design Decision #6) — env vars treated as personal infrastructure, not a public contract, so a hard removal is acceptable.

## What Changes

### 1. Resolver collapses to a single fixed path (`src/internal/config/resolve.go`)

Both entry points lose all env-var branching except `$HOME`.

**`Resolve()`** — used by every read path. Today it walks `$HOP_CONFIG` → `$XDG_CONFIG_HOME/hop/hop.yaml` → `$HOME/.config/hop/hop.yaml` with a hard-error on `$HOP_CONFIG` set-but-missing. After: it builds the single path `$HOME/.config/hop/hop.yaml`, stats it, returns it if present, and otherwise returns the not-found error.

**`ResolveWriteTarget()`** — used by `hop config init` and `hop config where`. Today it walks the same order without stat checks. After: it returns `$HOME/.config/hop/hop.yaml` with no env-var consultation and no stat.

The two functions share a single path-builder helper. Proposed shape:

```go
// configPath returns the single, fixed config location. The only environment
// input is $HOME (unavoidable). No $HOP_CONFIG, no $XDG_CONFIG_HOME — the path
// is identical on macOS and Linux by construction.
func configPath() (string, error) {
    home := os.Getenv("HOME")
    if home == "" {
        return "", fmt.Errorf("hop: $HOME is not set; cannot locate config")
    }
    return filepath.Join(home, ".config", "hop", "hop.yaml"), nil
}

func Resolve() (string, error) {
    p, err := configPath()
    if err != nil {
        return "", err
    }
    if _, err := os.Stat(p); err != nil {
        if os.IsNotExist(err) {
            return "", fmt.Errorf("hop: no hop.yaml found at %s. Run 'hop config init' to create one.", p)
        }
        return "", fmt.Errorf("hop: stat %s: %w", p, err)
    }
    return p, nil
}

func ResolveWriteTarget() (string, error) {
    return configPath()
}
```

`ResolveWriteTarget()` keeps a distinct identity (no stat) even though it now wraps `configPath()` directly — `init`/`where` must return the path regardless of file existence. The `ErrNoConfig` sentinel is retained (exported) for compatibility; the returned error continues to use `fmt.Errorf` with the message above.

### 2. Removed behaviors

- **`$HOP_CONFIG` set-but-missing hard error disappears.** There is no env var to be misconfigured, so the entire `resolve.go:22-27` hard-error branch (`"$HOP_CONFIG points to <path>, which does not exist..."`) and its `stat $HOP_CONFIG` error are deleted. This is a documented behavior (Design Decision #2, and a GIVEN/WHEN/THEN scenario in `config-resolution.md`) — its removal is intentional and must be reflected in specs/memory.
- **`$XDG_CONFIG_HOME` is no longer consulted at all.** Even when set, it does not move the path.

### 3. Error message rewrites

Three user-facing messages reference `$HOP_CONFIG` and must change:

- **`resolve.go` "no hop.yaml found"** (line 49) — drop the "Set $HOP_CONFIG to a tracked file (e.g., a Dropbox path or a git-tracked dotfile)" advice. New wording names the fixed path and points at `hop config init`:
  `hop: no hop.yaml found at /Users/you/.config/hop/hop.yaml. Run 'hop config init' to create one.`
- **`src/internal/config/config.go` init "already exists"** (line 269) — drop "or set $HOP_CONFIG to a different path." New wording:
  `hop config init: <path> already exists. Delete it first to recreate it.`
- **`src/cmd/hop/config.go` init success tip** (line 36) — the entire `$HOP_CONFIG`-portability tip is now wrong advice. Replace with symlink-based sync guidance:
  `Tip: to sync this config across machines, keep it in your dotfiles and symlink ~/.config/hop/hop.yaml to it.`

### 4. `hop add` missing-config message (`src/cmd/hop/config_add.go:78-86`)

`runAdd` currently composes a bootstrap path via `ResolveWriteTarget()` for its "no hop.yaml found" message. The path-building still works (returns the fixed path), so this message stays structurally correct — but its wording should be reviewed for `$XDG_CONFIG_HOME` fallback references in the `werr` branch (`bootstrap = "$XDG_CONFIG_HOME/hop/hop.yaml"` at line 82). With XDG gone, that fallback literal is misleading and should become a plain description or be dropped (the `werr` case now only fires when `$HOME` is unset).

> NOTE: This change does NOT alter `hop add`'s require-existing-config gate — that gate is *replaced* by Change B (auto-init). Change A only fixes the path and the message wording.

### 5. Root help text (`src/cmd/hop/root.go:14,69`)

- Line 14 (`rootLong` notes): drop "Optional: set $HOP_CONFIG in your shell rc to point at a tracked file".
- Line 69 (`rootLong` config search-order note): the "search order: $HOP_CONFIG, then $XDG_CONFIG_HOME/hop/hop.yaml, then $HOME/.config/hop/hop.yaml" line is no longer accurate. Replace with a single statement: `Config lives at ~/.config/hop/hop.yaml.`

### 6. README (`README.md:11,101`)

- Line 11 ("One config, every machine"): "Drop it in Dropbox, dotfiles, or `$HOP_CONFIG`" → reword around symlinking the fixed path.
- Line 101: "By default the file lives at `$XDG_CONFIG_HOME/hop/hop.yaml` (or `~/.config/hop/hop.yaml`). Set `$HOP_CONFIG`..." → "The file lives at `~/.config/hop/hop.yaml`. To sync it across machines, keep it in your dotfiles and symlink that path to it."

### 7. Starter content comment (`src/internal/config/starter.yaml`)

The embedded starter's header comment (lines 32-33 in `init-bootstrap.md`'s reproduction) says "Tip: set $HOP_CONFIG to a tracked path (dotfiles, Dropbox) so this config moves with you across machines." Update to the symlink guidance to stay consistent with the new init tip. `TestStarterParses` must still pass (comment-only change, parses identically).

### 8. Tests (`src/.../*_test.go`)

Seven test files reference `$HOP_CONFIG`:
`src/cmd/hop/config_add_test.go`, `config_test.go`, `integration_test.go`, `repo_completion_test.go`, `testutil_test.go`; `src/internal/config/config_test.go`, `resolve_test.go`.

Two categories of edit, per Constitution "Test Integrity" (tests conform to the spec, not the reverse):

- **Delete** the `$HOP_CONFIG`-specific behavior tests: the set-but-missing hard-error case, the `$HOP_CONFIG` precedence-wins cases, and any `$XDG_CONFIG_HOME` precedence cases. These assert behavior that no longer exists.
- **Migrate** the fixture-feeding mechanism: tests that set `$HOP_CONFIG=<tmpfile>` to point hop at a fixture config must instead override `$HOME=<tmpdir>` and place the fixture at `<tmpdir>/.config/hop/hop.yaml`. The shared helper in `testutil_test.go` is the highest-leverage edit — most other tests likely route through it. Update the helper first, then fix call sites.

## Affected Memory

- `config/search-order`: (modify) Rewrite around the single fixed path. Remove `$HOP_CONFIG`/`$XDG_CONFIG_HOME` precedence, the set-but-missing hard error, and the multi-candidate walk. Document `$HOME`-only resolution and the "no fallback to legacy paths" note remains.
- `config/init-bootstrap`: (modify) Update the "already exists" message (drop `$HOP_CONFIG` reference), the init success tip (symlink guidance), and the starter content comment reproduction.
- `config/index`: (modify) The one-liner currently reads "`$HOP_CONFIG` → `$XDG_CONFIG_HOME/hop/hop.yaml` → `$HOME/.config/hop/hop.yaml`, hard-error semantics, ..." — rewrite to the single fixed path.
- `cli/subcommands`: (modify) The `hop config init` and `hop config print` rows reference `$HOP_CONFIG` in their error/tip descriptions; update to match the new wording.
- `architecture/package-layout`: (modify) Line 33 comment "`resolve.go # $HOP_CONFIG search order`" → "single fixed config path".

## Impact

**Code:**
- `src/internal/config/resolve.go` — core change (collapse both resolvers).
- `src/internal/config/config.go` — "already exists" message.
- `src/internal/config/starter.yaml` — header comment.
- `src/cmd/hop/config.go` — init success tip.
- `src/cmd/hop/config_add.go` — bootstrap-path message wording (the `$XDG_CONFIG_HOME` literal fallback).
- `src/cmd/hop/root.go` — `rootLong` help notes.
- 7 test files (see What Changes §8).

**Docs:** `README.md`; specs (`docs/specs/config-resolution.md`) and memory updated at hydrate.

**No new dependencies. No schema change. No CLI surface change** (no subcommands added/removed). This is purely a resolution-logic narrowing.

**Cross-platform:** the fixed literal path is identical on darwin-arm64/amd64 and linux-arm64/amd64 (Constitution "Cross-Platform Behavior" satisfied trivially — no platform branch).

**Breaking change for existing users:** anyone (the user, across machines, plus any scripts/CI) currently relying on `$HOP_CONFIG` or a non-default `$XDG_CONFIG_HOME` will need to migrate to the fixed path (symlink). `hop` is personal infrastructure, so this is an acceptable clean break — consistent with the `$REPOS_YAML` precedent. Worth a one-line note in the change's eventual PR/release notes.

## Open Questions

- None blocking. The two minor implementation details below are Tentative and resolvable at apply time.

## Clarifications

### Session 2026-06-05

| # | Action | Detail |
|---|--------|--------|
| 9 | Confirmed | Wording kept; clarified that Change A never auto-creates — message fires on every command pre-bootstrap, and Change B narrows it to read-only commands |
| 10 | Confirmed | `ResolveWriteTarget()` stays distinct (no-stat vs. stat seam) |

### Session 2026-06-05 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 6 | Confirmed | — |
| 7 | Confirmed | — |
| 8 | Confirmed | — |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Fixed path is `~/.config/hop/hop.yaml`, nested (not flattened to `~/.config/hop.yaml`) | Discussed — user explicitly chose nested to avoid migrating existing file; standard XDG layout | S:98 R:80 A:90 D:95 |
| 2 | Certain | Consult zero env vars for the path; remove both `$HOP_CONFIG` and `$XDG_CONFIG_HOME` | Discussed — user explicitly chose the fully-fixed variant over keep-XDG | S:98 R:70 A:90 D:95 |
| 3 | Certain | `$HOME` remains the sole env input (unavoidable to build the path) | No alternative — a fixed home-relative path requires `$HOME` | S:95 R:85 A:95 D:95 |
| 4 | Certain | Use a literal `filepath.Join(home, ".config", "hop", "hop.yaml")`, NOT Go's `os.UserConfigDir()` | Discussed — `os.UserConfigDir()` diverges to `~/Library/Application Support` on macOS, defeating cross-platform consistency | S:90 R:80 A:90 D:90 |
| 5 | Certain | The `$HOP_CONFIG` set-but-missing hard error is removed entirely | Direct consequence of removing the env var — no var to misconfigure | S:95 R:75 A:90 D:95 |
| 6 | Certain | This is a clean break with no migration shim (no env-var fallback retained) | Clarified — user confirmed | S:95 R:55 A:80 D:80 |
| 7 | Certain | Sync guidance changes from "set `$HOP_CONFIG`" to "symlink the fixed path" everywhere it appears (init tip, README, starter comment) | Clarified — user confirmed | S:95 R:75 A:85 D:80 |
| 8 | Certain | Tests are migrated from `$HOP_CONFIG=<tmpfile>` to `$HOME=<tmpdir>` fixture feeding; behavior-specific env tests deleted | Clarified — user confirmed | S:95 R:70 A:85 D:75 |
| 9 | Certain | Minimal not-found message wording: `hop: no hop.yaml found at <path>. Run 'hop config init' to create one.` Note: Change A never auto-creates — this message fires on every command (read and write) when no config exists; Change B later narrows it to read-only commands and refines wording to also point at `hop add`. | Clarified — user confirmed (after discussion of when the message fires) | S:95 R:90 A:75 D:55 |
| 10 | Certain | `ResolveWriteTarget()` is kept as a distinct function (wrapping `configPath()`) rather than merged into `Resolve()` | Clarified — user confirmed | S:95 R:85 A:75 D:50 |

10 assumptions (10 certain, 0 confident, 0 tentative, 0 unresolved).
