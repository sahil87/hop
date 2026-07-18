# Intake: `hop rm <name>` Consent Gate

**Change**: 260717-clc4-rm-consent-gate
**Created**: 2026-07-18

## Origin

> [clc4] 2026-07-18: `hop rm` (and the hidden `hop config rm` alias) mutate `hop.yaml` — a destructive registry write — with NO consent gate on the direct `hop rm <name>` path (no `--yes`, no confirmation prompt), deferred from the toolkit-standards conformance audit (260717-fcvp). Toolkit principle №5 says destructive writes MUST require explicit consent per №1 (interactive confirm on a TTY, `--yes`/`-y` for automation, fast refusal when neither). The audit added `--dry-run` (the small additive half) but deferred consent because `hop rm <name>` is DOCUMENTED as immediate/non-interactive (see docs/memory/cli/subcommands.md: "no confirmation prompt") and the picker path is already interactive consent — adding a mandatory prompt/`--yes` changes that contract and needs a design decision.

Invoked via `/fab-new clc4` (backlog ID). Interactive mode: the deferred design decision — *does `hop rm <name>` warrant consent at all?* — was put to the user as a three-way choice (full consent gate / documented exemption / prompt-on-fuzzy-match-only). **The user chose the full consent gate**, including the shown preview: TTY prompt displaying the resolved match, `--yes`/`-y` for automation, no-TTY refusal naming the flag with exit 3. The competing options were rejected: the documented exemption leaves the fuzzy-match footgun and a permanent "why is hop special" carve-out; the fuzzy-only prompt makes consent data-dependent (a caller can't predict whether it fires without knowing the registry), which cuts against toolkit predictability.

## Why

1. **The pain point**: `hop rm <name>` resolves its argument by case-insensitive **substring** match (the same match-or-fzf seam as `hop <name> where`). A short or typo'd argument that happens to match exactly one *wrong* repo removes that entry immediately — no preview of what matched, no chance to abort. The write is visible only after the fact (`removed:` + `wrote:` lines). `--dry-run` (added by fcvp) helps only callers who think to use it.

2. **The consequence of not fixing**: hop stays non-conformant with toolkit principle №5 ("Destructive writes … MUST require explicit consent per №1's contract") — a standard the constitution binds this repo to (Constitution § Toolkit Standards). The fcvp conformance report carries an open "Deferred → [clc4]" entry that never closes, and the toolkit loses idiom uniformity: agents learn one consent pattern from `shll uninstall` (the named reference implementation) and find hop behaving differently.

3. **Why this approach**: the full gate (rather than a documented exemption) was chosen by the user because fuzzy resolution + immediate write is a real footgun regardless of blast radius, and one uniform consent idiom across the toolkit is worth the small automation migration (`--yes`). The reconciliation of "non-interactive by default" (№1) with "destructive writes need consent" (№5) is exactly the standard's own contract: prompt on a TTY, flag-based consent always available, fast refusal — never a hang — when neither is given.

## What Changes

### 1. TTY confirmation prompt on the direct `hop rm <name>` path

In `src/cmd/hop/config_rm.go`, the positional path (`runRm` with `name != ""`) gains a consent step between `resolveOne` and `removeRepo`. On a TTY (checked via the existing `isTTY` seam in `src/cmd/hop/tty.go`), before any write:

```
$ hop rm widget
remove: widget  (git@github.com:sahil87/widget.git)
Proceed? [y/N] y
removed: git@github.com:sahil87/widget.git
wrote: /Users/sahil/.config/hop/hop.yaml
```

- The pre-prompt line shows the **resolved match** — repo name and URL — so a wrong substring match is visible *before* the write (this is the footgun the gate exists to catch).
- Prompt text and the match line go to **stderr** (principle №2: stdout is data; `hop rm` keeps stdout empty).
- Accepted input: `y`/`yes` (case-insensitive) proceeds; anything else (including bare Enter — the `[y/N]` default is No) aborts with `aborted: no changes written` on stderr and **exit 0** (an answered "no" is a benign no-op, matching hop's forgiving exit-0 convention for "nothing to remove"; it is not an fzf-style cancellation, so 130 is not used).

### 2. `--yes`/`-y` flag for automation

`newRmCmd()` gains `cmd.Flags().BoolVarP(&yes, "yes", "y", false, ...)`. When set, the prompt is skipped entirely and the removal proceeds — this is №1's flag-based consent. Threaded through `runRm` alongside the existing `stale`/`dryRun` params.

- `--yes` composes with `--dry-run` trivially: `--dry-run` never requires consent at all (per the standard's reference: "`--dry-run` requiring no consent"), so the dry-run path is checked **before** the consent gate and behaves exactly as today, with or without `--yes`.
- `--yes` on the picker shape (`hop rm` with no positional) is **accepted and ignored** — the fzf pick is itself the consent (per the fcvp deferral rationale: "the picker path is already interactive consent"). It is redundant, not contradictory, so it does not get a usage-error rejection the way `--stale` + `<name>` does.

### 3. No-TTY refusal naming the flag

When `name != ""`, `--yes` absent, `--dry-run` absent, and `isTTY()` is false (agent / CI / pipe): fast refusal, no write, no hang:

```
$ hop rm widget        # stdin not a TTY
hop rm: consent required for removal — re-run with --yes (or preview with --dry-run)
(exit 3)
```

- **Exit 3** reuses hop's documented "a TTY was required and none is present" convention (`errNoTTY`, change `1x1u`) — but with a consent-specific message naming `--yes`, NOT the generic `noTTYHint` (which says "pass a repo name or use `hop ls --json`" — wrong advice here, since a name was passed). Mechanism: an `&errExitCode{code: 3, msg: ...}`-style sentinel or a parallel sentinel alongside `errNoTTY`, decided at plan time; the observable contract is the message + exit 3.
- This is the **breaking-ish** half: scripts/agents that today run `hop rm <name>` non-interactively will start refusing until they add `--yes`. The refusal is fast and actionable (names the flag), so callers adapt in one round-trip. Documented as a contract change.

### 4. Scope: which paths are gated

| Path | Consent behavior |
|---|---|
| `hop rm <name>` (TTY) | **NEW**: match-preview + `Proceed? [y/N]` prompt |
| `hop rm <name> --yes` | **NEW flag**: no prompt, proceeds |
| `hop rm <name>` (no TTY, no `--yes`) | **NEW**: refusal naming `--yes`, exit 3 |
| `hop rm <name> --dry-run` | unchanged — no consent needed (with or without TTY) |
| `hop rm` / `hop rm --stale` (picker) | unchanged — the fzf pick is the consent; no post-pick prompt |
| `hop config rm` (hidden alias, picker-only) | unchanged — **no `--yes` flag added** (it has no positional path, so there is no consent point; its consent is the pick) |

### 5. Shim / `HOP_WRAPPER` reconciliation

No shim changes needed, verified against the machinery: `rm` is a registered cobra subcommand, so the `--shim-plan` classifier emits `PASSTHROUGH` and the shim runs `command hop "$@"` with stdio inherited — the TTY prompt works through the shim, and exit codes propagate. `HOP_WRAPPER` only suppresses shell-only *hints*; the consent prompt/refusal are not hints and are never suppressed. The consent gate uses the same `isTTY` seam the fzf guards use, so shim and bare-binary behavior are identical.

### 6. Help text, spec, and docs updates

- `rmLong` (`config_rm.go`): document the prompt, `--yes`, the no-TTY refusal, and update the examples block (e.g., add `hop rm widget --yes`).
- `docs/specs/cli-surface.md`: update the `hop rm` inventory row, the exit-code table (exit 3 now also covers consent refusal), and behavioral scenarios (prompt-accept, prompt-decline, no-TTY refusal, `--yes`, `--dry-run` unaffected).
- `main.go::translateExit` doc comment: extend the exit-3 line beyond "interactive selection".
- README: update the `hop rm` row/mention if it documents immediacy (check at apply time).
- Memory updates happen at hydrate (see Affected Memory).
- The backlog `[clc4]` item is marked done at archive time (existing `/fab-archive` behavior).

### 7. Tests

Extend `src/cmd/hop/config_rm_test.go` (table-driven, seam-injected — follow existing patterns):

- TTY + `y` → removal proceeds (stub `isTTY` true, stub prompt input).
- TTY + Enter/`n`/garbage → abort, exit 0, `hop.yaml` unchanged, `aborted:` line.
- No TTY, no `--yes` → exit 3, consent message names `--yes`, no write.
- `--yes` (TTY and no-TTY) → no prompt, removal proceeds.
- `--dry-run` without `--yes`, no TTY → unchanged preview behavior, exit 0 (regression guard: dry-run needs no consent).
- Picker path → no prompt after pick (regression guard).
- `hop config rm` alias → no `--yes` flag registered.

## Affected Memory

- `cli/subcommands`: (modify) `hop rm [<name>]` inventory row — the "no confirmation prompt" contract is replaced by the consent gate (prompt / `--yes` / exit-3 refusal); exit-code convention table gains the consent-refusal trigger for 3
- `cli/agent-non-interactive-usage`: (modify) agent-facing contract — `hop rm <name>` now requires `--yes` non-interactively; recommended invocation pattern becomes `hop rm <name> --yes` (or `--dry-run` to preview); exit-3 semantics widened from "picker needs TTY" to "TTY-or-flag required"

## Impact

- **Code**: `src/cmd/hop/config_rm.go` (prompt + flag + refusal; ~the whole change), `src/cmd/hop/tty.go` or a small new confirm helper in `cmd/hop` (read a y/N line from the terminal — no new packages), `src/cmd/hop/main.go` (exit-3 doc comment only, if the sentinel approach reuses `errExitCode`).
- **Tests**: `src/cmd/hop/config_rm_test.go`.
- **Docs**: `docs/specs/cli-surface.md`, `rmLong` help text, README (conditional).
- **No changes** to: `internal/yamled` (RemoveURL/WouldRemoveURL untouched), the shim (`shell_init.go`), the `--shim-plan` classifier, match resolution, the hidden alias's flag surface.
- **Breaking-ish**: non-interactive `hop rm <name>` without `--yes` starts refusing (exit 3). Deliberate, user-approved contract change; migration is adding `--yes`.
- **Security**: prompt input is read locally and compared against a fixed accept set — no subprocess, no injection surface (Constitution I unaffected).

## Open Questions

- None — the central design decision (consent at all, and its shape) was asked and resolved at intake; remaining choices are graded assumptions below.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | `hop rm <name>` gets the full consent gate (TTY prompt + `--yes`/`-y` + no-TTY refusal) rather than a documented exemption or fuzzy-only prompt | Asked — user chose "Full consent gate" from three options, approving the previewed UX | S:100 R:90 A:100 D:100 |
| 2 | Certain | Prompt shape: stderr match-preview line (`remove: <name>  (<url>)`) then `Proceed? [y/N]` with No as default, mirroring `shll uninstall` | Reference implementation named by both principle №1 and the backlog entry; user-approved preview showed exactly this | S:90 R:85 A:95 D:90 |
| 3 | Certain | `--dry-run` requires no consent — checked before the gate, behavior unchanged | Principle №5's reference impl states it verbatim ("`--dry-run` requiring no consent at all") | S:90 R:90 A:100 D:95 |
| 4 | Certain | Picker paths (`hop rm`, `hop rm --stale`, `hop config rm`) get no post-pick prompt — the fzf pick is the consent | fcvp deferral rationale states it ("the picker path is already interactive consent"); backlog scopes the gap to the direct `<name>` path | S:85 R:80 A:90 D:85 |
| 5 | Confident | No-TTY refusal exits **3**, reusing the documented "TTY required" convention (`1x1u`) with a consent-specific message naming `--yes` — not a new code, not 2 | User-approved preview showed exit 3; keeps "3 = needed a terminal" coherent for agents branching on codes; message differs from `noTTYHint` because its advice ("pass a repo name") is wrong here | S:70 R:75 A:80 D:70 |
| 6 | Confident | Declined prompt (Enter/`n`/other) → `aborted: no changes written`, **exit 0** | Matches hop's forgiving exit-0 convention (not-found "Nothing to remove." exits 0); an answered "no" is not an fzf-style cancellation, so 130 is wrong | S:45 R:90 A:70 D:60 |
| 7 | Confident | `--yes` is accepted-and-ignored on the picker shape; the hidden `hop config rm` alias does NOT gain `--yes` at all | Redundant-not-contradictory (unlike `--stale`+`<name>`, which stays exit 2); alias is picker-only so it has no consent point — adding a meaningless flag is surface bloat (Constitution VI) | S:45 R:90 A:75 D:65 |
| 8 | Certain | No shim/`HOP_WRAPPER` changes — `rm` is `PASSTHROUGH`, stdio is inherited, prompt and exit codes flow through; consent gate reuses the `isTTY` seam | Verified against `shell_init.go`/`shim_plan.go` machinery docs; `HOP_WRAPPER` only suppresses shell-only hints | S:75 R:85 A:85 D:80 |
| 9 | Confident | Refusal message wording: `hop rm: consent required for removal — re-run with --yes (or preview with --dry-run)` | Follows №4's what/why/next shape and hop's `cmdName`-prefixed stderr voice; exact words are trivially reversible at apply | S:40 R:95 A:70 D:55 |

9 assumptions (5 certain, 4 confident, 0 tentative).
