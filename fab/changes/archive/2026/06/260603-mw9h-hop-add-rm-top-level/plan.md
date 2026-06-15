# Plan: Promote `add` and `rm` to top-level commands

**Change**: 260603-mw9h-hop-add-rm-top-level
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### CLI: Top-level `hop add`

#### R1: `hop add <dir>` is a pure promotion of `hop config add <dir>`
hop SHALL expose a top-level `add` subcommand that classifies a single on-disk directory and registers its remote URL into `hop.yaml`, with behavior, exit codes, and output IDENTICAL to today's `hop config add <dir>` — the same `runConfigAdd` body MUST back both spellings (no logic duplication).

- **GIVEN** a `hop.yaml` exists and `<dir>` is a normal git repo with a remote not yet registered
- **WHEN** the user runs `hop add <dir>`
- **THEN** the URL is merged into `hop.yaml` and stderr shows `added: <url>` + `wrote: <path>`, exit 0
- **AND** running `hop config add <dir>` against the same setup produces the identical merge outcome

#### R2: `hop add <dir>` preserves the exit-code and forgiving-no-op contract
`hop add` SHALL exit 2 on a bad dir argument, exit 1 on missing `hop.yaml` / load / write / `git`-missing failure, and exit 0 on success AND on every forgiving no-op (non-git dir, worktree/bare/no-remote candidate, already-registered URL) with the corresponding stderr message.

- **GIVEN** `<dir>` is a plain (non-git) directory and `hop.yaml` exists
- **WHEN** the user runs `hop add <dir>`
- **THEN** stderr shows the "is not a git repo" forgiving message and exit is 0, `hop.yaml` byte-unchanged
- **AND** **GIVEN** `<dir>` is not a directory **WHEN** `hop add <dir>` runs **THEN** exit is 2

### CLI: Top-level `hop rm` (promotion + positional)

#### R3: `hop rm` (no positional) is a pure promotion of `hop config rm`
hop SHALL expose a top-level `rm` subcommand that, with zero positionals, behaves IDENTICALLY to today's `hop config rm` — load registry, optional `--stale` pre-filter, fzf picker (the `pickOne` seam), path-column map-back, `yamled.RemoveURL`. The same `runConfigRm` body MUST back the bare path of both spellings.

- **GIVEN** a `hop.yaml` with two registered repos
- **WHEN** the user runs `hop rm` and selects an entry in the picker
- **THEN** stderr shows `removed: <url>` + `wrote: <path>`, that URL is dropped, exit 0
- **AND** the exit-code contract matches `config rm`: exit 1 (fzf-missing / no-config / load / write failure), 130 (fzf cancel), 0 (success and forgiving no-ops)

#### R4: `hop rm <name>` resolves the named repo and removes it directly, skipping the picker
hop SHALL accept an optional positional on `rm` (`Args: cobra.MaximumNArgs(1)`). With one positional, hop MUST resolve `<name>` to a single `*repos.Repo` via the existing `resolveByName` (match-or-fzf) helper and call `yamled.RemoveURL(configPath, repo.Group, repo.URL)` directly — the picker (`pickOne`) MUST NOT be invoked on this path.

- **GIVEN** a `hop.yaml` registering a repo whose name uniquely substring-matches `<name>`
- **WHEN** the user runs `hop rm <name>`
- **THEN** `resolveByName` resolves it, `RemoveURL` drops the entry, stderr shows `removed: <url>` + `wrote: <path>`, exit 0, and the fzf picker seam is never called

#### R5: `hop rm <name>` removes the registry entry regardless of on-disk existence and does NOT prompt
`hop rm <name>` MUST remove the matched entry as a pure YAML edit with NO on-disk `Stat` check on the positional path and NO confirmation prompt (parity with the picker's implicit confirmation).

- **GIVEN** a registered repo whose on-disk folder has been deleted
- **WHEN** the user runs `hop rm <name>`
- **THEN** the entry is removed (no "not cloned" error, no prompt), exit 0

#### R6: `--stale` combined with a positional name is a usage error (exit 2)
`hop rm` MUST reject the combination of `--stale` and a positional `<name>` as a usage error (exit 2) with a hop-emitted stderr message; `--stale` stays a picker-only scoping flag.

- **GIVEN** the user supplies both `--stale` and a positional name
- **WHEN** `hop rm <name> --stale` runs
- **THEN** hop emits a usage-error message to stderr and exits 2, without resolving or removing anything

### CLI: Migration via hidden aliases

#### R7: `hop config add` / `hop config rm` survive as hidden aliases sharing the same RunE
The `config` subtree MUST keep `add` and `rm` as hidden cobra subcommands (`Hidden: true`) that run the SAME `runConfigAdd` / `runConfigRm` bodies as the top-level commands. They MUST disappear from `hop --help`, `hop config --help`, and self-filter from `hop help-dump` JSON (via the existing `shouldSkipChild` Hidden drop).

- **GIVEN** the migrated binary
- **WHEN** the user runs `hop config add <dir>` or `hop config rm`
- **THEN** the command executes its historical behavior unchanged (zero breakage)
- **AND** neither `add` nor `rm` appears under `config` in `hop config --help` output, and `help-dump` omits them from the `config` node's children

#### R8: Canonical stderr prefix is per-path
Canonical top-level commands MUST emit `hop add:` / `hop rm:` as their stderr command-name prefix, while the hidden aliases MUST keep emitting the historical `hop config add:` / `hop config rm:` prefix. The prefix MUST be parameterized per invocation path — NOT a single hardcoded module constant baked into the shared body.

- **GIVEN** a not-a-directory argument
- **WHEN** invoked as `hop add <bad>` **THEN** stderr reads `hop add: '<bad>' is not a directory.`
- **AND** **WHEN** invoked as `hop config add <bad>` **THEN** stderr reads `hop config add: '<bad>' is not a directory.`

#### R9: The `config` parent summary drops `add, rm`
The `config` parent command's `Short` MUST no longer advertise `add, rm` (they are no longer the documented home).

- **GIVEN** the migrated binary
- **WHEN** the user views `hop config --help` or `hop --help`
- **THEN** the `config` one-line summary lists `init, where, scan, print` (and not `add`/`rm`)

### CLI: Grammar / shim / completion / help wiring

#### R10: Root command wires the new top-level factories
`root.go::newRootCmd()` MUST `AddCommand` the new `newAddCmd()` and `newRmCmd()` factories alongside the existing subcommands.

- **GIVEN** the migrated binary
- **WHEN** the user runs `hop add` / `hop rm`
- **THEN** cobra dispatches to the new top-level commands (not the repo-first RunE fallback)

#### R11: The shim's known-subcommand list gains `add|rm`
`shell_init.go::posixInit`'s known-subcommand case MUST include `add|rm` so the shim routes `hop add`/`hop rm` through `_hop_dispatch` instead of misrouting them through the repo-first "otherwise" branch.

- **GIVEN** the emitted shim is sourced
- **WHEN** the user runs `hop rm` (no positional) or `hop add ~/x`
- **THEN** the shim invokes `command hop rm` / `command hop add ~/x` (the binary's real subcommands), not `_hop_dispatch cd "rm"` or `command hop -R add ~/x`

#### R12: `rootLong` Usage table reflects the new top-level commands
The `rootLong` help text MUST add `hop add <dir>` and `hop rm [<name>]` rows to the `Usage:` table and remove the `hop config add` / `hop config rm` rows.

- **GIVEN** the migrated binary
- **WHEN** the user runs `hop --help`
- **THEN** the Usage table lists `hop add <dir>` and `hop rm [<name>]` and omits `config add` / `config rm`

#### R13: `repo_completion.go` subNames filter absorbs `add`/`rm` with no code change
The dynamically-built `subNames` collision filter MUST pick up `add` and `rm` automatically (since it is built from `cmd.Commands()`), so a repo literally named `add`/`rm` is dropped from bare-name completion. NO code change is required here; this is a verify-only requirement.

- **GIVEN** a `hop.yaml` registering a repo literally named `rm`
- **WHEN** the root command's `$1` completion runs
- **THEN** `rm` is excluded from the bare-name candidate list (it resolves to the subcommand)

### Non-Goals

- No `internal/` package changes — all primitives (`scan.ClassifyOne`, `yamled.MergeScan`/`RemoveURL`, `repos.FromConfig`, `resolveByName`) are reused as-is.
- No new dependencies, no new flags beyond the `rm` positional.
- No new files — the top-level factories live in the existing `config_add.go` / `config_rm.go`.
- No confirmation prompt on `hop rm <name>`.
- No change to `hop config scan` / `init` / `where` / `print`.

### Design Decisions

1. **Migration: promote + hidden alias (not hard-remove, not document-both)**: top-level `add`/`rm` become canonical; `config add`/`config rm` stay as `Hidden: true` cobra commands sharing the exact RunE bodies. — *Why*: zero breakage for scripts/muscle-memory while adding discoverability, best fit with Constitution VI. — *Rejected*: hard removal (breaks callers); documenting both (worst surface-area fit).
2. **Per-path command-name prefix parameterization**: the shared `runConfigAdd`/`runConfigRm` bodies take the command-name string as a parameter (e.g., `runAdd(cmd, cmdName, userArg)`); top-level factories pass `"hop add"`/`"hop rm"`, hidden aliases pass `"hop config add"`/`"hop config rm"`. The module-level `addCmdName`/`rmCmdName` constants are removed in favor of factory-supplied values; `addSkipMessage` likewise takes the prefix as an argument. — *Why*: Assumption 8 requires divergent prefixes per path while DRY (Constitution VI) forbids duplicating the body; threading the prefix is the minimal seam. — *Rejected*: a single hardcoded const (cannot diverge); duplicating the RunE body (DRY violation, parsimony reviewer would flag).
3. **`hop rm <name>` reuses `resolveByName`, not the picker map-back**: the positional path resolves through the established name-or-fzf seam (`resolve.go::resolveByName`), consistent with `where`/`-R`/`open`/`clone`, then removes via `RemoveURL` directly. — *Why*: substring-match + fzf-on-ambiguity come for free and stay consistent with the rest of the CLI. — *Rejected*: re-implementing matching inside `rm` (duplicates resolution logic).
4. **`--stale` + positional rejected in RunE before resolution**: the factory's RunE checks `len(args) == 1 && stale` and returns `*errExitCode{code: 2}` before any resolution. — *Why*: `--stale` is a picker-scoping flag, contradictory with an explicit name (Assumption 9). — *Rejected*: silently ignoring `--stale` (hides a user error).

## Tasks

### Phase 1: Shared-body refactor (parameterize the command-name prefix)

- [x] T001 In `src/cmd/hop/config_add.go`, remove the module-level `addCmdName` const and rename `runConfigAdd(cmd, userArg)` to `runAdd(cmd, cmdName, userArg)`, threading `cmdName` through every stderr line (`validateConfigDir`, no-hop.yaml message, load error, classify error, `addSkipMessage`, already-registered, write error, and keep the literal `added:`/`wrote:` lines unchanged). Update `addSkipMessage` to take `cmdName` as its first parameter. <!-- R1 R2 R8 -->
- [x] T002 In `src/cmd/hop/config_rm.go`, remove the module-level `rmCmdName` const and rename `runConfigRm(cmd, stale)` to `runRm(cmd, cmdName, stale, name)` (new `name string` positional param), threading `cmdName` through every stderr line and `pickRepo` (`pickRepo(cmdName, rs)`). Keep the bare-picker path (empty `name`) byte-identical in behavior to today. <!-- R3 R8 -->

### Phase 2: New positional path for `rm <name>`

- [x] T003 In `src/cmd/hop/config_rm.go::runRm`, when `name != ""` resolve via `resolveByName(name)` (map `errFzfMissing` → `fzfMissingHint`+`errSilent`, propagate `errFzfCancelled`, surface `*errExitCode` verbatim), then call `yamled.RemoveURL(configPath, repo.Group, repo.URL)` directly (reusing the existing forgiving not-found / write-failure / success stderr handling), skipping `pickRepo`. No on-disk Stat, no prompt. <!-- R4 R5 -->

### Phase 3: Factories, aliases, grammar wiring

- [x] T004 In `src/cmd/hop/config_add.go`, add `newAddCmd()` (top-level, `Use: "add <dir>"`, `Args: cobra.ExactArgs(1)`, RunE → `runAdd(cmd, "hop add", args[0])`) and refactor `newConfigAddCmd()` to set `Hidden: true` and RunE → `runAdd(cmd, "hop config add", args[0])`. Both share `addLong`. <!-- R1 R7 R8 -->
- [x] T005 In `src/cmd/hop/config_rm.go`, add `newRmCmd()` (top-level, `Use: "rm [<name>]"`, `Args: cobra.MaximumNArgs(1)`, `--stale` flag) whose RunE rejects `--stale`+positional with `*errExitCode{code:2, msg:"hop rm: --stale cannot be combined with a repo name."}` then calls `runRm(cmd, "hop rm", stale, name)`; refactor `newConfigRmCmd()` to set `Hidden: true`, keep `Args: cobra.NoArgs`, RunE → `runRm(cmd, "hop config rm", stale, "")`. Both share `rmLong`. <!-- R4 R6 R7 R8 -->
- [x] T006 In `src/cmd/hop/config.go`, update `newConfigCmd()`'s `Short` to `"config helpers (init, where, scan, print)"` (drop `add, rm`). Keep `AddCommand(... newConfigAddCmd(), newConfigRmCmd() ...)` so the hidden aliases stay wired. <!-- R9 R7 -->
- [x] T007 In `src/cmd/hop/root.go::newRootCmd()`, add `newAddCmd()` and `newRmCmd()` to the `AddCommand(...)` call. <!-- R10 -->
- [x] T008 In `src/cmd/hop/shell_init.go::posixInit`, add `add|rm` to the known-subcommand case (the `clone|pull|push|sync|ls|shell-init|config|update|help|--help|-h|--version|completion)` line); update the adjacent doc comment listing known subcommands. <!-- R11 -->
- [x] T009 In `src/cmd/hop/root.go`, update `rootLong`'s Usage table: add `hop add <dir>` and `hop rm [<name>]` rows (near the registry-editing verbs), and remove any `config add` / `config rm` rows (there are none today — the table never listed them, so this is an add-only edit; verify). <!-- R12 -->

### Phase 4: Tests

- [x] T010 [P] In `src/cmd/hop/config_add_test.go`, add top-level `hop add` tests mirroring the existing `config add` cases: a convention-repo add (assert `added:`/`wrote:` and `hop.yaml` merge) and a not-a-directory case asserting the `hop add:` prefix (R8) and exit 2. Reuse `runArgs(t, "add", ...)` and the existing git-runner fakes. <!-- R1 R2 R8 -->
- [x] T011 [P] In `src/cmd/hop/config_rm_test.go`, add top-level `hop rm` (no-arg picker) test mirroring `TestConfigRmRemovesSelectedURL` via `runArgs(t, "rm")`; add a `hop rm <name>` positional test that injects a resolvable name and asserts direct `RemoveURL` WITHOUT the `pickOne` seam firing (set a `pickOne` fake that fails the test if called); add a `--stale`+positional usage-error test asserting exit 2. <!-- R3 R4 R5 R6 -->
- [x] T012 In `src/cmd/hop/integration_test.go`, add an end-to-end test that: runs `hop add <dir>` and `hop rm <name>` against a built binary with a fake git/wt environment; asserts `hop config add`/`hop config rm` still execute (hidden alias); and asserts `add`/`rm` are absent from `hop config --help` and present in `hop --help`. <!-- R1 R3 R4 R7 R12 -->
- [x] T013 In `src/cmd/hop/repo_completion.go`-adjacent tests (e.g., a new case in an existing `*_test.go`), verify a repo literally named `rm` is filtered out of bare-name completion candidates (subNames now includes `add`/`rm`). No production code change. <!-- R13 -->

### Phase 5: Build / vet / verify

- [x] T014 Run `cd src && go test ./cmd/hop/...`, then `go vet ./...` and a `gofmt -l` check; finally `go build ./... && go test ./...`. Fix any failures (tests conform to spec per Constitution Test Integrity). <!-- R1 R2 R3 R4 R5 R6 R7 R8 R9 R10 R11 R12 R13 -->

## Execution Order

- T001, T002 (shared-body refactor) block everything else.
- T003 depends on T002.
- T004 depends on T001; T005 depends on T002 + T003; T006/T007 depend on T004/T005.
- T008, T009 independent of the factory work (string edits) but land after for coherent build.
- T010-T013 depend on their respective implementation tasks.
- T014 last.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `hop add <dir>` registers a convention repo into `hop.yaml` with identical outcome to `hop config add <dir>`, backed by the shared `runAdd` body (no duplicated logic).
- [x] A-002 R2: `hop add` exits 2 on bad dir, 0 on forgiving no-ops (non-git dir leaves `hop.yaml` unchanged), with the documented stderr.
- [x] A-003 R3: `hop rm` (no positional) drives the fzf picker and removes the selection exactly as `hop config rm` does, backed by the shared `runRm` body.
- [x] A-004 R4: `hop rm <name>` resolves via `resolveByName` and removes directly via `RemoveURL` without invoking the `pickOne` seam.
- [x] A-005 R5: `hop rm <name>` removes a registry entry whose on-disk folder is absent, with no prompt and no "not cloned" error.
- [x] A-006 R6: `hop rm <name> --stale` exits 2 with a hop-emitted usage message, removing nothing.
- [x] A-007 R7: `hop config add`/`hop config rm` still execute (hidden), are absent from `hop config --help`, and self-filter from `help-dump`; both share the top-level RunE bodies.
- [x] A-008 R8: stderr prefixes are per-path — `hop add:`/`hop rm:` for canonical, `hop config add:`/`hop config rm:` for aliases; no single hardcoded module const remains.
- [x] A-009 R9: the `config` parent `Short` no longer lists `add, rm`.
- [x] A-010 R10: `root.go` wires `newAddCmd()`/`newRmCmd()`; cobra dispatches `hop add`/`hop rm` to them.
- [x] A-011 R11: `posixInit` known-subcommand case includes `add|rm`; the shim routes them through `_hop_dispatch`.
- [x] A-012 R12: `hop --help` Usage table lists `hop add <dir>` and `hop rm [<name>]` and omits `config add`/`config rm`.
- [x] A-013 R13: a repo literally named `rm` is excluded from bare-name completion (subNames collision), verified by test; no `repo_completion.go` code change.

### Behavioral Correctness

- [x] A-014 R4: the positional `rm <name>` path never calls `pickRepo`/`pickOne` (asserted by a fake that fails the test if invoked).
- [x] A-015 R8: an integration/unit assertion confirms `hop add: '<bad>' is not a directory.` vs `hop config add: '<bad>' is not a directory.` divergence.

### Scenario Coverage

- [x] A-016 R1 R3 R4 R7: integration test exercises `hop add <dir>`, `hop rm <name>`, and the hidden `hop config add`/`hop config rm` aliases end-to-end against a built binary.

### Edge Cases & Error Handling

- [x] A-017 R6: `--stale` + positional usage error (exit 2) is covered by a unit test.
- [x] A-018 R3: `hop rm` fzf-cancel (exit 130) and fzf-missing (exit 1) paths remain covered (existing `config rm` cases plus the top-level mirror inherit the shared body).

### Code Quality

- [x] A-019 Pattern consistency: new factories follow the `newXxxCmd()` convention, error sentinels (`errSilent`, `errExitCode`, `errFzfCancelled`), and `cmd.ErrOrStderr()` stderr style.
- [x] A-020 No unnecessary duplication: top-level and alias share the same RunE body via parameterized `runAdd`/`runRm` (DRY; reuses `resolveByName`, `validateConfigDir`, `buildScanPlan`, `RemoveURL`) — no copy-pasted command bodies.
- [x] A-021 Readability over cleverness: the per-path prefix is a plain string parameter, not a context-stash or reflection trick.
- [x] A-022 No magic strings: command-name prefixes (`"hop add"`, etc.) are supplied at the single factory call sites, not scattered.
- [x] A-023 No God functions: `runAdd`/`runRm` stay within the codebase's typical function size; the positional branch is a small, focused addition.

### Security

- [x] A-024 R4: no new subprocess execution introduced — `hop rm <name>` reuses `resolveByName` (which already wraps `fzf`/`wt` via `internal/proc`) and `yamled.RemoveURL` (pure YAML edit); Constitution I preserved.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)

## Deletion Candidates

- None — this change renames `runConfigAdd`→`runAdd` / `runConfigRm`→`runRm` in place and removes the `addCmdName`/`rmCmdName` module consts as part of the parameterization (already gone, no orphans). The hidden `config add`/`config rm` aliases are intentionally retained per the migration design (R7), so the old factories are not deletion candidates. No redundant code was discovered.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | The shared body takes the command-name prefix as a string parameter (`runAdd(cmd, cmdName, arg)` / `runRm(cmd, cmdName, stale, name)`); the `addCmdName`/`rmCmdName` module consts are removed. | Determined by Assumption 8 (Certain, intake) + DRY: per-path prefix divergence requires parameterization, and a single const cannot serve both paths. | S:95 R:80 A:90 D:85 |
| 2 | Certain | `--stale`+positional usage error wording is `hop rm: --stale cannot be combined with a repo name.` (exit 2). | Intake Assumption 9 fixes the exit code (2) and that it is a usage error; the exact wording follows the codebase's hop-emitted usage-message voice (concise, command-prefixed). Low blast radius, easily tweaked. | S:80 R:90 A:85 D:80 |
| 3 | Certain | `rootLong`'s Usage table never listed `config add`/`config rm` today, so R12 is an add-only edit (place `hop add`/`hop rm` rows among the registry-editing verbs); no rows are removed. | Verified by reading `root.go` — the table lists `config init/where/print/scan` but not `config add`/`config rm`. Determined by the source, no judgment. | S:95 R:90 A:95 D:95 |

3 assumptions (3 certain, 0 confident, 0 tentative, 0 unresolved).
