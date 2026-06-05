# Intake: Auto-init hop.yaml on write-commands

**Change**: 260605-44hm-auto-init-on-write
**Created**: 2026-06-05
**Status**: Draft

## Origin

> feat: auto-init hop.yaml on write-commands (add, scan --write, clone url) when config absent, instead of erroring

Initiated conversationally via `/fab-discuss`, as the follow-on to the fix-config-location discussion. The user observed that once the config has a single fixed location, the first command a user runs can *be* the bootstrap: `hop add <dir>` should create the config when it's missing rather than forcing a separate `hop config init` first.

**Dependency:** This is **Change B**, and it depends on **Change A** (`260605-xgmu-fix-config-location`). Change A removes `$HOP_CONFIG`/`$XDG_CONFIG_HOME` so the config has exactly one fixed path. That fixed-path property is the *precondition* that makes auto-init safe: with env overrides gone, "no file at the one known path" has exactly one meaning — it doesn't exist yet. Today, a `Resolve()` failure is ambiguous (forgot to init? typo'd `$HOP_CONFIG`?), and auto-creating on the latter would silently mask the config bug that Change A's removed hard error was designed to catch. Change B SHALL NOT be implemented before Change A lands.

Key decisions reached in discussion:
- **Auto-init writes a minimal skeleton, NOT the embedded starter content.** A user running `hop add <dir>` is telling hop exactly what to register; seeding the starter's `git@github.com:sahil87/hop.git` self-bootstrap entry would inject an unrelated repo they didn't ask for. The starter's hop.git seed exists to give a bare-`init` user *something* to `hop` at — that purpose doesn't apply to add/scan/clone, which already carry the user's intent. So: create `repos: {}` (minimal), then apply the operation on top. Result contains exactly what the user asked for.
- **Only write-commands auto-init; read-commands do not.** Writers (`hop add`, `hop config scan --write`, `hop clone <url>`) have something to write, so create-on-absence is natural. Readers (`hop`, `hop ls`, `hop <name> where`, `hop config print`) have nothing to write — silently conjuring an empty config just to then report "no repos" is worse UX than a clear error. Readers keep erroring, but with the improved message Change A introduces (naming the one fixed path).
- **Auto-init is announced, not silent.** Emit `created <path>` to stderr when a write-command auto-creates the config, preserving the project's "no surprises" posture. The user should know a new file appeared at `~/.config/hop/hop.yaml`.
- **`hop config init` is retained.** Explicit init still writes the annotated starter (example group comments + hop.git self-bootstrap) for users who want the documented template without registering anything. Auto-init does not replace `init`; it removes `init` as a *mandatory precondition* for the write-commands.

## Why

**The problem.** Today the write-commands hard-gate on an existing `hop.yaml`. `hop add <dir>` against a fresh machine prints (`src/cmd/hop/config_add.go:78-86`):

```
hop add: no hop.yaml found at ~/.config/hop/hop.yaml.
Run 'hop config init' first, then re-run add.
```

This is a two-step dance for zero benefit: the command already knows the exact path it would write to, it's about to write to it anyway, and `init` just drops a file there. `hop config scan --write` and `hop clone <url>` have the same precondition. A brand-new user (or an AI workflow on a fresh environment) hits a wall on their very first useful command and must context-switch to `init`.

**Consequence of not fixing.** Onboarding stays a two-step ritual. The friction is small per-occurrence but hits every fresh environment — and Change A specifically makes fresh AI/CI environments a first-class use case, so the friction compounds exactly where the user wants hop to "just work."

**Why this approach over alternatives.** Auto-init on the write path is the minimal, intent-preserving fix: the command the user already wants to run becomes self-bootstrapping. The alternative (keep requiring explicit `init`) preserves a gate whose only justification — ambiguity of a failed resolve — is eliminated by Change A. Seeding the full starter on auto-init was rejected (injects an unrelated repo). Auto-initing on read commands was rejected (nothing to write; worse UX than a clear error). This aligns with Constitution Principle III (Convention Over Configuration): not requiring an explicit setup step when the convention (fixed path) makes the target unambiguous.

## What Changes

### 1. New shared helper: ensure-config-exists (write path)

Introduce a helper used by the three write-commands that, given the fixed config path (from `config.ResolveWriteTarget()`), creates a minimal skeleton if the file is absent. Proposed location: `src/internal/config/` (alongside `WriteStarter`), e.g. `EnsureSkeleton(path string) (created bool, err error)`:

- If the file exists → return `created=false, nil` (no-op; never overwrites — mirrors `WriteStarter`'s refuse-to-clobber posture, but here absence is the trigger, presence is the no-op).
- If absent → `os.MkdirAll(dir, 0o755)`, then write the minimal skeleton bytes with mode **0644** (same mode rationale as the starter — repo paths + public URLs, no credentials), return `created=true, nil`.

The caller emits `created <path>` to stderr when `created==true`, then proceeds with its normal write logic against the now-existing file.

**Minimal skeleton content** (Tentative — see Assumption 8): the leading candidate is just:

```yaml
repos: {}
```

`code_root` defaults to `~` when the `config:` block is absent (per `docs/specs/config-resolution.md` — "Optional. Defaults to `~`"). The `add`/`scan` group-convention logic lands convention repos at `<code_root>/<org>/<name>`, so `repos: {}` alone is functional; the open question is only whether to *also* seed an explicit `config:\n  code_root: ~/code` for ergonomics. Default lean: `repos: {}` only — minimal means minimal, and the bare-`~` default is sane.

### 2. `hop add <dir>` (`src/cmd/hop/config_add.go`)

Replace the require-existing-config gate (lines 78-86) with auto-init. Today step 2 of `runAdd` calls `config.Resolve()` and, on failure, prints the "Run 'hop config init' first" message and returns `errSilent`. After:

- Resolve the write target via `config.ResolveWriteTarget()` (always succeeds post-Change-A unless `$HOME` is unset).
- Call `EnsureSkeleton(path)`; emit `created <path>` to stderr if it created the file.
- Then `config.Load(path)` and proceed with the existing classify → `buildScanPlan` → `MergeScan` flow unchanged.

The `$HOME`-unset case (the only remaining `ResolveWriteTarget` error) still surfaces an error — that's an environment failure, not a missing-config condition.

### 3. `hop config scan --write` (`src/cmd/hop/config_scan.go` / `runConfigScan`)

Same pattern: when `--write` is set and the config is absent, auto-init the skeleton (announced), then perform the merge. The stdout-rendering default (no `--write`) does NOT auto-init — it never touches the file, so there is nothing to create. Confirm the precondition check in `runConfigScan` mirrors `add`'s.

### 4. `hop clone <url>` (`src/cmd/hop/clone.go`)

The auto-register path appends to a target group (default `default`) via `internal/yamled.AppendURL`. **Edge (Tentative — Assumption 9):** `AppendURL` returns `ErrGroupNotFound` when the named group is absent — and a fresh `repos: {}` skeleton has *no* `default` group. So `clone <url>` auto-init needs to create not just the file but also the **target group** before appending. Options to resolve at apply time:

- (a) `EnsureSkeleton` seeds an empty `default:` group when it creates the file (e.g. `repos:\n  default: []`), so `AppendURL` to `default` always finds a target. Simplest, but special-cases `default` into the skeleton — which slightly contradicts "minimal `repos: {}`".
- (b) `clone`'s auto-init path creates the file with `repos: {}`, then ensures the target group exists (creating it if absent — possibly via a new `yamled` ensure-group step) before `AppendURL`. More general (handles `--group <other>` too), more code.
- (c) `AppendURL` is extended to create the group when missing (changes its contract — likely too broad).

Default lean: **(b)** — keeps the skeleton truly minimal and handles `--group` uniformly, with `clone` owning the "ensure target group" step. This is the most significant implementation decision in the change and should be settled during planning.

### 5. Read-commands: improved error pointing at `hop add`, NO auto-init

`hop`, `hop ls`, `hop <name> where`, `hop config print` continue to call `config.Resolve()` and error on absence. This change **refines the not-found message** (which Change A left as `Run 'hop config init' to create one.`) to also point at the now-self-bootstrapping write path, so a fresh user's most natural first command is surfaced as the bootstrap:

```
hop: no hop.yaml found at /Users/you/.config/hop/hop.yaml. Run 'hop add <dir>' to register a repo (creates the config), or 'hop config init' for a starter.
```

Why this hint belongs in **Change B, not Change A**: under Change A alone, `hop add` does NOT auto-create — it still gates on an existing config (`config_add.go:78-86`). So advising "run `hop add` to create the config" is only TRUE once this change removes that gate (What Changes §2). Putting it in Change A would be a false hint — a user following it would run `hop add` and hit the same not-found gate. The honest ordering: Change A says `hop config init`; Change B upgrades the message to add `hop add` once auto-create exists. (Decision confirmed in discussion.)

> NOTE: This wording lives in `config.Resolve()` (the message Change A introduces). This change edits that one message. If both changes land close together, agree the final phrasing once to avoid editing it twice — but the *content* (point at `hop add`) is settled, not open.

### 6. `hop config init` unchanged

`init` continues to write the embedded annotated starter and refuse to overwrite. It remains the "give me the documented template" path. No behavior change here — only the *requirement* to run it before add/scan/clone is removed.

## Affected Memory

- `config/init-bootstrap`: (modify) Document the new auto-init-on-write behavior alongside explicit `hop config init`: which commands auto-init, the minimal-skeleton content, the `created <path>` announcement, and the read-vs-write split. Note that `init` writes the starter while auto-init writes the skeleton.
- `cli/subcommands`: (modify) Update the `hop add`, `hop config scan`, and `hop clone <url>` rows: the "requires an existing hop.yaml / config init pointer message" behavior is replaced by auto-init. Update exit-code notes (the missing-config exit-1 path on these write-commands goes away; replaced by create-then-proceed).
- `config/scan`: (modify) The bootstrap-then-populate workflow note (`hop config init` followed by `hop config scan`) should mention that `scan --write` now self-bootstraps.
- `config/search-order`: (modify, light) Cross-reference the auto-init behavior from the resolver doc (read commands error; write commands create).

## Impact

**Code:**
- `src/internal/config/` — new `EnsureSkeleton` helper (+ skeleton content constant; possibly an `internal/yamled` ensure-group helper for the clone path).
- `src/cmd/hop/config_add.go` — replace gate with auto-init.
- `src/cmd/hop/config_scan.go` — auto-init on `--write`.
- `src/cmd/hop/clone.go` — auto-init + ensure-target-group before `AppendURL`.
- `src/cmd/hop/where.go` (or wherever `Resolve()` callers route) — read-command error wording refinement (coordinated with Change A).
- Tests: new coverage for auto-init (file created, announced, skeleton content, idempotent when file exists), the clone empty-group edge, and the read-command-still-errors invariant. Existing `hop add`/`scan`/`clone` missing-config tests must flip from "errors" to "creates and proceeds."

**Docs:** `README.md` onboarding section (the "run `hop config init` first" step can be dropped from the first-run story); specs (`docs/specs/config-resolution.md` `hop config init` section gains an auto-init subsection) and memory updated at hydrate.

**CLI surface:** No new subcommands, no new flags (Constitution VI satisfied — this is behavior on existing commands, not new surface area). Behavior-widening on three existing commands.

**Constitution check:** III (Convention Over Configuration) — directly advanced (no mandatory setup step). II (No Database) — skeleton is created on disk, re-read each invocation; no cache/state store introduced. VI (Minimal Surface Area) — no new commands. Test Integrity — new tests assert the new spec behavior.

**Breaking-ish:** behavior change for scripts that relied on `hop add`/`scan --write`/`clone <url>` *failing* when no config exists (e.g., a guard script). Low risk — failing-on-absence was never a documented contract to depend on, and the new behavior is strictly more permissive. Worth a release-note mention.

## Open Questions

1. **Skeleton content** — `repos: {}` only, or also seed `config:\n  code_root: ~/code`? (Assumption 8; default lean: `repos: {}` only.)
2. **`clone <url>` empty-group handling** — which of options (a)/(b)/(c) in What Changes §4 to take? (Assumption 9; default lean: (b).) This is the change's most material implementation decision.
3. **Read-command error wording coordination with Change A** — if both land together, agree the final `config.Resolve()` message once to avoid editing it twice. (Assumption 10.)

## Clarifications

### Session 2026-06-05

| # | Action | Detail |
|---|--------|--------|
| 8 | Confirmed | Skeleton content is exactly `repos: {}` (no seeded `code_root`, no header comment) |
| 9 | Confirmed | `clone <url>` empty-group: option (b) — minimal skeleton + clone ensures target group before `AppendURL`; `AppendURL` contract unchanged |
| 11 | Changed | Final message string set to the full-hint variant: `Run 'hop add <dir>' to register a repo (creates the config), or 'hop config init' for a starter.` |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Depends on Change A (`260605-xgmu`); not implemented before A lands | Discussed — the fixed-path/unambiguous-absence property is the safety precondition for auto-init | S:95 R:60 A:90 D:90 |
| 2 | Certain | Auto-init writes a minimal skeleton, NOT the embedded starter content | Discussed — user explicitly chose minimal so add/scan/clone don't inject the unrelated hop.git seed | S:98 R:75 A:90 D:95 |
| 3 | Certain | Only write-commands auto-init (`add`, `scan --write`, `clone <url>`); read-commands keep erroring | Discussed — writers have something to write; readers conjuring empty config is worse UX | S:95 R:75 A:90 D:90 |
| 4 | Certain | Auto-init is announced via `created <path>` on stderr, not silent | Discussed — preserves the project's no-surprises posture | S:95 R:90 A:90 D:90 |
| 5 | Certain | `hop config init` is retained, unchanged, still writing the annotated starter | Discussed — explicit init and auto-init serve different intents; init stops being a precondition only | S:95 R:90 A:90 D:95 |
| 6 | Confident | New `EnsureSkeleton` helper lives in `internal/config` next to `WriteStarter`, mode 0644, never overwrites | Mirrors existing `WriteStarter` structure and mode rationale; absence-triggered create is the natural seam | S:80 R:75 A:85 D:75 |
| 7 | Confident | `scan` auto-inits only with `--write` (the stdout-render default never touches the file) | Direct from the command's existing semantics — no file write means nothing to bootstrap | S:85 R:85 A:90 D:85 |
| 8 | Certain | Skeleton content is exactly `repos: {}` only (no seeded `config.code_root`, no header comment) | Clarified — user confirmed; `code_root` defaults to `~`, so minimal is functional and avoids injecting unasked-for values | S:95 R:88 A:75 D:55 |
| 9 | Certain | `clone <url>` auto-init takes option (b): `EnsureSkeleton` writes `repos: {}`, then clone's path ensures the target group (`default` or `--group <name>`) exists — creating `<g>: []` if absent — before `AppendURL`. `AppendURL`'s `ErrGroupNotFound` contract is left UNCHANGED (so a typo'd `--group` against an existing config still errors). The ensure-group step is clone-owned (likely a new small `yamled` ensure-group helper). | Clarified — user confirmed (b) over (a) skeleton-seeds-default and (c) AppendURL-creates-group; (b) keeps the skeleton minimal, handles `--group` uniformly, and preserves AppendURL's typo-catching contract | S:95 R:55 A:70 D:45 |
| 10 | Certain | The read-command not-found message is refined in THIS change to point at `hop add <dir>` (creates the config) in addition to `hop config init` — and this hint belongs in Change B, not A, because `hop add` only auto-creates once B's gate-removal lands | Discussed — user asked for the `hop add` hint; confirmed it would be a false hint under Change A alone (add still gates pre-B), so Change B owns it | S:95 R:80 A:85 D:80 |
| 11 | Certain | Exact message string is: `hop: no hop.yaml found at <path>. Run 'hop add <dir>' to register a repo (creates the config), or 'hop config init' for a starter.` (full hint, both paths) | Clarified — user confirmed the full-hint variant over the terse hop-add-only and defer-to-apply options | S:95 R:90 A:80 D:60 |

11 assumptions (9 certain, 2 confident, 0 tentative, 0 unresolved).
