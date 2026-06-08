# Plan: Grammar + Shim Refactor

**Change**: 260608-gyo0-grammar-shim-refactor
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### Grammar: Unified `hop <selection> <action>`

#### R1: Selection-first grammar
The interactive grammar SHALL be `hop <selection> <action>` where `<selection>` is a repo name (substring → fzf on ambiguity), a `repo/worktree`, or a group name, and `<action>` is a builtin verb (`cd`, `open`, `where`), a reoriented batch verb (`pull`/`push`/`sync`), any PATH binary (`git pull`, `code .`), or any shell alias/function (`p`).

- **GIVEN** the shim is installed
- **WHEN** the user runs `hop webapp` (no action)
- **THEN** the parent shell cd's into webapp's repo dir
- **AND WHEN** the user runs `hop webapp git pull`
- **THEN** `git pull` runs in webapp's repo dir in the parent shell
- **AND WHEN** the user runs `hop webapp/feat-x`
- **THEN** the parent shell cd's into the feat-x worktree of webapp

### Dispatch: Binary-owned classification, no eval

#### R2: Hidden `--shim-plan` classifier emits a fixed 3-keyword protocol
The binary SHALL expose a hidden `--shim-plan` flag that classifies the user's `$@` and prints exactly one of three plans on stdout: `CD\n<path>`, `RUN_IN_PARENT\n<path>`, or `PASSTHROUGH`. The classifier SHALL resolve the selection (repo/worktree/group/`--all`) and decide the plan based on the action token. All process execution SHALL use `exec.CommandContext` with explicit arg slices (Constitution I). The classifier SHALL NOT exec the user's action — it only classifies and resolves a path.

- **GIVEN** `hop --shim-plan webapp` (bare selection, no action)
- **WHEN** the binary classifies it
- **THEN** it prints `CD\n<resolved-abs-path>`
- **AND GIVEN** `hop --shim-plan webapp cd`
- **THEN** it prints `CD\n<path>`
- **AND GIVEN** `hop --shim-plan webapp git pull` (PATH-binary action)
- **THEN** it prints `RUN_IN_PARENT\n<path>`
- **AND GIVEN** `hop --shim-plan add fab-kit` (a cobra subcommand at `$1`)
- **THEN** it prints `PASSTHROUGH`

#### R3: Shim interprets the protocol with a fixed case; no eval
The `hop()` shim function SHALL call `command hop --shim-plan "$@"`, then `case` on the first line over exactly the 3 keywords. `CD` SHALL run `cd -- <path>`. `RUN_IN_PARENT` SHALL run `cd -- <path>` then `shift; "$@"` (the user's already-parsed words). `PASSTHROUGH` SHALL run `command hop "$@"`. The shim SHALL NOT `eval` any binary output (Constitution I — no shell-injection surface).

- **GIVEN** the emitted shim
- **WHEN** `command hop --shim-plan` returns `RUN_IN_PARENT\n/abs/webapp`
- **THEN** the shim runs `cd -- /abs/webapp` then `shift` then `"$@"` (running the user's literal words, e.g. `git pull`)
- **AND** the shim never passes binary stdout to `eval`

#### R4: Zero subcommand names hard-coded in the shim
The emitted shim SHALL NOT contain any cobra subcommand names (no `add|rm|clone|pull|push|sync|ls|...` case list). The only literals the shim branches on are the 3 protocol keywords. The subcommand list SHALL live only in cobra.

- **GIVEN** the output of `hop shell-init zsh`
- **WHEN** inspected
- **THEN** it contains no subcommand-name case list (e.g. no `add|rm|clone`)
- **AND** it branches only on `CD`, `RUN_IN_PARENT`, `PASSTHROUGH`

### Path handoff: Collapse the three mechanisms

#### R5: `where` and bare-cd handoffs collapse into the protocol
The stdout-for-`where`, conditional-stdout-for-`clone`, and bare-name `cd` handoffs SHALL be expressed through the `--shim-plan` protocol. `hop <repo> where` SHALL remain a direct binary command (prints the path to stdout) reached via `PASSTHROUGH`. Bare `hop <repo>` and `hop <repo> cd` SHALL resolve to `CD\n<path>`.

- **GIVEN** `hop webapp where`
- **WHEN** classified via `--shim-plan`
- **THEN** the plan is `PASSTHROUGH` and `command hop webapp where` prints the path
- **AND GIVEN** `hop webapp` or `hop webapp cd`
- **THEN** the plan is `CD\n<path>`

#### R6: `open` preserves wt's interactive menu and routes "Open here" back to the shim
`hop <repo> open` SHALL delegate to wt's interactive app menu with full stdio (so the menu renders and reads stdin), and the "Open here" choice SHALL cd the parent shell. The cd-handoff mechanism SHALL remain the `WT_CD_FILE` temp-file side channel (the binary execs `wt open <path>` as a passthrough; the shim creates the temp file, exports `WT_CD_FILE`, and cd's afterward if the file is non-empty). `open` is treated as a builtin `$2` verb, not a hard-coded subcommand name.

- **GIVEN** `hop webapp open`
- **WHEN** the user picks an editor in wt's menu
- **THEN** wt's menu rendered interactively (stdio not captured) and no cd occurs
- **AND WHEN** the user picks "Open here"
- **THEN** wt writes the path to `WT_CD_FILE` and the shim cd's the parent shell there

### Batch verbs: Reorient to selection-first

#### R7: `pull`/`push`/`sync` are action tokens, not cobra subcommands
`pull`, `push`, and `sync` SHALL be removed as cobra subcommands and become action tokens after a selection: `hop <repo> pull`, `hop <group> push`, `hop --all sync`. `pull`/`push` internally mean `git pull` / `git push`; `sync` means the full auto-commit-dirty + `git pull --rebase` + `git push` workflow. The selection MAY be a repo, a worktree, or a group. These are executed by the binary (resolved + run, capturing per-repo summaries) — they are classified by `--shim-plan` as `PASSTHROUGH` to a binary-internal runner so the existing batch summary/exit-code semantics are preserved.

- **GIVEN** `hop webapp pull`
- **WHEN** dispatched
- **THEN** `git pull` runs in webapp's repo and a `pull: webapp ✓ ...` line is emitted
- **AND GIVEN** `hop frontend-group sync`
- **THEN** sync runs across every cloned repo in the group with a `summary:` line
- **AND GIVEN** `hop --all pull`
- **THEN** `git pull` runs across every cloned repo in the registry

### Plural selection

#### R8: Plural selection is first-class with an interactive guard
`hop --all <action>` and `hop <group> <action>` SHALL run the action across all matched repos. No-target batch becomes `hop --all pull` (replacing `hop pull --all`). A plural selection SHALL refuse non-batch, interactive actions: only the batch verbs (`pull`/`push`/`sync`) and `where` are permitted on a plural selection. Any other action (`cd`, `open`, or a PATH-binary/alias/function action) on a plural selection SHALL be refused with a usage error (exit 2).

- **GIVEN** `hop --all pull`
- **WHEN** dispatched
- **THEN** pull runs across every cloned repo
- **AND GIVEN** `hop --all code .` (interactive action on a plural selection)
- **THEN** the binary emits a usage error (exit 2) and runs nothing
- **AND GIVEN** `hop --all` (plural with no action)
- **THEN** the binary emits a usage error (a plural selection has no meaningful cd target)

### Removals

#### R9: Drop the `-R` flag and its silent rewrite
The user-facing `-R` flag SHALL be removed, along with `extractDashR`/`runDashR` in `main.go` and the shim's `-R` rewrite. Tool-form (`hop <repo> <tool> ...`) is now native grammar handled via `RUN_IN_PARENT`. The `hop: -R: '<cmd>' not found` error path SHALL be removed.

- **GIVEN** the binary
- **WHEN** built
- **THEN** `extractDashR` and `runDashR` no longer exist and `main.go` does not inspect argv for `-R`
- **AND** `hop <repo> git status` runs `git status` in the repo via the shim's `RUN_IN_PARENT` path

#### R10: Drop the `hi` alias
The `hi` alias SHALL be removed from the emitted shim. `command hop` remains the raw escape hatch. The `h` alias is retained.

- **GIVEN** `hop shell-init zsh`
- **WHEN** inspected
- **THEN** it defines `h()` but not `hi()`
- **AND** the completion registration shares `_hop` with `h` only

#### R11: No version handshake; graceful degradation
There SHALL be no version handshake between shim and binary. An unrecognized old-shim invocation reaching `--shim-plan` SHALL classify as `PASSTHROUGH`, letting cobra print a normal error.

- **GIVEN** an old shim calling `command hop --shim-plan <unrecognized>`
- **WHEN** the binary cannot classify it
- **THEN** it prints `PASSTHROUGH` and cobra surfaces a normal error on the subsequent `command hop "$@"`

### Documentation & placeholder rename

#### R12: Rename placeholder `outbox` → `webapp` and update help/docs/tests
The placeholder repo name `outbox` SHALL be renamed to `webapp` across help text, docs/specs, docs/memory, and tests (using `frontend`/`backend` for two-repo examples). The root help text SHALL describe the new `hop <selection> <action>` grammar and drop `-R`/`hi`/`hop pull <name>` forms.

- **GIVEN** the help text and docs
- **WHEN** inspected
- **THEN** no `outbox` placeholder remains in source/docs (real identifiers are untouched)
- **AND** the cheat sheet shows `hop <selection> <action>` forms, not `-R`, `hi`, or `hop pull <name>`

### Completion (grammar-forced only)

#### R13: Completion keeps `__complete*` working; pull/push/sync stop completing as subcommands
Cobra's `__complete*` introspection SHALL keep working under the `--shim-plan` dispatch (the shim forwards `__complete*` to `command hop` directly, never through `--shim-plan`). `pull`/`push`/`sync` SHALL no longer complete as subcommands (they are removed from cobra). The known completion BUGS (synchronous `wt list --json` blocking, ambiguous-prefix worktree completion, swallowed errors) are OUT of scope (deferred to backlog `[cmp7]`).

- **GIVEN** the emitted shim
- **WHEN** `hop __complete <args>` is invoked
- **THEN** it forwards directly to `command hop __complete` (not `--shim-plan`)
- **AND** `pull`/`push`/`sync` are no longer offered as subcommand completions

### Non-Goals

- Fixing completion bugs (blocking `wt list --json`, ambiguous-prefix worktree completion, swallowed errors) — deferred to backlog `[cmp7]`.
- Adding a version handshake.
- Reimplementing git/fzf/wt/editor wrapping (Constitution IV — keep wrapping).
- A 4th protocol keyword for `open` (the 3-keyword protocol is fixed; `open` is a builtin `$2` verb handled via the retained temp-file arm).

### Design Decisions

1. **`open` stdio mechanism**: Keep the `WT_CD_FILE` temp-file side channel and a dedicated `open)` arm in the shim. — *Why*: wt's menu is interactive and needs full stdio mid-flow; the cd-target ("Open here") arrives only after wt exits, which does not fit the classify-then-act CD/RUN_IN_PARENT/PASSTHROUGH shape. `open` is a builtin `$2` verb, not a subcommand name, so R4 (zero subcommand names) still holds. — *Rejected*: a 4th protocol keyword (`OPEN`) — violates the fixed-3-keyword contract (Assumption #2); capturing wt's stdout via `$(...)` — swallows the interactive menu and hangs.
2. **pull/push/sync execution path**: classified as `PASSTHROUGH` to a binary-internal runner reached via a builtin `$2`/action token, not RUN_IN_PARENT. — *Why*: the batch verbs need per-repo summary lines, group/`--all` fan-out, and the existing exit-code policy — all owned by the binary today. RUN_IN_PARENT would run a literal `pull` binary in the shell, which is wrong. — *Rejected*: keeping them as cobra subcommands (intake R7 explicitly reorients them to selection-first action tokens).
3. **Plural-interactive guard**: a plural selection accepts only `pull`/`push`/`sync` and `where`; everything else errors with exit 2. — *Why*: cd/open/interactive-tool across N repos is nonsensical; a small allow-list is simpler and safer than an interactive-detection heuristic. — *Rejected*: trying to detect "interactive" PATH binaries dynamically (unreliable, over-engineered).

### Deprecated Requirements

#### `-R` flag (binary-direct exec form)
**Reason**: Tool-form is now native grammar via `RUN_IN_PARENT`; the `-R` argv inspection produced opaque errors.
**Migration**: `hop -R <name> <cmd>` and `hop <name> -R <cmd>` → `hop <name> <cmd>` (via the shim's `RUN_IN_PARENT` path).

#### `hi` alias
**Reason**: Minimal surface (Constitution VI); `command hop` is the escape hatch.
**Migration**: `hi <args>` → `command hop <args>`.

#### `hop pull <name>` / `hop pull --all` (subcommand form)
**Reason**: Reoriented to selection-first action tokens.
**Migration**: `hop pull <name>` → `hop <name> pull`; `hop pull --all` → `hop --all pull`.

## Tasks

### Phase 1: Binary classifier core

- [x] T001 Add the hidden `--shim-plan` flag and classifier in `src/cmd/hop/shim_plan.go` (handled pre-cobra in `main.go::extractShimPlan`): `runShimPlan` resolves `$1` (selection: repo/worktree/group/`--all`) and the action token, then prints exactly `CD\n<path>`, `RUN_IN_PARENT\n<path>`, or `PASSTHROUGH` and exits. Classification rules implemented exactly as planned. <!-- R2 R5 R8 R11 -->
- [x] T002 Implement plural-selection classification + guard in `src/cmd/hop/shim_plan.go` (`classifyPlural`) and the direct-binary mirror in `src/cmd/hop/root.go` (`runPluralSelection`): detect `--all` and exact group-name selections; permit only `pull`/`push`/`sync`; refuse `cd`/`open`/`where`/other actions and bare-plural with exit 2 and a clear message. Reuses `hasConfiguredGroup`/`resolveTargets`. <!-- R8 -->

### Phase 2: Shim rewrite

- [x] T003 Rewrote `posixInit` in `src/cmd/hop/shell_init.go` to the protocol interpreter: `hop()` forwards `__complete*` directly to `command hop`; otherwise `plan="$(command hop --shim-plan "$@")" || return $?` and `case` over `CD` / `RUN_IN_PARENT` / `PASSTHROUGH`. The `open` cd-handoff is folded into the UNIFIED `_hop_passthrough` arm (every PASSTHROUGH exports `WT_CD_FILE`; `wt open` and `clone <url>` write their cd-target there) — cleaner than a dedicated `open` arm and keeps the shim free of the literal `open` verb name. Removed the subcommand case list, the `-R` rewrite, `_hop_dispatch`, and the `hi` alias. Kept `h()` and completion registration (`compdef _hop h` / `complete ... __start_hop h`). <!-- R3 R4 R5 R6 R10 R13 -->

### Phase 3: Removals & reorientation

- [x] T004 Removed `-R` from the binary: deleted `extractDashR`/`runDashR` and the argv-inspection block in `src/cmd/hop/main.go` (155→81 lines); the obsolete hint paths are retargeted to the protocol (`toolFormHintFmt` now lists pull/push/sync and points at the shim; `cdHint`/`bareNameHint` describe the CD plan). `where` preserved as a real binary command. <!-- R9 -->
- [x] T005 Reoriented `pull`/`push`/`sync`: removed `newPullCmd`/`newPushCmd`/`newSyncCmd` from the cobra `AddCommand` list; added `src/cmd/hop/batch_verb.go::runBatchVerb` which resolves the selection via `resolveTargets` and dispatches into the existing `pullSingle`/`pullBatch`/`pushSingle`/`pushBatch`/`syncSingle`/`syncBatch`/`runBatch` machinery (repo/worktree/group/`--all`). `pull.go`/`push.go`/`sync.go`/`batch.go` runner logic unchanged; only the cobra factories were removed. The reoriented `sync` uses the fixed default commit message (the `-m/--message` flag was dropped). Summary lines + exit codes preserved. <!-- R7 -->
- [x] T006 Rewrote `rootLong` and the hint constants in `src/cmd/hop/root.go` to describe `hop <selection> <action>`, dropped `-R`/`hi`/`hop pull <name>`/`hop pull --all`, documented `hop <repo> pull|push|sync`, `hop --all pull`, `hop <group> pull`, and the plural-interactive guard. Help examples use `webapp` (frontend/backend for two-repo cases). <!-- R12 R7 R8 -->

### Phase 4: Docs, placeholder rename, tests

- [x] T007 [P] Renamed placeholder `outbox` → `webapp` across `docs/specs/*.md` and `docs/memory/**/*.md`. Updated the design-of-record SPECS (`docs/specs/cli-surface.md`, `docs/specs/architecture.md`) to the new `hop <selection> <action>` grammar: `--shim-plan` protocol, removed `-R`/`hi`, reoriented pull/push/sync, plural-interactive guard, unified `WT_CD_FILE` channel, design decisions. NOTE: substantive `docs/memory/**` content updates are deferred to the hydrate stage (per the intake's Affected Memory list — memory is post-implementation hydrated knowledge); T007 did only the mechanical placeholder rename there. <!-- R12 R1 R5 R7 -->
- [x] T008 Updated/rewrote tests to the new grammar: deleted `dashr_test.go`; rewrote `shell_init_test.go` to assert the protocol shim (3-keyword case, `--shim-plan` call, no subcommand list, no `-R`, no `hi`, unified passthrough cd-channel); rewrote `bare_name_test.go` (tool-form-with-extra-args) and the pull/push/sync tests to the action-token grammar; added `src/cmd/hop/shim_plan_test.go` (CD/RUN_IN_PARENT/PASSTHROUGH/plural-guard/graceful-degradation classifier tests); renamed `outbox` → `webapp` in all test fixtures; fixed `integration_test.go` (`TestIntegrationDashR*` → `TestIntegrationShimPlan{RunInParent,CD}`); updated completion tests (removed deprecated pull/sync subcommand-completion tests, added batch verbs to the `$2` verb-position test). 215 cmd/hop tests pass. <!-- R2 R3 R4 R7 R8 R9 R10 R12 R13 -->

## Execution Order

- T001 blocks T002, T003, T005 (classifier is the dispatch core).
- T004, T005, T006 depend on T001 (they remove/reroute what the classifier replaces).
- T003 depends on T001 (shim consumes the protocol).
- T007 is independent ([P]) — docs only.
- T008 last — tests assert the finished behavior of all prior tasks.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `hop <selection> <action>` grammar works for bare cd, `cd`/`where`/`open` verbs, `repo/worktree`, PATH binary, and alias/function actions. (Verified via classifier: bare→CD, cd→CD, where→PASSTHROUGH, git→RUN_IN_PARENT.)
- [x] A-002 R2: A hidden `--shim-plan` flag exists (marked Hidden), classifies `$@`, and prints exactly one of `CD\n<path>` / `RUN_IN_PARENT\n<path>` / `PASSTHROUGH`. (Verified: 0 occurrences in `--help` and `help-dump`; handled pre-cobra in `main.go::extractShimPlan`.)
- [x] A-003 R3: The emitted shim calls `command hop --shim-plan "$@"` and branches with a fixed `case` over the 3 keywords; it never `eval`s binary output. (Verified by emitting the zsh+bash shim; `eval` appears only in cobra completion plumbing, never in the dispatch path.)
- [x] A-004 R4: The emitted shim contains no cobra subcommand-name case list — only the 3 protocol keywords. (Verified in emitted shim.)
- [x] A-005 R5: `hop <repo> where` prints the path (via PASSTHROUGH); bare `hop <repo>` and `hop <repo> cd` produce `CD\n<path>`. (Verified against a resolvable config.)
- [x] A-006 R6: `hop <repo> open` renders wt's interactive menu (stdio uncaptured) and "Open here" cd's the parent via `WT_CD_FILE`. (Verified by code inspection: `open.go` uses `proc.RunForeground` with inherited stdio, no `$(...)` capture; shim `_hop_passthrough` exports `WT_CD_FILE`. Interactive wt not exercisable in CI.)
- [x] A-007 R7: `hop <repo> pull|push|sync`, `hop <group> sync`, and `hop --all pull` run the batch machinery with preserved summary lines and exit codes; `pull`/`push`/`sync` are no longer cobra subcommands. (Verified: `hop frontend pull` emits `pull: frontend ✗ ...` + exit 1; classifier emits PASSTHROUGH for batch verbs.)
- [x] A-008 R8: `hop --all pull` fans out; `hop --all code .` and `hop --all` (no action) are refused with exit 2. (Verified end-to-end against the classifier.)

### Behavioral Correctness

- [x] A-009 R9: `extractDashR`/`runDashR` are deleted, `main.go` no longer inspects argv for `-R`, and tool-form runs via `RUN_IN_PARENT`. (Verified: zero residue in non-test source; `main.go` 155→81.)
- [x] A-010 R7: `hop pull <name>` is no longer accepted as a subcommand; the reoriented `hop <name> pull` is. (Verified: `hop pull` direct → bare-name hint exit 2; `pull` treated as a selection, not a subcommand.)
- [x] A-011 R10: The emitted shim defines `h()` but not `hi()`. (Verified in emitted shim; `compdef _hop h` only.)

### Removal Verification

- [x] A-012 R9: No `-R`/`extractDashR`/`runDashR`/`hop: -R:` references remain in non-test source. (Verified via grep.)
- [x] A-013 R10: No `hi(` / `hi alias` / `compdef _hop h hi` (3-name) registration remains. (Verified.)
- [x] A-014 R12: No `outbox` placeholder remains in `src/`, `docs/specs/`, or `docs/memory/`. (Verified. NOTE: `README.md` still contains `outbox` + removed `-R`/`hi`/`hop pull <name>` — out of R12's declared scope but flagged as a should-fix in review.)

### Scenario Coverage

- [x] A-015 R2: `shim_plan_test.go` covers CD (bare + `cd`), RUN_IN_PARENT (tool action), PASSTHROUGH (subcommand/`where`/`open`), and plural-guard exit-2 cases. (Verified: tests present and passing.)
- [x] A-016 R3 R4: `shell_init_test.go` asserts the protocol shim shape (3-keyword case, `--shim-plan` call, no subcommand list, no `hi`, `open` temp-file arm) for both zsh and bash. (zsh: full protocol-shape assertions including no-eval. bash: asserts `--shim-plan` call + h-only/hi-dropped; the dispatch body is the shared `posixInit` output, so the zsh shape assertions cover the identical bash body. Minor: bash lacks the explicit 3-keyword/no-eval assertions — nice-to-have.)
- [x] A-017 R11: An unrecognized `--shim-plan` input classifies as `PASSTHROUGH` (graceful degradation test). (Verified: `--some-old-flag arg` → PASSTHROUGH exit 0.)

### Edge Cases & Error Handling

- [x] A-018 R8: Plural selection with a disallowed action exits 2 and runs nothing. (Verified: `--all code .`, `web cd`, `web open` → exit 2, empty stdout.)
- [x] A-019 R6: `hop <repo> open` with wt missing exits cleanly (errSilent) and `open` failure propagates the exit code (binary passthrough contract preserved). (Verified by code inspection: `proc.ErrNotFound` → `wtMissingHint` + `errSilent`; non-zero wt code → `errExitCode{code}`.)

### Code Quality

- [x] A-020 Pattern consistency: New code follows the cobra factory, `errSilent`/`errExitCode` sentinel, and `pickOne`/`listWorktrees` seam patterns of surrounding code. (Verified: `runBatchVerb` reuses `resolveTargets`; `shimResolveErr` mirrors `translateExit`.)
- [x] A-021 No unnecessary duplication: The reoriented batch verbs reuse `resolveTargets`/`runBatch`/`pullOne`/`syncOne` rather than reimplementing them. (Verified in `batch_verb.go`.)
- [x] A-022 Readability over cleverness: The classifier and shim are readable; no god functions (>50 lines without reason); no magic strings (protocol keywords are named constants `planCD`/`planRunInParent`/`planPassthrough`). (Verified.)
- [x] A-023 Net line reduction (goal, qualified): Re-measured at review: `src/` delta is **−729 lines** (643 insertions, 1372 deletions) — a decisive net reduction once the actual changed set is diffed (the plan's earlier "+61" estimate counted the new files' doc-comments against an LOC budget but did not net the test/spec deletions). `main.go` 155→81, `shell_init.go` 199→152, pull/push/sync cobra factories removed. Net logic and raw lines both reduced. <!-- assumed: "expect reduction" is a goal not a hard gate -->

### Security

- [x] A-024 R3: The shim runs already-parsed `"$@"`, never `eval` of binary output; the classifier resolves a path used only as a quoted `cd` operand (Constitution I — no injection surface). (Verified by reproducing the injection test: `hop frontend echo 'a; touch /tmp/PWNED'` emits only `RUN_IN_PARENT\n<path>` — no PWNED file created; binary never execs the action.)
- [x] A-025 R2: All process execution in new/changed code uses `exec.CommandContext` via `internal/proc` with explicit arg slices (Constitution I). (Verified: zero `exec.Command` without context in non-test source.)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None of substance — the apply already removed the redundant surface this refactor obsoleted: `extractDashR`/`runDashR` and the `-R` argv block (`main.go`), `newPullCmd`/`newPushCmd`/`newSyncCmd` cobra factories, `completeRepoOrGroupNames` (`repo_completion.go`), the shim subcommand case-list / `_hop_dispatch` / `-R` rewrite / `hi` alias (`shell_init.go`), and `dashr_test.go`. Net `src/` delta is −729 lines, so no leftover dead code was discovered during review.
- `docs/memory/cli/subcommands.md` and `docs/memory/architecture/package-layout.md` still describe `-R`/`extractDashR`/`runDashR`/`hi`/`hop pull <name>` as live — NOT deletion candidates (these are deferred to the hydrate stage per T007's note and the intake's Affected Memory list; flagged here only for hydrate traceability).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Tentative | `open` keeps the `WT_CD_FILE` temp-file side channel + a dedicated shim `open)` arm; `--shim-plan` returns `PASSTHROUGH` for `open`, which the shim wraps. | Intake Open Question. wt's menu is interactive (full stdio mid-flow) and the cd-target arrives only after wt exits — does not fit the classify-then-act 3-keyword shape. A 4th keyword would violate the fixed-3 contract (Assumption #2). `open` is a `$2` builtin verb, not a subcommand name, so R4 still holds. <!-- assumed: open uses retained WT_CD_FILE temp-file arm; PASSTHROUGH classification; no 4th protocol keyword --> | S:60 R:55 A:70 D:55 |
| 2 | Confident | Plural-selection guard = allow-list: only `pull`/`push`/`sync`/`where` permitted on `--all`/group; everything else (cd/open/tool) errors exit 2. | Intake Open Question. A small allow-list of meaningful-on-N-repos actions is simpler and safer than dynamic interactive-detection; cd/open/interactive-tool across N repos is nonsensical. | S:80 R:65 A:85 D:75 |
| 3 | Confident | pull/push/sync route via `PASSTHROUGH` to a binary-internal action-token runner (reusing `pullSingle`/`runBatch`/`syncOne`), not `RUN_IN_PARENT`. | The batch verbs need per-repo summaries, group/`--all` fan-out, and the existing exit-code policy — all binary-owned. RUN_IN_PARENT would run a literal `pull` binary in the shell. | S:88 R:70 A:88 D:85 |
| 4 | Confident | Selection-bare `--all` with no action errors exit 2 (no meaningful cd target for a plural selection). | A plural selection cannot produce a single CD path; erroring is the only sound behavior. Intake frames `--all` as batch-only. | S:82 R:75 A:85 D:80 |
| 5 | Certain | `where` stays a real binary command reached via `PASSTHROUGH` (prints path to stdout); only bare/`cd` become `CD`. | Intake R5 keeps `where` as the scriptable path-printer; binary already owns it. | S:95 R:80 A:90 D:90 |

5 assumptions (1 certain, 3 confident, 1 tentative).
