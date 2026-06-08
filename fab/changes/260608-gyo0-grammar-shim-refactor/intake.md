# Intake: Grammar + Shim Refactor

**Change**: 260608-gyo0-grammar-shim-refactor
**Created**: 2026-06-08
**Status**: Draft

## Origin

> grammar-shim-refactor — Standardize hop's invocation grammar to `hop <selection> <action>` and radically simplify the shell shim. THIS IS THE CORE BUG FIX: hop "feels buggy" because the shell shim is a stateful blob that hard-codes the subcommand list and silently drifts out of sync with the binary.

Originated from an extended `/fab-discuss` session (2026-06-07 → 2026-06-08) that began with "hop is becoming very buggy." The discussion diagnosed the root cause empirically (see Why), explored the dispatch-design space, and locked every major decision interactively. This is the second of two changes from that session; the first (CI test gate, `260607-y1kf-ci-test-gate`) shipped as PR #39 and is now enforced via branch protection on `main`, so this refactor lands test-gated.

**Interaction mode**: conversational. Decisions were made one at a time with the user via structured questions; rationale is captured per-assumption below.

## Why

**The pain point.** hop "feels buggy." The triggering symptom: `hop add fab-kit` returned `hop: fzf failed: exit status 1` — an error that looks like a binary bug but isn't. The `add`/`rm` command code is correct; the failure was a **stale shell shim**. The user's live zsh session had an older `hop()` function in memory (from a previous `eval "$(hop shell-init zsh)"`) that predated the `add`/`rm` top-level promotion (commit 28517ab). That old shim's hard-coded subcommand `case` didn't list `add`, so it fell through to the repo-name branch, treated `add fab-kit` as a two-token *selection*, and ran fzf. Restarting zsh (reloading the current shim) fixed it.

**The structural cause.** This is not a one-off. The shim is a **stateful blob loaded into the user's interactive shell** that **hard-codes the subcommand list** (`add|rm|clone|pull|push|sync|ls|shell-init|config|update|...`), **duplicating** the binary's cobra registration in `src/cmd/hop/root.go`. Every grammar change (new/promoted subcommand, new verb) leaves every already-open shell running stale dispatch logic until re-eval or restart — and the failure is **silent**: a misroute surfaces as a confusing `hop:` error indistinguishable from a binary bug. The current invocation contract compounds this with **three inconsistent path-handoff mechanisms** (stdout for `where`, a temp-file via `WT_CD_FILE` for `open`, conditional-stdout for `clone`) and a `-R` flag whose silent rewrite produces opaque errors (`hop: -R: 'cursor' not found`).

**The consequence if unfixed.** The bugginess recurs by construction — every future grammar change risks reintroducing the same silent misroute class, and the contract sprawl makes the shim progressively harder to reason about.

**Why this approach.** Make the grammar uniform (`hop <selection> <action>`) and move subcommand classification *out of the shim into the binary*, so the subcommand list lives in exactly one place (cobra). The shim becomes a logic-free interpreter of a tiny fixed protocol — name-drift becomes structurally impossible. Alternatives considered and rejected: (a) a shorter hard-coded shim list (still drifts, just with fewer names); (b) a version handshake (subsumed once the shim hard-codes zero names); (c) `eval`-ing binary output (rejected — shell-injection surface, violates Constitution Principle I). Expected secondary benefit: significant code reduction across the ~1,657 lines currently in the shim/dispatch/batch surface.

## What Changes

### 1. Unified grammar: `hop <selection> <action>`

Selection-first, always. One grammar for every interactive invocation.

- `<selection>` = a repo name (substring → fzf on ambiguity), a `repo/worktree`, OR a group name.
- `<action>` = a builtin verb (`cd`, `open`, `where`), a reoriented batch verb (`pull`/`push`/`sync`), any PATH binary (`git pull`, `code .`), or any shell alias/function (`p`).

```
hop webapp                # cd into webapp
hop webapp open           # open in editor (via wt menu)
hop webapp where          # print abs path
hop webapp/feat-x         # cd into the feat-x worktree of webapp
hop webapp git pull       # run `git pull` in webapp
hop webapp code .         # open editor in webapp
hop webapp p              # run shell alias `p` in webapp
hop webapp pull           # reoriented batch verb (was `hop pull webapp`)
```

### 2. Dispatch via binary-owned classification — NO `eval`

The binary classifies `$1` via a hidden internal flag (`--shim-plan`-style) and emits a small FIXED 3-keyword protocol. The shim interprets it with a fixed `case` and runs `"$@"` (the user's already-parsed words) in the parent shell — never `eval` of binary output.

Protocol vocabulary:
- `CD\n<path>` → shim runs `cd -- <path>` (plain navigation, e.g. bare `hop webapp` / `hop webapp cd`).
- `RUN_IN_PARENT\n<path>` → shim runs `cd -- <path>; shift; "$@"` so aliases/functions/PATH resolve.
- `PASSTHROUGH` → shim runs `command hop "$@"` (binary handles it: `add`, `rm`, `clone`, `ls`, `config`, `update`, `shell-init`, `--help`/`-h`/`--version`/`completion`/`__complete*`).

```zsh
hop() {
  local plan; plan="$(command hop --shim-plan "$@")" || return $?
  case "${plan%%$'\n'*}" in
    CD)            cd -- "$(print -r -- "$plan" | sed -n 2p)" ;;
    RUN_IN_PARENT) cd -- "$(print -r -- "$plan" | sed -n 2p)" || return; shift; "$@" ;;
    PASSTHROUGH)   command hop "$@" ;;
  esac
}
```

**Why not `eval`:** `"$@"` are already-parsed shell words (what the user typed), not binary-generated code. The binary emits only the fixed vocabulary + a path used as a quoted `cd` operand — no re-parsing of binary output as code, so no shell-injection surface (Constitution Principle I).

### 3. Zero subcommand names hard-coded in the shim

The shim's `case` is over the 3 action keywords, NOT over subcommand names. The subcommand list lives ONLY in cobra. Shim/binary name-drift becomes structurally impossible — this is the permanent fix for the stale-shim class of bug.

### 4. Collapse the three path-handoff mechanisms into the protocol

Today's stdout (`where`) / temp-file `WT_CD_FILE` (`open`) / conditional-stdout (`clone`) handoffs are unified under the single protocol contract. The binary resolves and returns a plan; the shim acts on it. (Note: `open` delegates to wt's interactive menu — its stdio handling must be preserved within the protocol; resolve exact mechanism at plan stage.)

### 5. Reorient `pull`/`push`/`sync` to selection-first

`hop pull <repo>` → `hop <repo> pull`. They are no longer cobra subcommands the shim recognizes — they are action tokens after a selection.
- `pull`/`push` internally mean `git pull` / `git push`.
- `sync` = the full pull-rebase + commit + push workflow.
- The selection can be a repo, a worktree, OR a group.

### 6. Plural selection is first-class

`hop --all <action>` and `hop <group> <action>` run the action across all matched repos.
- No-target batch becomes `hop --all pull` (selection = `--all`), replacing the old `hop pull --all`.
- **Guard**: refuse interactive commands (e.g. `code .`) on a plural selection (running an interactive tool across N repos is nonsensical). Exact detection heuristic resolved at plan stage.

### 7. Drop the `-R` flag and its silent rewrite

The user-facing `-R` flag and the shim's silent `hop <name> <tool>` → `hop -R <name> <tool>` rewrite are removed entirely. Tool-form is now the native grammar, not a rewrite. Kills the confusing `hop: -R: '<cmd>' not found` errors.

### 8. Drop the `hi` alias

Remove `hi` (binary-direct). `command hop` remains the raw escape hatch. Minimal surface per Constitution Principle VI.

### 9. Rename placeholder repo name `outbox` → `webapp`

Across docs/specs/tests/help text. Use `frontend`/`backend` when an example needs two repos. "outbox" reads as an email folder and confuses readers.

### 10. No version handshake

The shim is logic-free (just `hop shell-init zsh` output) and hard-codes zero names, so the only drift risk is the protocol vocabulary changing — rare, and self-heals on re-eval. Graceful degradation: if the binary sees an unrecognized old-shim invocation, it emits `PASSTHROUGH` and lets cobra print a normal error.

## Affected Memory

- `cli/subcommands`: (modify) — grammar reorients to `hop <selection> <action>`; `pull`/`push`/`sync` become action tokens not subcommands; `-R` and `hi` removed
- `cli/match-resolution`: (modify) — selection resolution now feeds the `--shim-plan` classification; plural selection (`--all`/group) semantics added
- `architecture/wrapper-boundaries`: (modify) — shim ↔ binary boundary redrawn: binary owns classification, shim interprets a fixed 3-keyword protocol; three path-handoff mechanisms collapse to one; `-R` argv inspection in `main.go` removed
- `architecture/package-layout`: (modify) — likely net code reduction (shell_init, repo_completion, resolve, pull/push/sync/batch, open, main `-R` path)

## Impact

**Code areas** (all under `src/cmd/hop/`, ~1,657 lines in scope today):
- `shell_init.go` (199) — shim rewritten to the protocol interpreter; `hi` removed
- `main.go` (155) — `-R` extraction (`extractDashR`/`runDashR`) removed
- `root.go` (138) — root RunE dispatch reworked; new hidden `--shim-plan` classifier; help text updated
- `resolve.go` (309) — feeds classification; plural-selection resolution
- `repo_completion.go` (202) — completion revisited under new grammar (and the synchronous `wt list --json` blocking issue noted in discussion)
- `pull.go` (129), `push.go` (114), `sync.go` (306), `batch.go` (61) — reoriented to selection-first; batch via plural selection
- `open.go` (44) — `WT_CD_FILE` handoff folded into the protocol

**External tools**: continues to wrap `git`, `fzf`, `wt`, editor/`open` (Principle IV). No new dependencies.

**Cross-cutting**: docs/specs/tests + help text (placeholder rename, grammar examples). Tests must conform to the new spec (Constitution Test Integrity).

## Open Questions

- `open`'s interactive wt-menu stdio: exact mechanism for preserving wt's interactive menu while routing the "Open here" cd-target through the protocol (today it uses the `WT_CD_FILE` side channel) — resolve at plan stage.
- Plural-selection interactive-command guard: the precise heuristic for detecting "interactive" actions to refuse on `--all`/group selections.
- Completion under the new grammar: how `--shim-plan` interacts with cobra's `__complete*` path (in scope — the grammar strictly requires `__complete*` to keep working, and `pull`/`push`/`sync` to stop completing as subcommands). The completion *bugs* (sync `wt list --json` blocking, ambiguous-prefix worktree completion, swallowed errors) are OUT of scope here — deferred to backlog `[cmp7]` (likely root-fixed via a `wt list --porcelain` option). <!-- clarified: completion bug fixes deferred to backlog [cmp7]; this change only does grammar-forced completion updates -->

## Scope boundary: completion

In scope: keep cobra `__complete*` working under the new `--shim-plan` dispatch; `pull`/`push`/`sync` no longer complete as subcommands (they're action tokens). Out of scope: the known completion bugs (blocking `wt list --json`, ambiguous-prefix worktree completion, swallowed errors) — deferred to backlog `[cmp7]`, whose root fix likely lands in `wt` as a `--porcelain` option.

## Clarifications

### Session 2026-06-08

| # | Q | A |
|---|---|---|
| 12 | Fix completion bugs in this change, or defer? | Defer to backlog `[cmp7]`; this change does only grammar-forced completion updates. Root fix likely belongs in `wt` as a `--porcelain` option (two-repo fix). |

### Session 2026-06-08 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 8 | Confirmed | — |
| 9 | Confirmed | — |
| 10 | Confirmed | — |
| 11 | Confirmed | — |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Unified grammar `hop <selection> <action>`, selection-first | Discussed — user explicitly chose this as the standard form | S:98 R:70 A:95 D:95 |
| 2 | Certain | Binary-owned classification via hidden `--shim-plan`; shim interprets fixed 3-keyword protocol (`CD`/`RUN_IN_PARENT`/`PASSTHROUGH`); NO eval | Discussed — user confirmed "structured protocol, no eval"; eval rejected for injection risk (Principle I) | S:95 R:55 A:90 D:90 |
| 3 | Certain | Zero subcommand names hard-coded in the shim; list lives only in cobra | Discussed — direct consequence of #2; user's stated goal (kill name-drift) | S:95 R:60 A:95 D:95 |
| 4 | Certain | Drop the `hi` alias entirely; `command hop` is the escape hatch | Discussed — user said "we can get rid of the hi command" | S:98 R:90 A:95 D:98 |
| 5 | Certain | Drop the `-R` flag and its silent rewrite | Discussed — user OK'd; tool-form is native grammar now | S:95 R:75 A:90 D:92 |
| 6 | Certain | Rename placeholder `outbox` → `webapp` (frontend/backend for two-repo examples) | Discussed — user chose `webapp (+frontend/backend)` | S:98 R:95 A:95 D:98 |
| 7 | Certain | Reorient `pull`/`push`/`sync` to selection-first (`hop <repo> pull`), keep their meaning | Discussed — user: "change the form from hop pull outbox to hop outbox pull"; pull/push=git pull/push, sync=full workflow | S:95 R:70 A:90 D:90 |
| 8 | Certain | Plural selection first-class (`hop --all <action>`, `hop <group> <action>`); refuse interactive commands on plural | Clarified — user confirmed (bulk) | S:95 R:60 A:80 D:82 |
| 9 | Certain | No-target batch becomes `hop --all pull` (selection=`--all`), replacing `hop pull --all` | Clarified — user confirmed (bulk) | S:95 R:65 A:80 D:80 |
| 10 | Certain | No version handshake; graceful degradation via `PASSTHROUGH` on unrecognized old-shim calls | Clarified — user confirmed (bulk) | S:95 R:75 A:85 D:85 |
| 11 | Certain | Collapse the three path-handoff mechanisms (stdout/`WT_CD_FILE`/conditional-stdout) into the protocol | Clarified — user confirmed (bulk); exact `open` stdio mechanism deferred to plan (Open Question) | S:95 R:55 A:80 D:75 |
| 12 | Certain | Completion BUG fixes (sync `wt list --json` blocking, ambiguous-prefix worktree completion, swallowed errors) are DEFERRED to backlog `[cmp7]`; this change does only what the new grammar strictly forces in completion | Clarified — user chose defer + backlog; noted the root fix likely belongs in `wt` as a `--porcelain` option (two-repo fix) | S:95 R:60 A:65 D:50 |

12 assumptions (12 certain, 0 confident, 0 tentative, 0 unresolved).
