# Intake: Friendlier No-Match DX for Repo Resolution

**Change**: 260608-8v9h-friendly-no-match-resolution
**Created**: 2026-06-08
**Status**: Draft

## Origin

> fix: friendlier no-match DX for repo resolution. When a query matches zero repos (e.g. `hop looo`), suppress the fzf `--query` prefill so the user sees the full browsable repo list instead of a dead-end empty `0/13` picker; keep prefill for 2+ matches. Also fix the fzf exit-1 leak (screenshot showed `hop: fzf failed: exit status 1`) by folding fzf exit code 1 into `errFzfCancelled` alongside 130 — both mean "no repo selected, exit quietly".

Initiated conversationally via `/fab-discuss` → `/fab-new`. The user reported a poor DX after running `hop looo` (a query matching none of 13 repos) and shared two screenshots:

1. **Screenshot 1** — fzf opened with `looo` pre-filled as its `--query`, filtering the 13-repo list down to `0/13`: an empty, dead-end picker.
2. **Screenshot 2** — after dismissing that picker, hop printed the raw internal error `hop: fzf failed: exit status 1`, leaking an implementation detail as if something had broken.

Two decisions were made interactively before this intake (see Assumptions for SRAD grades):

- **No-match picker behavior** — the user chose "Open picker WITHOUT prefilled query": on 0 matches, still open fzf but omit `--query`, showing the full browsable list (`13/13`) instead of the empty `0/13`. This preserves the documented "clear query to browse all repos" intent (`docs/memory/cli/match-resolution.md` §"Why the full list … goes to fzf") while removing the dead-end. Prefill is **retained** for the 2+ match case, where narrowing is helpful.
- **fzf exit-1 handling** — the user chose "Treat as cancellation (exit 130)": fold fzf exit code 1 into `errFzfCancelled` alongside 130. Both mean "no repo selected, exit quietly," so no error message is printed.

## Why

**The problem.** hop's repo resolver (`resolveByName` in `src/cmd/hop/resolve.go`) treats a zero-match query and a 2+-match query identically: both "fall through to fzf" with the user's query passed as `--query`. When the query matches nothing, fzf filters the piped list down to zero rows and shows an empty `0/13` picker with the dead query still active. The user must *know* to clear the query (Ctrl-U) to recover the browsable list — an affordance that is documented in memory but invisible in the UI. Most users instead dismiss the picker, which surfaces the second problem.

**The leak.** The cancellation handler in `resolveByName` only special-cases fzf exit code **130** (Esc / Ctrl-C):

```go
if code, ok := proc.ExitCode(err); ok && code == 130 {
    return nil, errFzfCancelled
}
return nil, fmt.Errorf("hop: fzf failed: %w", err)   // ← exit 1 lands here
```

fzf returns exit code **1** when its list is exhausted with no selectable match (distinct from 130, user abort). That exit-1 path falls through to the generic `fzf failed: %w` wrap, leaking `hop: fzf failed: exit status 1` to the user. The screenshot confirms this: it reads `exit status 1`, not 130. So the no-match flow can produce a scary internal-error string for what is really "you didn't pick anything."

**Consequence if unfixed.** Every typo'd or non-matching query (`hop looo`, `hop xyz`) is a dead end: an empty picker followed by a fake error. This compounds the broader "hop feels buggy" perception. It also undermines Constitution Principle III (Convention Over Configuration) in spirit — the tool should gracefully guide, not punish, a near-miss query.

**Why this approach over alternatives.** Three options were weighed during discussion:

1. *Friendly error, no picker* — print `hop: no repo matches 'looo'` and never open fzf on 0 matches. Cleanest, but discards the documented "browse all repos" affordance the bash original and Go port both preserve.
2. *Open picker WITHOUT prefilled query* **(chosen)** — keeps the browse affordance, kills the dead-end empty picker.
3. *Keep picker, just fix the error* — minimal, fixes only Screenshot 2, leaves the empty `0/13` picker (Screenshot 1) intact.

Option 2 threads the needle: it honors the existing design intent (browse-the-full-list-on-zero-match) while removing the confusing empty state, and it's a ~4-line localized change.

## What Changes

All changes are localized to `src/cmd/hop/resolve.go` plus tests. No new flags, subcommands, or config (Constitution Principles III & VI preserved).

### Change Area 1 — Suppress `--query` prefill on zero matches

In `resolveByName`, the pre-fzf match step currently:

```go
if query != "" {
    candidates := rs.MatchOne(query)
    if len(candidates) == 1 {
        return &candidates[0], nil
    }
}

pickerLines := buildPickerLines(rs)
selected, err := fzf.Pick(context.Background(), pickerLines, query)
```

The decision to prefill keys off the match count. After the change, the query passed to `fzf.Pick` is suppressed (set to `""`) when there were **0** matches, and retained when there were **2+**:

```go
fzfQuery := query
if query != "" {
    candidates := rs.MatchOne(query)
    if len(candidates) == 1 {
        return &candidates[0], nil
    }
    if len(candidates) == 0 {
        fzfQuery = ""   // 0 matches → browse full list, no dead-end prefill
    }
    // 2+ matches → keep query prefilled
}

pickerLines := buildPickerLines(rs)
selected, err := fzf.Pick(context.Background(), pickerLines, fzfQuery)
```

`fzf.Pick` already omits the `--query` flag entirely when its `query` argument is `""` (`internal/fzf/fzf.go::buildArgs` — verified: `if query != "" { args = append(args, "--query", query) }`). So passing `""` produces the full browsable list with no code change in `internal/fzf`.

**Behavior matrix after the change:**

| Query state | `MatchOne` result | fzf invoked? | `--query` passed? | Picker shows |
|---|---|---|---|---|
| empty (bare `hop`) | n/a (skipped) | yes | no (already) | full list `13/13` *(unchanged)* |
| `hop looo` (no match) | 0 candidates | yes | **no (changed — was `looo`)** | full list `13/13` *(was empty `0/13`)* |
| `hop web` (2+ match) | 2+ candidates | yes | yes (`web`) | narrowed, non-empty *(unchanged)* |
| `hop hop` (1 match) | 1 candidate | no | n/a | resolves directly, no fzf *(unchanged)* |

### Change Area 2 — Fold fzf exit code 1 into cancellation

The cancellation handler is widened so both exit 130 (Esc/Ctrl-C) and exit 1 (no selectable match) map to `errFzfCancelled`:

```go
// fzf exits 130 on Esc/Ctrl-C and 1 when no match is selectable — both mean
// "no repo chosen". Treat both as a quiet cancellation; any other code is a
// real failure.
if code, ok := proc.ExitCode(err); ok && (code == 130 || code == 1) {
    return nil, errFzfCancelled
}
return nil, fmt.Errorf("hop: fzf failed: %w", err)
```

`errFzfCancelled` already maps to exit code 130 via `translateExit` (and to `shimResolveErr` on the `--shim-plan` path). With Change Area 1 in place, exit 1 should rarely fire (the list piped to fzf is always the full non-empty repo list), but folding it in removes the leak unconditionally and is the behavior the user chose.

### Change Area 3 — Tests

Existing `resolve_test.go` tests deliberately *avoid* triggering fzf (see line ~231: "resolveByName triggers fzf — which we want to avoid in tests"), and `resolve.go` calls `fzf.Pick` **directly** rather than through a swappable package-level seam (unlike `config_rm.go`, which uses a `pickOne = fzf.Pick` var seam). Two test layers:

1. **`internal/fzf/fzf_test.go`** — already fakes at the package-internal `runInteractive` var (lines ~42-44). Add/confirm a `buildArgs` case asserting `--query` is omitted when `query == ""`. (A `buildArgs("")` test likely already exists for the bare-`hop` path — verify and reuse.)

2. **`resolve_test.go`** — to assert the *resolve-level decision* (0 matches → empty query reaches Pick; 2+ matches → query reaches Pick; exit 1 → `errFzfCancelled`), introduce a swappable seam in `resolve.go` mirroring `config_rm.go`'s precedent (decided — see Clarifications §6):

   ```go
   // pickRepo is the seam for the interactive fzf selection in resolveByName.
   // Defaults to fzf.Pick; tests swap it to capture the query argument and
   // inject exit-code errors without spawning a real fzf. Mirrors the
   // pickOne = fzf.Pick seam in config_rm.go.
   var pickRepo = fzf.Pick
   ```

   `resolveByName` MUST call `pickRepo(context.Background(), pickerLines, fzfQuery)` instead of `fzf.Pick(...)` directly. Tests swap `pickRepo` (saving and restoring the original via `defer`) to: (a) capture the `query` argument and assert it is `""` for a zero-match query and equals the input for a 2+-match query; (b) return a fake `*exec.ExitError` with code 1 and assert the result is `errFzfCancelled` (not a `fzf failed` wrap). The fake exit error is constructed so `proc.ExitCode` extracts code 1 (e.g., via a real `exec.Command("false").Run()` error or an equivalent `*exec.ExitError`). This is a small refactor following the existing in-package `pickOne` pattern — chosen over testing only at the `internal/fzf` level (which would leave the 0-vs-2+ decision and the exit-1 mapping unverified) and over a pure-helper extraction (which wouldn't cover the exit-1 mapping).

### Inherited consequences (no extra code)

- **`resolveTargets` (`pull`/`push`/`sync` rule 3)** calls `resolveByName` for substring matches, so `hop looo pull` inherits the no-prefill browse and the exit-1 fix automatically.
- **Worktree-suffix LHS** (`hop looo/feat-x`) resolves the LHS via the same `resolveByName` recursion, so a no-match LHS inherits the no-prefill picker. The RHS worktree match is exact (not fzf) and is unaffected.
- **`config_rm.go`** has its *own* fzf path (`pickOne`, line ~229 with its own `fzf failed` wrap). It is **out of scope** — the user's report and the chosen fix target repo resolution (`resolveByName`), not the `hop rm`/`hop config rm` interactive removal picker. (Noted as a non-goal; a follow-up could apply the same exit-1 treatment there for consistency.)

## Affected Memory

- `cli/match-resolution`: (modify) Update §"Cancellation and missing fzf" to record that fzf exit **1** (no selectable match) now maps to `errFzfCancelled` alongside 130. Update §"Why the full list (not the filtered subset) goes to fzf" — and add to the §"Algorithm" step 4 / step 5 description — that a **zero-match** query now suppresses the `--query` prefill (passes `""` to `fzf.Pick`) so the picker opens on the full browsable list rather than the user's dead query; the 2+-match case still prefills.

## Impact

- **Code**: `src/cmd/hop/resolve.go` (`resolveByName` — prefill decision + cancellation handler; possibly a new `pickRepo` seam var). ~4 lines of behavior change + 1 seam var.
- **Tests**: `src/cmd/hop/resolve_test.go` (new cases for prefill suppression + exit-1 cancellation), `src/internal/fzf/fzf_test.go` (confirm `buildArgs("")` omits `--query`).
- **No changes** to: `internal/fzf/fzf.go` (already omits `--query` on empty), `internal/proc` (`ExitCode` already extracts the code), CLI surface (no new flags/subcommands), config schema, or the `--shim-plan` classifier (it consumes `errFzfCancelled` unchanged via `shimResolveErr`).
- **Exit codes**: A dismissed/no-match picker exits **130** (was: leaked `exit status 1` from the generic wrap; the process exit was 1). This is a deliberate, documented change — both mean "no selection," and 130 is hop's existing cancellation code.

## Open Questions

- Should `config_rm.go`'s separate fzf path (`hop rm` / `hop config rm` interactive picker, with its own `%s: fzf failed: %w` wrap at line ~229) receive the same exit-1 treatment for consistency? Out of scope for this change, but a natural follow-up. (Leaning: defer to a separate change to keep this one minimal and focused on the reported path.)

## Clarifications

### Session 2026-06-08

| # | Question | Resolution |
|---|----------|------------|
| 6 | How to make the prefill decision + exit-1 mapping unit-testable in resolve.go? | Add a swappable `var pickRepo = fzf.Pick` seam in resolve.go; `resolveByName` calls `pickRepo(...)`. Tests swap it to capture the query arg (assert `""` on 0-match, input on 2+) and inject a fake `*exec.ExitError{code:1}` to assert `errFzfCancelled`. Mirrors `config_rm.go`'s existing `pickOne = fzf.Pick` precedent — no new in-package pattern. |

### Session 2026-06-08 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 3 | Confirmed | — |
| 4 | Confirmed | — |
| 5 | Confirmed | — |
| 7 | Confirmed | — |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | On 0 matches, suppress `--query` prefill (pass `""` to `fzf.Pick`) so the full browsable list shows; retain prefill for 2+ matches | Decided interactively — user chose "Open picker WITHOUT prefilled query" over the no-picker and fix-error-only alternatives | S:98 R:80 A:90 D:95 |
| 2 | Certain | Fold fzf exit code 1 into `errFzfCancelled` alongside 130 (quiet exit, no error message) | Decided interactively — user chose "Treat as cancellation (exit 130)" over a distinct no-match message | S:98 R:85 A:90 D:95 |
| 3 | Certain | Change is localized to `resolveByName` in `src/cmd/hop/resolve.go`; `internal/fzf` and `internal/proc` need no changes | Clarified — user confirmed | S:95 R:75 A:88 D:85 |
| 4 | Certain | `resolveTargets` (pull/push/sync) and worktree-suffix LHS inherit the fix with no extra code | Clarified — user confirmed | S:95 R:75 A:85 D:80 |
| 5 | Certain | `config_rm.go`'s separate fzf picker is out of scope (different path, own `fzf failed` wrap) | Clarified — user confirmed | S:95 R:70 A:82 D:78 |
| 6 | Certain | Introduce a swappable `pickRepo = fzf.Pick` seam var in resolve.go to unit-test the prefill decision and exit-1 mapping; mirrors `config_rm.go`'s `pickOne` precedent | Clarified — user confirmed seam approach over internal/fzf-only testing and pure-helper extraction | S:95 R:75 A:65 D:55 |
| 7 | Certain | A dismissed/no-match picker exits 130 (hop's existing cancellation code), accepted as a deliberate behavior change | Clarified — user confirmed | S:95 R:78 A:80 D:75 |

7 assumptions (7 certain, 0 confident, 0 tentative, 0 unresolved).
