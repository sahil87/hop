# Intake: shll update/version/shell-init Standards Conformance

**Change**: 260719-g94r-shll-update-version-shell-init-conformance
**Created**: 2026-07-20

## Origin

One-shot `/fab-new` invocation:

> Bring this repo into conformance with the shll toolkit 'update', 'version', and 'shell-init' standards (docs/site/standards/update.md, version.md, and shell-init.md in the shll repo, or https://shll.ai/standards). Audit the update, --version, and shell-init subcommands against every MUST/SHOULD in all three standards, fix any gaps found, and add/update tests pinning the fixed behavior. If the audit finds the repo is already fully conformant with no code changes needed, skip /git-pr entirely — do not open an empty PR.

The standards were read at intake time via `shll standards update`, `shll standards version`, and `shll standards shell-init` (shll 2026-07 text; canonical source `sahil87/shll` `docs/site/standards/`). The intake-time audit **found gaps** (one MUST violation plus test-coverage gaps enumerated below), so the "skip /git-pr if fully conformant" directive is moot — this change proceeds through the normal pipeline including PR. The conditional is recorded here for traceability only.

Constitution § Toolkit Standards binds this repo to these standards without further amendment.

## Why

1. **Pain point / MUST violation**: `hop update` wraps `brew upgrade` in a **120-second hard timeout** (`brewUpgradeTimeout = 120 * time.Second` in `src/internal/update/update.go`), executed via `internal/proc`'s `exec.CommandContext` — whose default context-cancel behavior is **SIGKILL**. The update standard's brew-handling clause prohibits exactly this: "MUST NOT send `SIGKILL` to a package-manager subprocess mid-transaction" and "MUST NOT impose a short hard timeout on `brew upgrade`". The standard cites the motivating incident verbatim: on 2026-07-19 a stalled `api.github.com` call inside `brew upgrade` exceeded a wrapper's 120-second hard kill; the SIGKILL landed between `brew unlink` and `brew link` and corrupted the keg, leaving `zsh: permission denied: <tool>`. hop's current code is that wrapper pattern, byte for byte the same 120s bound. `brew update --quiet` is likewise under a 30s hard-SIGKILL cap (`brewUpdateTimeout`), and brew update mutates tap git state — same clause applies.
2. **Consequence of not fixing**: any machine with a slow network moment during `hop update` (or `shll update`, which delegates to it) can be left with a half-installed, broken hop keg. This is the single most damaging failure the update standard exists to prevent, and it is silent until the binary stops resolving.
3. **Test gaps**: the standards' "Verifying conformance" checklists ask each tool to pin the contracts with tests. Today: `update --help`'s literal `--skip-brew-update` substring (a frozen textual contract — shll discovers it via `strings.Contains`) is not pinned; `TestIntegrationVersion` only asserts non-empty output, not exit 0 + first-line token shape + stdout placement; the shell-init usage-error tests assert exit 2 and the stderr message but not that **stdout stays empty**; and only bash has an eval-the-output-in-a-subshell test — zsh (the primary documented shell) has none.
4. **Why this approach**: fix the one behavioral violation at its root (subprocess cancellation semantics in `internal/proc` + timeout policy in `internal/update`), and pin every already-conformant behavior with tests so the contracts can't regress. No behavior changes to `--version` or `shell-init` — the audit found them conformant (details per section below).

## What Changes

### Intake-time audit results (full MUST/SHOULD sweep)

**update standard** (`shll standards update`):

| Clause | Status |
|---|---|
| MUST expose `update` subcommand, in-place upgrade | ✅ `newUpdateCmd` → `update.Run` |
| MUST work standalone | ✅ |
| MUST advertise literal `--skip-brew-update` in `update --help` | ✅ behavior (cobra flag renders the literal name) — ❌ **no test pins the substring** |
| MUST honor `--skip-brew-update` (skip internal `brew update`) | ✅ (`TestRunSkipBrewUpdate` pins it) |
| MUST exit 0 on success incl. already-up-to-date | ✅ (`Run` returns nil on up-to-date and non-brew install) |
| MUST exit non-zero only on genuine failure | ✅ |
| MUST NOT SIGKILL a package-manager subprocess mid-transaction | ❌ **VIOLATED** — `proc.Run`/`proc.RunForeground` use `exec.CommandContext` default cancel (SIGKILL) with 30s/120s deadlines on `brew update`/`brew upgrade` |
| MUST NOT impose a short hard timeout on `brew upgrade` | ❌ **VIOLATED** — `brewUpgradeTimeout = 120s` |
| SHOULD: any bound generous, SIGTERM + grace | ❌ (follows from above) |
| SHOULD self-update only when brew-installed, via `/Cellar/` in resolved `os.Executable()` | ✅ (`isBrewInstalled`, clear degrade message) |
| Naming/release alignment (repo == roster name == `sahil87/tap/hop` leaf == binary; `v{semver}` tags) | ✅ (no rename in flight) |

**version standard** (`shll standards version`):

| Clause | Status |
|---|---|
| MUST support `--version`, exit 0, version on stdout | ✅ cobra `rootCmd.Version` → `hop version <v>` |
| MUST respond within 2s, no network I/O | ✅ purely local |
| Version token on first non-empty line (canonical `<tool> version vX.Y.Z` shape) | ✅ cobra default single line satisfies both `versionPrefixRE` and token rule |
| Binary name on PATH == tool name | ✅ `hop` |
| Verify checklist: minimal test pinning exit 0 + line-1 shape | ❌ **test gap** — `TestIntegrationVersion` only asserts non-empty `CombinedOutput` |

**shell-init standard** (`shll standards shell-init`):

| Clause | Status |
|---|---|
| MUST emit eval-safe shell source on stdout, exit 0, for `zsh` and `bash` | ✅ `posixInit` + per-shell cobra completion |
| Diagnostics to stderr only | ✅ (`errExitCode.msg` printed to stderr by `translateExit`) |
| On any failure exit non-zero | ✅ (write/completion errors return error → exit 1) |
| Missing/unsupported shell → exit 2, usage on stderr, **stdout empty** | ✅ behavior (RunE returns before any stdout write) — ❌ **tests don't assert stdout emptiness** |
| Verify checklist: test that evals output in a subshell and asserts clean exit | ⚠️ bash covered indirectly (integration test evals the bash shim and uses it); **zsh has no eval/syntax test** |

Net scope: **one behavioral fix** (brew-handling safety) + **test additions** pinning the rest. No changes to `--version` or `shell-init` runtime behavior, no help-text changes, no README/docs/site changes.

### 1. Brew-handling safety (`src/internal/proc`, `src/internal/update`) — the behavioral fix

- **`brew upgrade` (foreground): drop the deadline entirely.** Replace the 120s `context.WithTimeout` with `context.Background()` for the `brewRunForeground(…, "brew", "upgrade", brewFormula)` call. Rationale: it runs with inherited stdio — the user watches brew's own progress and can Ctrl-C (SIGINT reaches the foreground process group and brew handles it; that is user-initiated interruption, not a wrapper-imposed hard kill). This is the standard's preferred no-bound posture; no `HOMEBREW_NO_GITHUB_API=1` workaround needed once no bound exists.
- **`brew update --quiet` (captured/quiet): generous graceful bound.** This call has no visible progress, so an unbounded hang would look like a frozen `hop update`. Give it a **10-minute** bound that terminates via **SIGTERM + grace**, never SIGKILL-first: add graceful-cancel support to `internal/proc` (the only package allowed to import `os/exec`, Constitution I) — set `cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }` and `cmd.WaitDelay` (grace period, e.g. 20s) so Go only escalates to kill after brew had a real chance to unwind. Shape: either a new `proc.RunGraceful(ctx, …)` or an option on `proc.Run` — pick whichever reads best against proc's existing five-function surface at apply time. Unix-only signal use is fine: Windows is unsupported (Constitution § Cross-Platform Behavior).
- **`brew info --json=v2` (read-only query): keep a bounded call.** It is not a mutation/transaction, so the clause doesn't bind it; keep `brewInfoTimeout` (30s) but route it through the same graceful-cancel path for consistency (SIGTERM first costs nothing).
- Remove the now-dead `brewUpgradeTimeout` const; keep/rename the others to reflect the new policy.

### 2. Tests pinning the update contract (`src/cmd/hop/update_test.go`, `src/internal/update/update_test.go`)

- **Help substring**: `hop update --help` output contains the exact literal `--skip-brew-update` (substring check, mirroring shll's `strings.Contains` discovery).
- **No hard deadline on `brew upgrade`**: via the existing test seams (`brewRunForeground` swap), assert the context passed to the upgrade call has **no deadline** (`ctx.Deadline()` second return false). This pins the MUST at the seam where it lives.
- **Graceful bound on `brew update`**: assert the update-path context carries the generous deadline (and, at the proc level, that the graceful runner sets `Cancel`/`WaitDelay` — pin at whichever seam is testable without invoking real brew).
- Existing tests (exit-0 wiring, skip-brew-update honor, non-brew degrade) stay as-is.

### 3. Tests pinning the version contract (`src/cmd/hop/integration_test.go` or a sibling)

Strengthen `TestIntegrationVersion` (built-binary integration path) to pin the standard's verify checklist:

- exit code 0;
- version written to **stdout** (capture stdout separately, not `CombinedOutput`);
- the **first non-empty line** matches shll's parse: either the token regex `v?\d+(\.\d+)*([.-][\w.+-]+)?` or the `<word> version <rest>` prefix shape (the dev build emits `hop version dev` — the prefix shape; a release build emits `hop version vX.Y.Z`). Assert the prefix shape `hop version <nonempty>` and, when the trailing field looks like a version, the token regex — so the test passes for both `dev` and tagged builds.

### 4. Tests pinning the shell-init contract (`src/cmd/hop/shell_init_test.go`, integration)

- **Usage-error stdout emptiness**: extend `TestShellInitMissingShell` / `TestShellInitUnsupportedShell` to assert the captured stdout buffer is empty (exit 2 + stderr message already pinned).
- **zsh eval test**: integration test that `eval`s `hop shell-init zsh` output in a `zsh` subshell (`zsh -f -c 'eval "$(hop shell-init zsh)"'` — or eval + a trivial `hop <name> where` round-trip mirroring the existing bash test) and asserts clean exit. **Skip via `t.Skip` when `zsh` is not on PATH** (CI runners may lack it — same portability discipline as the pinned-default-branch lesson).
- **bash**: the existing integration test already evals the bash shim and exercises it; add an explicit clean-exit eval assertion only if it falls out naturally — do not duplicate coverage.

## Affected Memory

- `cli/subcommands`: (modify) `hop update` brew-handling semantics — no hard timeout on `brew upgrade` (foreground, user-interruptible), generous SIGTERM+grace bound on the internal `brew update`, rationale = shll update standard's brew-handling clause.
- `architecture/wrapper-boundaries`: (modify) `internal/proc` gains graceful-cancel (SIGTERM + `WaitDelay`) subprocess termination alongside the existing default-kill runners.

## Impact

- `src/internal/proc/proc.go` (+ test): graceful-cancel runner (new function or option); no changes to existing call sites outside update.
- `src/internal/update/update.go` (+ test): timeout policy per § What Changes 1.
- `src/cmd/hop/update_test.go`, `src/cmd/hop/integration_test.go`, `src/cmd/hop/shell_init_test.go`: new/strengthened pins.
- No CLI surface, help-text, README, or docs/site changes. No dependency changes.
- `shll update` / `shll version` / `shll shell-init` consumer behavior: unchanged by design — this change only tightens hop's producer side.

## Open Questions

*(none — zero Unresolved decisions; see Assumptions)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Timeout policy: no bound on foreground `brew upgrade`; 10-min SIGTERM+grace bound on captured `brew update`; keep bounded read-only `brew info` | Standard prefers no/short-free bounds and names SIGTERM+grace as the sanctioned fallback; foreground vs. captured split follows from which calls have user-visible progress. Two valid shapes (no bound everywhere vs. graceful bounds), clear front-runner | S:70 R:80 A:75 D:60 |
| 2 | Certain | Graceful-cancel lives in `internal/proc` (Cancel=SIGTERM + WaitDelay), not at call sites | Constitution I: only `internal/proc` may import `os/exec`; deterministic placement | S:65 R:85 A:90 D:75 |
| 3 | Certain | `--version` and `shell-init` need **test-only** work — runtime behavior already conforms | Grounded in intake-time code audit of `main.go`/`root.go`/`shell_init.go` against each clause (tables above) | S:80 R:90 A:85 D:85 |
| 4 | Confident | zsh eval test skips (`t.Skip`) when zsh is absent from PATH | CI portability precedent (runners lack zsh more often than bash); standard asks for the test, not for a hard CI dependency | S:60 R:90 A:85 D:70 |
| 5 | Confident | Naming/release-alignment clauses need no work in this change | repo, roster name, formula leaf (`sahil87/tap/hop`), and binary are already the one string "hop"; releases tagged `v*` per Constitution V; no rename in flight | S:70 R:85 A:80 D:75 |

5 assumptions (2 certain, 3 confident, 0 tentative, 0 unresolved).
