# Intake: Add --skip-brew-update flag to update command

**Change**: 260531-dx50-skip-brew-update-flag
**Created**: 2026-05-31
**Status**: Draft

## Origin

This change originates from a one-shot, fully-specified contract (no conversational
back-and-forth). It is part of a **cross-toolkit rollout**: the identical flag
(`--skip-brew-update`) is being added to 6 sibling tools, so the flag name, semantics,
and default behavior are fixed by an external contract and are NOT open to local
reinterpretation. This repo (hop) is one of the 6.

> Add a boolean `--skip-brew-update` flag to the `update` command. CONTRACT
> (cross-toolkit, identical in 6 tools): flag name EXACTLY `--skip-brew-update`. When
> set, skip ONLY the internal `brew update` tap-metadata refresh. Everything else
> unchanged: `brew info` version check, up-to-date short-circuit, `brew upgrade`.
> Default (absent) = current behavior exactly preserved. THIS REPO (hop): update logic
> in `src/internal/update/update.go` (func `Run`, the `brew update` call ~L64); wire a
> real cobra bool flag in `cmd/hop/update.go` and pass it into `Run()`. Thread
> `skipBrewUpdate bool` through `Run()`. Preserve the intentional brew output routing.
> Match existing subprocess convention (do NOT refactor). Add a test asserting
> `--skip-brew-update` omits `brew update` but still runs `brew upgrade`, following the
> repo test pattern. Build + run the update package tests before the PR.

PR target: an OPEN PR (do NOT merge) titled exactly
`feat: add --skip-brew-update flag to update command`.

## Why

1. **Problem.** `hop update` unconditionally runs `brew update --quiet` (a full
   tap-metadata refresh across *every* installed tap) before checking and upgrading
   hop. That refresh is the slow, network-heavy part of the flow — frequently 5–30s —
   and is wasted work when the caller already knows their tap metadata is fresh (e.g.
   CI that just ran `brew update`, or a batch tool that updates 6 binaries in sequence
   and only needs one metadata refresh, not six).

2. **Consequence if not fixed.** Each of the 6 sibling tools redundantly refreshes all
   tap metadata on every self-update, multiplying a slow global operation. There is no
   way to opt out short of not running the tool's `update` at all, which defeats the
   purpose.

3. **Why this approach.** A single boolean opt-out flag is the minimal surface-area
   addition (Constitution VI): it is a flag on an existing subcommand, not a new
   subcommand. The flag is *additive and default-off* — absent flag reproduces today's
   behavior byte-for-byte. The cross-toolkit contract mandates the exact name and
   semantics, so there is no design latitude on the interface; the only local work is
   wiring and a conformance test. Skipping `brew update` is safe because the subsequent
   `brew info --json=v2` still reports the version brew currently knows about; with a
   skipped refresh that may be a slightly stale "latest", which is precisely the
   trade-off the caller opts into when they pass the flag.

## What Changes

### 1. `update.Run` signature — add `skipBrewUpdate bool`

`src/internal/update/update.go`. The current signature is:

```go
func Run(currentVersion string, out, errOut io.Writer) error
```

It becomes:

```go
func Run(currentVersion string, skipBrewUpdate bool, out, errOut io.Writer) error
```

The new parameter gates **only** the `brew update --quiet` block (currently ~L63–L72):

```go
ctx, cancel := context.WithTimeout(context.Background(), brewUpdateTimeout)
_, err := proc.Run(ctx, "brew", "update", "--quiet")
cancel()
if err != nil {
    if errors.Is(err, proc.ErrNotFound) {
        fmt.Fprintln(errOut, "hop update: brew not found on PATH.")
        return err
    }
    return fmt.Errorf("brew update failed: %w", err)
}
```

becomes guarded:

```go
if !skipBrewUpdate {
    ctx, cancel := context.WithTimeout(context.Background(), brewUpdateTimeout)
    _, err := proc.Run(ctx, "brew", "update", "--quiet")
    cancel()
    if err != nil {
        if errors.Is(err, proc.ErrNotFound) {
            fmt.Fprintln(errOut, "hop update: brew not found on PATH.")
            return err
        }
        return fmt.Errorf("brew update failed: %w", err)
    }
}
```

**Everything else is untouched**: the `isBrewInstalled()` short-circuit, the
`Current version:` / `Checking for updates...` wrapper lines, `brewLatestVersion()`
(which calls `brew info --json=v2`), the `normalizeVersion` equality short-circuit
(`Already up to date`), and the `proc.RunForeground(... "brew", "upgrade", brewFormula)`
call. The intentional output routing documented in the `Run` doc comment (wrapper
messages → `out`/`errOut`; subprocess streams → `internal/proc`) is preserved verbatim —
the guard introduces no new writes to `out`/`errOut`.

### 2. Cobra bool flag in `cmd/hop/update.go`

Wire a real cobra bool flag (matching the repo convention, e.g. `clone.go`/`sync.go`
which declare a local `var` inside the constructor and register via
`cmd.Flags().BoolVar(...)`). The current constructor returns a bare `&cobra.Command{}`
literal; it changes to capture a local var first, then register the flag and pass it
into `Run`:

```go
func newUpdateCmd() *cobra.Command {
    var skipBrewUpdate bool
    cmd := &cobra.Command{
        Use:   "update",
        Short: "self-update the hop binary via Homebrew",
        Args:  cobra.NoArgs,
        RunE: func(cmd *cobra.Command, args []string) error {
            err := update.Run(version, skipBrewUpdate, cmd.OutOrStdout(), cmd.ErrOrStderr())
            if errors.Is(err, proc.ErrNotFound) {
                return errSilent
            }
            return err
        },
    }
    cmd.Flags().BoolVar(&skipBrewUpdate, "skip-brew-update", false,
        "skip the internal `brew update` tap-metadata refresh (brew info check and brew upgrade still run)")
    return cmd
}
```

Flag name is EXACTLY `--skip-brew-update`; default `false`; `cobra.NoArgs` is retained.

### 3. Test: `--skip-brew-update` omits `brew update` but still runs `brew upgrade`

`src/internal/update/update_test.go`. The contract requires a test asserting the
flag's effect on subprocess invocation. The existing test pattern only exercises the
non-brew path because there is **no injection seam** today: `proc.Run` and
`proc.RunForeground` call `exec.CommandContext` with a hardcoded `"brew"` binary, and
`Run` short-circuits on `isBrewInstalled()` (false unless the binary lives under
`/Cellar/`).

To assert "omits `brew update`, still runs `brew upgrade`" without refactoring the
subprocess convention or touching `internal/proc`, the implementation will introduce a
**minimal, unexported, test-only seam in the `update` package**: a package-level
function variable that wraps the actual subprocess call, defaulting to the real
`internal/proc` invocation in production and overridable from the test. This keeps
production behavior identical (the default points at `proc.Run`/`proc.RunForeground`)
and does NOT change how subprocesses are run — it only makes the *recording of which
brew subcommands were invoked* observable to a test. The exact shape of this seam is a
Tentative decision to be finalized at spec/plan time (see Open Questions #1).

The test asserts both directions:
- With `skipBrewUpdate == true`: the recorded invocations contain `brew upgrade ...`
  and do NOT contain `brew update ...`.
- (Baseline, default behavior preserved) With `skipBrewUpdate == false`: the recorded
  invocations contain `brew update ...` (guards against silently dropping the refresh).

Following the repo test pattern: table-light, `testing`-only (no external deps),
`bytes.Buffer` for `out`/`errOut`, and restoring any overridden package var via
`t.Cleanup`/`defer`.

## Affected Memory

- `cli/subcommands`: (modify) document the new `--skip-brew-update` flag on `hop update`
  (flag name, default, and that it skips only the tap-metadata refresh).

## Impact

- **Code**: `src/internal/update/update.go` (signature + guarded block + minimal
  test seam), `src/cmd/hop/update.go` (flag wiring), `src/internal/update/update_test.go`
  (new test). Only caller of `update.Run` is `cmd/hop/update.go`, so the signature
  change has a single call site to update.
- **APIs / dependencies**: no new dependencies. No change to `internal/proc`. No change
  to other subcommands. CLI surface gains one additive flag.
- **Build / test**: `go build ./...` and `go test ./internal/update/...` (run from
  `src/`) must pass before the PR.
- **Constitution**: I (Security First) — subprocess args stay explicit slices via
  `internal/proc`; the seam defaults to that. IV (Wrap, Don't Reinvent) — brew is still
  wrapped, not reimplemented. VI (Minimal Surface Area) — a flag, not a new subcommand.

## Open Questions

- Exact shape of the test seam: a package-level `var runBrew = func(...)` indirection in
  `update.go` vs. a small recorder injected only in tests. Both keep production routed
  through `internal/proc`; the choice is internal and reversible. (Resolvable at
  spec/plan — does not block intake.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Flag name is EXACTLY `--skip-brew-update`, boolean, default `false` | Fixed by cross-toolkit contract (identical in 6 tools); no local latitude | S:100 R:90 A:100 D:100 |
| 2 | Certain | When set, skip ONLY `brew update --quiet`; `brew info` check, up-to-date short-circuit, and `brew upgrade` all still run | Explicit in contract; matches the single `brew update` call at update.go ~L64 | S:100 R:85 A:95 D:100 |
| 3 | Certain | Default (flag absent) preserves current behavior exactly | Contract states "Default (absent) = current behavior exactly preserved"; guard is `if !skipBrewUpdate` wrapping the existing block unchanged | S:100 R:90 A:100 D:100 |
| 4 | Certain | Thread `skipBrewUpdate bool` through `update.Run`; new signature `Run(currentVersion string, skipBrewUpdate bool, out, errOut io.Writer)` | Contract names `Run` and the threading explicitly; single call site (cmd/hop/update.go) | S:95 R:80 A:95 D:90 |
| 5 | Certain | Wire a real cobra `BoolVar` flag in `cmd/hop/update.go` matching repo convention | Contract says "wire a real cobra bool flag"; clone.go/sync.go show the `BoolVar` pattern | S:95 R:85 A:100 D:95 |
| 6 | Confident | Preserve the intentional brew output routing (wrapper msgs → out/errOut; subprocess streams → internal/proc); do NOT refactor | Contract says "Preserve the intentional brew output routing" and "do NOT refactor"; the guard adds no new writes | S:90 R:75 A:90 D:85 |
| 7 | Confident | PR title EXACTLY `feat: add --skip-brew-update flag to update command`; OPEN PR, do NOT merge; no Co-Authored-By / "Generated with Claude" footer | Stated verbatim in contract | S:100 R:95 A:90 D:95 |
| 8 | Confident | `cli/subcommands` memory updated to document the flag during hydrate | Spec-level CLI behavior changes; memory index lists cli/subcommands as the surface doc | S:80 R:80 A:85 D:80 |
| 9 | Tentative | Test introduces a minimal unexported test-only seam (package-level func var) in the `update` package so the test can assert which brew subcommands ran, without refactoring `internal/proc` | No injection seam exists today (hardcoded "brew", isBrewInstalled short-circuit); "do NOT refactor" constrains the approach. A package-local var is the least-invasive, reversible option; exact shape finalized at spec/plan | S:55 R:65 A:60 D:50 |

9 assumptions (5 certain, 3 confident, 1 tentative, 0 unresolved).
