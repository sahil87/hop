# Intake: Make hop usable by non-interactive agents (+ shim hardening)

**Change**: 260615-1x1u-agent-non-interactive-usage
**Created**: 2026-06-15
**Status**: Draft

## Origin

This change was initiated in autonomous dispatch mode via `/fab-proceed` → `/fab-new`, from a synthesized one-shot description (the sole source of truth for this intake — no other draft was consulted). The description was produced after a live investigation session that confirmed a concrete, reproducible failure and several latent gaps that together prevent AI coding agents (and non-interactive CI scripts) from reliably driving `hop`.

> AI coding agents (and CI scripts) cannot reliably drive `hop`. Investigation this session confirmed the concrete failure and several latent gaps:
>
> 1. CONFIRMED BUG — partial shim capture. `hop` is a shell FUNCTION emitted by `hop shell-init`, defined in `src/cmd/hop/shell_init.go::posixInit`. It defines TWO separate top-level shell functions: `hop()` (the dispatcher) and `_hop_passthrough()` (a sibling helper that `hop()` calls in its PASSTHROUGH arm). Claude Code's shell-snapshot mechanism captured `hop()` but NOT `_hop_passthrough()`. Result: every PASSTHROUGH plan (which is `where`, `ls`, `add`, `rm`, `clone`, `config`, `pull`/`push`/`sync`, `--help`, `--version`) dies with `_hop_passthrough: command not found`. The binary, config, and resolution logic were correct the whole time — verified `/home/linuxbrew/.linuxbrew/bin/hop fab-kit where` resolves cleanly with no fzf. This is the [shim-staleness] bug class, new facet: not drift/absence but PARTIAL CAPTURE — a dispatcher function depending on a separately-defined sibling is fragile to per-function snapshotting.
> 2. No machine-readable repo listing. `hop ls` is human-padded text; the only JSON anywhere is the hidden `help-dump`. Agents cannot enumerate repos as data.
> 3. fzf is a hard wall with no TTY. Bare `hop`, ambiguous matches, and `hop rm`/`hop clone` with no name invoke fzf. With no TTY, fzf exits 1 → folded to `errFzfCancelled` → exit 130 — a confusing "cancelled" code for "no TTY to pick with". No TTY detection exists anywhere (zero `IsTerminal`/`isatty` hits in `src/`).
> 4. Shell-only verbs (`hop <name>`, `hop <name> cd`, tool-form) error with exit 2 + an "install the shim" hint — correct for humans, a dead-end for agents.

**Interaction mode**: one-shot, autonomous (no clarifying questions asked — SRAD defaults applied per dispatch mode). All four scope items and the two locked decisions below were confirmed in the synthesized description.

**Investigation grounding** (verified against source during intake generation, not assumed):
- `shell_init.go::posixInit` (lines 47–114) confirms TWO separate top-level functions: `hop()` whose PASSTHROUGH arm (line 77) calls `_hop_passthrough "$@"`, and `_hop_passthrough()` (lines 94–110) defined as a sibling. The WT_CD_FILE cd side-channel lives entirely inside `_hop_passthrough` (mktemp → export `WT_CD_FILE` → run binary → read non-empty file → `cd` → propagate `rc` → `rm`). The `h()` alias is line 112.
- `ls.go` has only a `--trees` bool flag; no JSON anywhere. `WtEntry` exposes `Dirty bool` and `Unpushed int` (consumed by `formatTreesRow`).
- `resolve.go` fzf via the `pickResolve = fzf.Pick` seam (line 45, called line 115); `config_rm.go` via the `pickOne = fzf.Pick` seam (line 23, called line 221). Both fold fzf exit-1 and exit-130 into `errFzfCancelled` → exit 130.
- Zero `IsTerminal`/`isatty`/`golang.org/x/term` hits across `src/`. `go.mod` does NOT list `golang.org/x/term`.
- `root.go` hint constants `bareNameHint` (line 72), `cdHint` (line 77), `toolFormHintFmt` (line 83); returned as exit-2 errors at lines 145/159/168. No `HOP_WRAPPER`/`WT_WRAPPER` read by hop's binary today (only a comment mention in `internal/update/update.go`; a test fragment asserts `WT_WRAPPER` must NOT appear in hop's shim).

## Why

**Problem.** `hop` is built for an interactive human at a TTY with the shell shim installed. Three of its design assumptions break for an AI agent or CI script:

1. **The shim is fragile to partial capture.** `posixInit` emits two cooperating top-level functions. Claude Code's shell-snapshot captured `hop()` but not its sibling `_hop_passthrough()`, so every PASSTHROUGH command (the majority — `where`, `ls`, `add`, `rm`, `clone`, `config`, `pull`/`push`/`sync`, `--help`, `--version`) died with `_hop_passthrough: command not found`. The binary was correct all along. This is a new facet of the documented "stale shim" bug class (architecture/package-layout: "the structural fix for the stale shim bug class") — not name-drift, but a dispatcher depending on a separately-defined sibling that per-function snapshotting can split apart.

2. **There is no data interface.** `hop ls` emits human-aligned columns; the only JSON surface is the hidden `help-dump` (a help-tree, not a repo inventory). An agent has no way to enumerate repos, their paths, URLs, groups, or worktree status as structured data — it must scrape padded text.

3. **fzf is a hard wall with no TTY, and the failure is mislabeled.** Bare `hop`, ambiguous/zero-match name resolution, and `hop rm` with no name all reach fzf. With no controlling TTY, fzf exits 1, which `resolveByName`/`pickRepo` fold into `errFzfCancelled` → exit 130. An agent sees "cancelled" when the real problem is "there is no terminal to pick with" — undiagnosable from the exit code. No TTY detection exists in the codebase.

4. **Shell-only verbs dead-end agents.** `hop <name>`, `hop <name> cd`, and tool-form correctly error (exit 2) for a human without the shim, pointing them at `eval "$(hop shell-init zsh)"`. For an agent that has the wrapper present, that hint is noise on a path that should just work or stay quiet.

**Consequence if unfixed.** Agents and CI cannot use `hop` as a programmatic locator: PASSTHROUGH commands break under snapshotting, repo enumeration requires brittle text-scraping, the no-TTY path returns a misleading exit code, and shell-only verbs surface human-only hints. `hop`'s value as the canonical repo-locator is lost exactly in the automation context where a locator is most useful.

**Why this approach.** Each item mirrors an EXISTING precedent in the sibling tool `wt` (and `idea`), per Constitution IV (wrap, don't reinvent) and toolchain consistency — rather than inventing new patterns:
- Item 1 makes the shim a single self-contained capturable unit (inline the sibling's body) — the only form provably immune to partial capture, and consistent with the file's existing "logic-free interpreter" framing.
- Item 2 mirrors `wt list --json`'s field-naming and pointer-field/omitempty conventions.
- Item 3 mirrors `wt`/`idea`'s `term.IsTerminal` TTY detection (golang.org/x/term) and the no-TTY fast-fail pattern.
- Item 4 mirrors `wt`'s `WT_WRAPPER` env signal (using a hop-specific `HOP_WRAPPER` to avoid coupling to wt).

**Rejected alternatives.**
- *Item 1 — nest `_hop_passthrough` inside `hop()` instead of inlining.* Rejected (LOCKED by user): a nested function is still a separate definition object and a snapshotter could in principle split it; only inlining the body directly into the PASSTHROUGH arm is provably immune to partial capture, and it matches the file's logic-free-interpreter framing. The body is short enough that inlining costs little readability.
- *Item 2 — invent a new JSON schema.* Rejected: Constitution IV + toolchain consistency demand mirroring `wt list --json`'s established field names so agents that already parse `wt` output reuse the same shape.
- *Item 3 — reuse exit 130 for no-TTY.* Rejected: 130 means "user cancelled (Esc/Ctrl-C)"; conflating "no TTY" with "cancelled" is exactly the misdiagnosis being fixed. A distinct code is required.
- *Item 4 — reuse `WT_WRAPPER`.* Rejected: it would couple hop's hint suppression to wt's wrapper presence; a hop-specific `HOP_WRAPPER` is correct.

## What Changes

Four items. All four are confirmed in scope. The change is dominated by new agent-facing surface (items 2–4 add capability); item 1 is the bug fix but is the smallest delta. Change type is `feat`.

### Item 1 — Self-contained shim (the fix)

Inline `_hop_passthrough`'s body **directly** into the PASSTHROUGH arm of `hop()` in `posixInit` (`src/cmd/hop/shell_init.go`). After this change `posixInit` emits a single capturable `hop()` unit (plus the `h()` alias) — no sibling top-level function for a snapshotter to drop.

**DECISION LOCKED BY USER: inline (not nest).** Rationale: only inlining is provably immune to partial capture; nesting still produces a separate definition object. Inlining also matches the file's existing "logic-free interpreter" framing.

**MUST preserve the `WT_CD_FILE` cd side-channel exactly.** The inlined body keeps the current `_hop_passthrough` semantics verbatim:

```sh
PASSTHROUGH)
  # (inlined from the former _hop_passthrough — unified cd side-channel via WT_CD_FILE)
  local cdfile target rc
  cdfile="$(mktemp -t hop-cd.XXXXXX)" || { command hop "$@"; return $?; }
  WT_CD_FILE="$cdfile" command hop "$@"
  rc=$?
  target=""
  if [[ -s "$cdfile" ]]; then
    target="$(cat "$cdfile")"
  fi
  rm -f "$cdfile"
  if (( rc != 0 )); then
    return $rc
  fi
  if [[ -n "$target" ]]; then
    cd -- "$target"
  fi
  ;;
```

(Exact variable names, the `mktemp` fallback to `command hop "$@"`, the `-s` non-empty test, the `rc != 0` early return, the trailing `rm -f`, and the final conditional `cd` are all preserved. The body sits inside `hop()`'s `case` so its `local` declarations and `return` statements behave identically to the standalone function.)

**Unchanged:** the `__complete*` early-forward arm, the `CD` and `RUN_IN_PARENT` arms, the defensive `*)` fallback, the `h()` alias, and the per-shell cobra completion suffix (`compdef _hop h` / `complete -o default -F __start_hop h`) all stay as-is. The standalone `_hop_passthrough()` function definition (lines 94–110) is removed once its body is inlined.

**Binary side is unchanged.** hop already CONSUMES `WT_CD_FILE` in `clone.go` (clone's cd-target write) and `open.go` (`wt open`'s "Open here") — that binary-side behavior is not touched.

**Test impact:** `posixInit` is a Go string constant, exercised by existing shell-shim tests and `shim_plan_test.go`. `shell_init_test.go` currently asserts `_hop_passthrough` appears (and that `WT_WRAPPER`/`_hop_dispatch` do NOT) — those assertions conform to the implementation spec and update to match the new single-function shape (tests follow the spec, never the reverse — Test Integrity).

### Item 2 — `hop ls --json`

Add a `--json` bool flag to `hop ls` (`src/cmd/hop/ls.go`) producing machine-readable output for BOTH the default mode and `--trees` mode. `--json` + `--trees` compose.

**SCHEMA — mirror the sibling `wt list --json` field-naming convention** (CONFIDENT default per Constitution IV wrap-don't-reinvent + toolchain consistency):

- An array of repo objects, each with: `name`, `path`, `url`, `group`.
- Non-cloned repos and per-repo `wt list` failures MUST be representable IN JSON (not inline human text). The exact representation is settled below as a Tentative assumption.

For `--trees`, nest a `worktrees` array per repo. Per-worktree status uses wt's pointer-field + `omitempty` style so "not computed" is distinguishable from zero:
- `dirty` — `*bool` with `omitempty`
- `unpushed` — `*int` with `omitempty`
- plus the worktree's `name` and `path`

```jsonc
// hop ls --json
[
  { "name": "webapp", "path": "/home/u/code/org/webapp", "url": "git@github.com:org/webapp.git", "group": "default" }
]

// hop ls --json --trees
[
  {
    "name": "webapp", "path": "...", "url": "...", "group": "default",
    "worktrees": [
      { "name": "main", "path": "...", "dirty": false, "unpushed": 2 },
      { "name": "feat-x", "path": "..." }            // status not computed → fields omitted
    ]
  }
]
```

**Cloned-state / failure representation in JSON** (mirrors the text-mode behaviors in `runLsTrees`):
- A non-cloned repo (`--trees`): represent its uncloned state structurally rather than as the text `(not cloned)`. Tentative default below: omit `worktrees` and include a `cloned: false` field (or `worktrees: null`).
- A per-repo `wt list` failure (`--trees`): represent structurally rather than the text `(wt list failed: <err>)`. Tentative default below: an `error` string field on that repo object; the array is never aborted (matches text mode's never-abort contract).
- The FIRST `wt list` invocation hitting `proc.ErrNotFound` (wt missing on PATH) keeps the existing fail-fast: `wtMissingHint` to stderr, exit 1 — JSON is not emitted in that case (consistent with text mode).
- Empty repo list: `hop ls --json` emits `[]` (an empty array), not empty output — JSON consumers expect valid JSON. (Text mode prints nothing for an empty list; JSON mode emits `[]`.)

**Ordering:** preserve YAML source order, exactly as text mode does (via `repos.FromConfig`).

### Item 3 — TTY-aware fzf guard

Detect absence of a controlling TTY and, when fzf WOULD be spawned with no TTY, fail fast with a clear, actionable message and a DISTINCT exit code (NOT 130 fzf-cancel).

**Dependency:** `term.IsTerminal` via `golang.org/x/term` — NOT currently a hop dependency; it MUST be added to `src/go.mod` (and `go.sum`). Siblings `wt` and `idea` already depend on it (`idea` exposes an `internal/idea/term.go IsTTY` seam to mirror). It works on darwin-arm64/amd64 and linux-arm64/amd64 (Cross-Platform constraint satisfied).

**Detection placement (Tentative default below):** introduce a small TTY seam — either a tiny `src/internal/tty` package or a package-level `var isTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }` in `cmd/hop/` — mirroring `idea`'s `IsTTY` seam so it is swappable for tests (the codebase already favors package-level seams: `pickResolve`, `pickOne`, `listWorktrees`). The seam checks the relevant fd (stdin and/or stderr — settle which during apply; fzf needs a real terminal on its tty).

**Guarded paths:** the fzf invocation sites —
- `resolve.go::resolveByName` immediately before `pickResolve(...)` (line ~115),
- `config_rm.go::pickRepo` immediately before `pickOne(...)` (line ~221).

When no TTY: instead of spawning fzf, return a typed error that `translateExit` (and the `--shim-plan` path's `shimResolveErr`) maps to the new DISTINCT exit code, with a stderr line like:

```
hop: no TTY for interactive selection — pass a repo name or use `hop ls --json`
```

**OPEN DESIGN POINT 1 (settle in intake, propose a default): the no-TTY exit code.** Current codes are 0 (success), 1 (app error / `errSilent`), 2 (usage error / `errExitCode{code:2}`), 130 (`errFzfCancelled`). The no-TTY guard needs a code distinct from all of these. **Proposed default (Tentative): 3** — the next free small integer after the existing 0/1/2 usage band, clearly NOT 130 (cancel). A new sentinel (e.g. `errNoTTY`) maps to it in `translateExit` and `shimResolveErr`. (Alternative considered: reuse 2 as "usage: no TTY" — rejected because 2 already means a malformed-invocation usage error, and an agent should be able to distinguish "you typed it wrong" from "I have no terminal".)

**OPEN DESIGN POINT 2 (settle in intake, propose a default): scope of the guard — bare-`hop` picker too, or only named-resolution fzf paths?** **Proposed default (Tentative): cover ALL fzf-spawning paths, including the bare-`hop` picker.** Rationale: the bare picker hits the same `resolveByName` (empty-query) → `pickResolve` path, so guarding `resolveByName` before `pickResolve` naturally covers bare `hop`, the 0/2+-match named cases, AND `hop clone`/`hop rm` with no name — a single guard point per seam. Narrowing it to only named resolution would require a special-case that re-admits the exact no-TTY hang the change is removing. (If apply finds the bare picker genuinely needs different wording, that is a message refinement, not a scope change.)

### Item 4 — `HOP_WRAPPER` hint suppression

The shim exports `HOP_WRAPPER=1` (in `posixInit`); the binary reads it to suppress the shell-only hints (`bareNameHint`, `cdHint`, `toolFormHintFmt` in `root.go`) when a wrapper is known present.

**Shim side (`shell_init.go::posixInit`):** export `HOP_WRAPPER=1` so any process the shim invokes (the binary) sees it. Mirrors wt's `WT_WRAPPER` (wt's `shell_init.go` exports it). Use the hop-specific name `HOP_WRAPPER` — NOT `WT_WRAPPER` — to avoid coupling hop's hint suppression to wt's wrapper presence. (Settle during apply where exactly the export lives — most naturally a top-level `export HOP_WRAPPER=1` in the emitted shim, alongside the `hop()`/`h()` definitions.)

**Binary side (`root.go`):** before returning `bareNameHint` / `cdHint` / `toolFormHintFmt` (the exit-2 shell-only hints at lines 145/159/168), check the env. Mirrors wt's pattern (`apps.go`: `if os.Getenv("WT_WRAPPER") != "1"`):

```go
if os.Getenv("HOP_WRAPPER") != "1" {
    // print the shell-only hint as today
}
```

**Settle in apply — exit code when the hint is suppressed.** When `HOP_WRAPPER=1` and the binary is reached directly with a bare-name/cd/tool-form invocation (which the shim should normally have routed via `--shim-plan`, so this is an edge case): the hint text is suppressed, but the invocation is still one the binary cannot honor on its own (cd/tool-form run in the parent shell). Tentative default below: keep the exit-2 (usage) code, only suppressing the hint TEXT — matching wt's "suppress the hint, not the error" behavior. (`bareNameHint`'s exit is intrinsic to a form the binary genuinely cannot fulfill; suppression removes the now-redundant nudge, not the failure.)

## Affected Memory

- `architecture/package-layout`: (modify) shim shape — `posixInit` now a single self-contained `hop()` (inlined PASSTHROUGH, `_hop_passthrough` removed); new `golang.org/x/term` dependency in `src/go.mod`; possibly a new `internal/tty` package or a `cmd/hop` TTY seam.
- `cli/subcommands`: (modify) `hop ls --json` (new flag + schema for default and `--trees` modes); shell-only hint suppression via `HOP_WRAPPER`; the new no-TTY exit code added to the exit-code convention table.
- `cli/match-resolution`: (modify) TTY-aware guard on the fzf-spawning paths (`pickResolve` / `pickOne`); the no-TTY exit code distinct from 130 in the cancellation/missing-fzf section.
- `cli/agent-non-interactive-usage`: (new) a concept note tying together the agent/non-interactive contract — self-contained shim, `--json` enumeration, no-TTY exit code, `HOP_WRAPPER` — as the "how to drive hop without a human/TTY" reference. (Settle during hydrate whether this lands as a new `cli/` memory file or is folded into the existing three.)

## Impact

**Code areas / files (awareness, not prescriptive):**
- `src/cmd/hop/shell_init.go` — `posixInit`: inline PASSTHROUGH body (item 1), export `HOP_WRAPPER=1` (item 4).
- `src/cmd/hop/ls.go` — `--json` flag, JSON marshalling for default + `--trees` modes, JSON-representable non-cloned / wt-failure states (item 2).
- `src/cmd/hop/resolve.go` — TTY guard before `pickResolve`; new no-TTY sentinel/exit-code mapping (item 3).
- `src/cmd/hop/config_rm.go` — TTY guard before `pickOne` (item 3).
- `src/cmd/hop/root.go` — `HOP_WRAPPER` check around `bareNameHint`/`cdHint`/`toolFormHintFmt` (item 4).
- `src/cmd/hop/main.go` — `translateExit` gains the new no-TTY exit code; `shimResolveErr` (`shim_plan.go`) mirrors it (item 3).
- `src/go.mod` / `src/go.sum` — add `golang.org/x/term` (item 3).
- possibly `src/internal/tty/` (new) or a `cmd/hop` TTY seam (item 3).

**APIs / contracts:**
- New stdout contract: `hop ls --json` (and `--json --trees`) — a stable JSON schema agents parse.
- New exit code (distinct no-TTY code, proposed 3) — added to the documented exit-code convention.
- New env signal `HOP_WRAPPER` (shim → binary).
- Shim contract: `posixInit` becomes a single `hop()` unit (capturability guarantee).

**Dependencies:** one new direct dependency — `golang.org/x/term` (cross-platform, already used by sibling tools).

**Tests:** `shell_init_test.go` (shim shape — single function, `HOP_WRAPPER` present), `shim_plan_test.go` (keep green), `ls_test.go` (`--json` for both modes + edge states), `resolve_test.go` / `config_rm_test.go` (TTY guard via the swappable seam — inject no-TTY, assert the new exit code, never spawn real fzf), `root` hint tests (suppression under `HOP_WRAPPER`). All test changes conform to the implementation spec (Test Integrity).

**Constraints honored:**
- Constitution I (security): the shim stays a logic-free interpreter — no `eval` of binary output; the inlined PASSTHROUGH body keeps the same no-injection property (only the fixed vocabulary + a quoted `cd` operand).
- Constitution II (no database) / III (convention over config): no new persistent state; only flags (`--json`) and env (`HOP_WRAPPER`) where convention genuinely cannot suffice.
- Constitution IV (wrap, don't reinvent): every item mirrors the existing `wt`/`idea` precedent rather than inventing a pattern.
- Constitution VI (minimal surface area): no new top-level subcommand — `--json` is a flag on `ls`; the guard and env signal are not commands.
- Cross-Platform: `term.IsTerminal` builds/runs on all four supported targets.

## Open Questions

- No-TTY exit code: proposed **3** (distinct from 0/1/2/130). Confirm 3 vs another free code (e.g. a higher value to avoid any future 3=usage convention). (Open Design Point 1.)
- Guard scope: proposed to cover **all** fzf-spawning paths including the bare-`hop` picker (single guard per seam). Confirm vs named-resolution-only. (Open Design Point 2.)
- JSON representation of non-cloned repos and per-repo `wt list` failures under `--trees`: proposed `cloned: false` (omit `worktrees`) and an `error` string field, respectively. Confirm exact field names against the actual `wt list --json` schema during apply.
- Where exactly `HOP_WRAPPER=1` is exported in `posixInit`, and whether suppression keeps the exit-2 code (proposed: yes — suppress text only). Confirm against wt's `WT_WRAPPER` placement.
- Whether `cli/agent-non-interactive-usage` becomes a new memory file or folds into the existing three `cli/` files. (Hydrate-time decision.)

## Clarifications

### Session 2026-06-15 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 4 | Confirmed | — |
| 5 | Confirmed | — |
| 6 | Confirmed | — |
| 7 | Confirmed | Open Design Point 2 — guard covers all fzf paths |
| 8 | Confirmed | Open Design Point 1 — no-TTY exit code = 3 |
| 9 | Confirmed | — |
| 10 | Confirmed | — |
| 11 | Confirmed | — |
| 12 | Deferred | To hydrate — new file vs fold-in decided at hydrate |
| — | Change type | Corrected `fix` → `feat` in `.status.yaml` (intake body + assumption #2 always said feat) |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Item 1: inline `_hop_passthrough`'s body directly into `hop()`'s PASSTHROUGH arm (not nest), preserving the `WT_CD_FILE` side-channel verbatim | LOCKED by user in the description; only inlining is provably immune to partial capture; matches the file's logic-free-interpreter framing | S:98 R:80 A:90 D:95 |
| 2 | Certain | Change type is `feat` | Dominated by new agent-facing surface (items 2–4); description explicitly states feat; keyword + intent both point to feat | S:95 R:85 A:90 D:90 |
| 3 | Certain | Each item mirrors the existing `wt`/`idea` precedent (Constitution IV) rather than inventing new patterns | Constitution IV (wrap, don't reinvent) + explicit toolchain-consistency direction in the description | S:95 R:75 A:95 D:90 |
| 4 | Certain | Item 2 schema: array of `{name, path, url, group}`, nested `worktrees` for `--trees` with pointer-field+omitempty `dirty`/`unpushed`, mirroring `wt list --json` | Clarified — user confirmed | S:95 R:65 A:80 D:75 |
| 5 | Certain | Item 3 uses `term.IsTerminal` via `golang.org/x/term`, added to `src/go.mod`, behind a swappable TTY seam mirroring `idea`'s `IsTTY` | Clarified — user confirmed | S:95 R:70 A:85 D:80 |
| 6 | Certain | Item 4 uses a hop-specific `HOP_WRAPPER=1` (shim exports, binary reads to suppress hints), mirroring wt's `WT_WRAPPER` | Clarified — user confirmed | S:95 R:75 A:88 D:85 |
| 7 | Certain | Item 3 guard covers ALL fzf-spawning paths including the bare-`hop` picker (one guard point per seam: before `pickResolve` and before `pickOne`) | Clarified — user confirmed (Open Design Point 2 resolved) | S:95 R:60 A:75 D:70 |
| 8 | Certain | No-TTY exit code = **3** (distinct from 0/1/2/130), via a new `errNoTTY` sentinel mapped in `translateExit` + `shimResolveErr` | Clarified — user confirmed (Open Design Point 1 resolved) | S:95 R:70 A:75 D:70 |
| 9 | Certain | `--json --trees` non-cloned repo → `cloned: false` (omit `worktrees`); per-repo `wt list` failure → an `error` string field; empty list → `[]`; never abort the array | Clarified — user confirmed | S:95 R:65 A:75 D:68 |
| 10 | Certain | Hint suppression under `HOP_WRAPPER` removes the hint TEXT only, keeping the exit-2 usage code | Clarified — user confirmed | S:95 R:72 A:80 D:75 |
| 11 | Certain | TTY detection seam = a `cmd/hop` package-level `isTTY` var (mirroring `idea`'s `IsTTY` and the existing `pickResolve`/`pickOne`/`listWorktrees` idiom), not a new `internal/tty` package | Clarified — user confirmed | S:95 R:72 A:80 D:70 |
| 12 | Tentative | `cli/agent-non-interactive-usage` lands as a NEW memory file vs folding into the existing three `cli/` files | Deferred — user deferred to hydrate (genuinely a hydrate-time memory-shaping call with two valid options; near-zero blast radius but legitimately two-way) | S:50 R:78 A:58 D:48 |

12 assumptions (11 certain, 0 confident, 1 tentative, 0 unresolved).
