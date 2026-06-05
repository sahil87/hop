# Plan: Auto-init hop.yaml on write-commands

**Change**: 260605-44hm-auto-init-on-write
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

<!-- Derived from intake §What Changes (6 concrete changes) and the 11 graded
     intake assumptions (all Certain/Confident). Change A (260605-xgmu) has
     landed; ResolveWriteTarget already returns the fixed path without statting. -->

### Config: Skeleton Creation Helper

#### R1: EnsureSkeleton creates a minimal config when absent, never overwrites
`config.EnsureSkeleton(path string) (created bool, err error)` SHALL create a
minimal `hop.yaml` skeleton at `path` when the file is absent, and MUST be a
no-op when the file already exists. It mirrors `WriteStarter`'s refuse-to-clobber
posture but inverts the trigger (absence creates, presence is the no-op). The
skeleton content MUST be exactly the bytes `repos: {}\n` (no `config:` block, no
`code_root`, no header comment — Assumption 8). On create it MUST `os.MkdirAll`
the parent directory with mode `0o755` and write the skeleton with mode `0o644`.

- **GIVEN** no file at `path`
- **WHEN** `EnsureSkeleton(path)` is called
- **THEN** the parent dir is created (0755), the file is written with exactly
  `repos: {}\n` at mode 0644, and the function returns `(true, nil)`
- **AND GIVEN** the file already exists at `path`
- **WHEN** `EnsureSkeleton(path)` is called
- **THEN** the file is left byte-for-byte unchanged and the function returns
  `(false, nil)`
- **AND GIVEN** a stat error other than not-exist (e.g. a permission error on the
  path)
- **WHEN** `EnsureSkeleton(path)` is called
- **THEN** the error is returned (not swallowed)

### Config: Read-Command Not-Found Message

#### R2: Read-commands error on absence with the refined two-path hint
`config.Resolve()`'s not-found branch SHALL emit exactly the message (Assumption
11): `hop: no hop.yaml found at <path>. Run 'hop add <dir>' to register a repo
(creates the config), or 'hop config init' for a starter.` Read-commands (`hop`,
`hop ls`, `hop <name> where`, `hop config print`) MUST continue to error on a
missing config — they do NOT auto-init.

- **GIVEN** no config file at the fixed path
- **WHEN** a read-command resolves the config via `config.Resolve()`
- **THEN** it errors and the message names the fixed path and points at both
  `hop add <dir>` and `hop config init`

### CLI: `hop add` auto-init

#### R3: `hop add <dir>` auto-creates the config then registers
`hop add <dir>` (and its hidden alias `hop config add`) SHALL replace the
require-existing-config gate (the two-line "Run 'hop config init' first" message)
with auto-init: resolve the write target via `config.ResolveWriteTarget()`, call
`config.EnsureSkeleton(path)`, emit `created: <path>` to stderr when it created
the file, then `config.Load(path)` and proceed unchanged (classify →
buildScanPlan → MergeScan). The only remaining `ResolveWriteTarget` error
(`$HOME` unset) MUST still surface as an error with the command prefix.

- **GIVEN** no config at the fixed path on a fresh machine
- **WHEN** the user runs `hop add <dir>` on a normal git repo with a remote
- **THEN** the config is created (announced as `created: <path>` on stderr), the
  repo URL is merged, and the existing `added:`/`wrote:` lines are emitted
- **AND GIVEN** `$HOME` is unset
- **WHEN** the user runs `hop add <dir>`
- **THEN** the command errors with the `$HOME`-unset cause (not a misleading
  "no hop.yaml found")

### CLI: `hop config scan --write` auto-init

#### R4: `hop config scan --write` auto-inits only in write mode
`hop config scan --write` SHALL auto-init the skeleton (announced via
`created: <path>`) when the config is absent, then perform the merge. Print mode
(`hop config scan`, no `--write`) MUST keep erroring on absence (it never touches
the file, so there is nothing to create — Assumption 7).

- **GIVEN** no config at the fixed path
- **WHEN** the user runs `hop config scan <dir> --write`
- **THEN** the config is created (announced `created: <path>`) and the scan
  results are merged
- **AND GIVEN** no config at the fixed path
- **WHEN** the user runs `hop config scan <dir>` (no `--write`)
- **THEN** the command STILL errors with the scan-specific not-found message
  (print mode never creates)

### Yamled: Ensure-Group Helper

#### R5: yamled.EnsureGroup idempotently creates an empty group
`yamled.EnsureGroup(path, group string) error` SHALL load the YAML tree and, if
`repos.<group>` is absent, add an empty sequence node (`<group>: []`) under
`repos`, then write back atomically. It MUST be idempotent (no-op when the group
exists) and MUST preserve existing groups and comments. If the top-level `repos`
mapping is absent it is created (mirroring `mergeScanIntoTree`'s synthesize
behavior) so a fresh `repos: {}` skeleton is a valid target.

- **GIVEN** a config with `repos: {}` (no `default` group)
- **WHEN** `EnsureGroup(path, "default")` is called
- **THEN** `repos.default` exists as an empty sequence and the function returns nil
- **AND GIVEN** a config that already has the named group (with URLs)
- **WHEN** `EnsureGroup(path, group)` is called
- **THEN** the file is left unchanged (existing URLs and comments preserved)

### CLI: `hop clone <url>` auto-init

#### R6: `hop clone <url>` auto-creates the config and its target group
`hop clone <url>` SHALL replace `config.Resolve()` with
`config.ResolveWriteTarget()` + `config.EnsureSkeleton(path)` (announcing
`created: <path>` when it created the file). When `EnsureSkeleton` created the
file (`created==true`), clone SHALL call `yamled.EnsureGroup(path, group)` to
create the requested target group before reloading the config and proceeding,
then clone/register as before. `AppendURL`'s `ErrGroupNotFound` contract MUST
remain unchanged: against a PRE-EXISTING config, a typo'd `--group <nonexistent>`
MUST still error via `findGroup==nil` — clone MUST NOT create the group in that
case (Assumption 9).

- **GIVEN** no config on a fresh machine
- **WHEN** the user runs `hop clone <url>` (default group)
- **THEN** the config is created (announced), `repos.default` is created, the URL
  is cloned and registered under `default`
- **AND GIVEN** no config on a fresh machine
- **WHEN** the user runs `hop clone --group vendor <url>`
- **THEN** the config is created, `repos.vendor` is created, and the URL is
  cloned and registered under `vendor`
- **AND GIVEN** a pre-existing config WITHOUT a `nonexistent` group
- **WHEN** the user runs `hop clone --group nonexistent <url>`
- **THEN** clone STILL errors with the "no 'nonexistent' group" message (the
  group is NOT created — typo-catching contract preserved)

### CLI: `hop config init` unchanged

#### R7: `hop config init` and `hop rm` are untouched
`hop config init` SHALL continue to write the embedded annotated starter and
refuse to overwrite (no change). `hop rm` / `hop config rm` MUST NOT auto-init —
it is not in the auto-init command set (only add, scan --write, clone). Its
existing not-found behavior is preserved.

- **GIVEN** a fresh machine
- **WHEN** the user runs `hop config init`
- **THEN** the annotated starter (with the hop.git self-bootstrap seed) is
  written exactly as before
- **AND GIVEN** no config at the fixed path
- **WHEN** the user runs `hop rm` / `hop config rm`
- **THEN** it STILL errors (does not create the config)

### Design Decisions

1. **clone ensure-group fires only on `created==true`**: When clone just
   auto-created the skeleton, there is no pre-existing config to typo against, so
   creating the requested group (default OR `--group <name>`) is correct. On a
   pre-existing file, `EnsureGroup` is NOT called, so a typo'd `--group` still
   hits `findGroup==nil → error`. — *Why*: satisfies both intake goals — fresh
   `clone <url>` works for default and named groups, and the AppendURL
   typo-catching contract is preserved. — *Rejected*: (a) seeding `default:` into
   the skeleton (contradicts minimal `repos: {}` and doesn't handle `--group`),
   (c) extending AppendURL to create groups (changes its contract, too broad).
2. **`created:` colon-style stderr announcement**: The intake says `created
   <path>`; the codebase's existing write-command stderr uses a bare colon style
   (`added: <url>`, `wrote: <path>`, `removed: <url>`). For consistency, emit
   `created: <path>` (colon form, no cmdName prefix) — *Why*: matches neighboring
   `added:`/`wrote:` lines in the same functions.

### Non-Goals

- No new subcommands or flags (Constitution VI). Behavior-widening only.
- No change to `AppendURL` / `MergeScan` / `RemoveURL` contracts.
- No README/spec/memory edits in apply (those happen at hydrate).

## Tasks

### Phase 1: Core Helpers

- [x] T001 Add `config.EnsureSkeleton(path string) (created bool, err error)` + `skeletonContent` constant (`repos: {}\n`) in `src/internal/config/config.go`, next to `WriteStarter` <!-- R1 -->
- [x] T002 Add `yamled.EnsureGroup(path, group string) error` in `src/internal/yamled/yamled.go` (load tree, synthesize `repos` if absent, add `<group>: []` if absent, idempotent, atomic write-back) <!-- R5 -->

### Phase 2: Read-Command Message

- [x] T003 Update `config.Resolve()` not-found message in `src/internal/config/resolve.go:37` to the exact Assumption-11 string <!-- R2 -->

### Phase 3: Write-Command Wiring

- [x] T004 Rewrite the gate in `runAdd` (`src/cmd/hop/config_add.go`): ResolveWriteTarget → EnsureSkeleton (emit `created: <path>`) → Load → proceed; `$HOME`-unset still errors <!-- R3 -->
- [x] T005 Branch `runConfigScan` (`src/cmd/hop/config_scan.go`): when `write==true` use ResolveWriteTarget + EnsureSkeleton (announce) + Load; else keep the existing Resolve()-or-error path unchanged <!-- R4 -->
- [x] T006 Rewrite `cloneURL` (`src/cmd/hop/clone.go`): ResolveWriteTarget + EnsureSkeleton (announce); when `created` call `yamled.EnsureGroup(path, group)` + reload; preserve `findGroup==nil → error` for pre-existing configs <!-- R6 -->

### Phase 4: Tests

- [x] T007 [P] `config.EnsureSkeleton` tests in `src/internal/config/config_test.go`: creates exact `repos: {}\n` at 0644 + returns `created=true` when absent; no-op (`created=false`, unchanged) when present; MkdirAll creates parent dirs <!-- R1 -->
- [x] T008 [P] `yamled.EnsureGroup` tests in `src/internal/yamled/yamled_test.go`: creates `<group>: []` when absent; idempotent when present; preserves existing groups/comments; works on `repos: {}` <!-- R5 -->
- [x] T009 [P] Update `config.Resolve()` not-found test in `src/internal/config/resolve_test.go` to the new Assumption-11 message string <!-- R2 -->
- [x] T010 Flip `hop add` tests in `src/cmd/hop/config_add_test.go`: `TestConfigAddMissingHopYaml` must now assert create-and-proceed + `created:`; add a top-level `hop add` fresh-env create test; add an idempotency test (second run does not re-announce `created:`) <!-- R3 -->
- [x] T011 Update/add `config scan` tests in `src/cmd/hop/config_test.go`: `TestConfigScanMissingHopYaml` (print mode) STILL errors; add a `scan --write` fresh-env auto-init+merge test asserting `created:` <!-- R4 -->
- [x] T012 Add `hop clone <url>` fresh-env tests in `src/cmd/hop/clone_test.go`: default-group create+clone+register (announces `created:`); `--group <name>` fresh-env creates that group; keep `TestCloneURLMissingGroupErrors` (pre-existing config + typo'd group still errors) <!-- R6 -->
- [x] T013 Update read-command not-found expectations: `TestConfigPrintNoConfigErrors` in `src/cmd/hop/config_test.go` and any other read-path test asserting the old message — verify the new two-path hint appears <!-- R2 -->

## Execution Order

- T001, T002, T003 (helpers + message) precede the wiring tasks T004-T006.
- T004 depends on T001; T005 depends on T001; T006 depends on T001 and T002.
- Phase 4 test tasks depend on their respective implementation tasks.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `config.EnsureSkeleton` exists, creates `repos: {}\n` (exact bytes) at mode 0644 when absent (returns `created=true`), is a no-op when present (returns `created=false`), and MkdirAll's the parent dir
- [x] A-002 R2: `config.Resolve()` not-found message matches the exact Assumption-11 string and read-commands still error on absence
- [x] A-003 R3: `hop add <dir>` on a fresh env creates the config (announces `created:`) and registers the repo
- [x] A-004 R4: `hop config scan --write` on a fresh env auto-inits + merges; `hop config scan` (no `--write`) still errors
- [x] A-005 R5: `yamled.EnsureGroup` creates `<group>: []` when absent, is idempotent, and preserves existing content
- [x] A-006 R6: `hop clone <url>` on a fresh env creates config + `default` group + clones/registers; `--group <name>` creates that group
- [x] A-007 R7: `hop config init` and `hop rm` behavior unchanged (init still writes the starter; rm still errors on absence)

### Behavioral Correctness

- [x] A-008 R3: The two-line "Run 'hop config init' first" gate in `runAdd` is removed; the `$HOME`-unset error path is preserved
- [x] A-009 R6: clone's ensure-group runs ONLY when the skeleton was just created (`created==true`); a pre-existing config + typo'd `--group` STILL errors (AppendURL `ErrGroupNotFound` contract unchanged)
- [x] A-010 R3: Auto-init is idempotent — running a write-command a second time does NOT re-announce `created:` (file already exists)

### Edge Cases & Error Handling

- [x] A-011 R1: `EnsureSkeleton` returns non-not-exist stat errors instead of swallowing them
- [x] A-012 R4: Print-mode scan on a fresh env neither creates nor announces — it errors with the not-found message

### Code Quality

- [x] A-013 Pattern consistency: New code follows naming and structural patterns of surrounding code (mirrors `WriteStarter` for EnsureSkeleton; mirrors `mergeScanIntoTree`/`AppendURL` for EnsureGroup; `created:` matches the `added:`/`wrote:` stderr style)
- [x] A-014 No unnecessary duplication: EnsureGroup reuses existing yamled helpers (`mappingValue`, `atomicWrite`) rather than reimplementing tree navigation
- [x] A-015 Readability over cleverness: helpers are small and focused; no god functions
- [x] A-016 No magic strings: skeleton content is a named constant (`skeletonContent`)

### Security

- [x] A-017 R6: No new subprocess exec is introduced; the existing `git clone` call (Constitution I — `exec.CommandContext` via `proc.Run`) is not regressed

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)

### Review (fab-fff, cycle 1 — PASS, no must-fix)

Inward + outward review both returned PASS (0 must-fix). Should-fix triage:

- **Acted on:** `clone --no-add <url>` on a fresh machine had no test pinning its
  (intentional) behavior — `--no-add` suppresses the URL write-back but NOT the
  config file's creation (the skeleton + target group are needed for path
  resolution). Added `TestCloneURLFreshEnvNoAddStillCreatesConfig`
  (`clone_test.go`) to lock this in. No production-code change.
- **Deferred (out of scope):** `hop rm` (`config_rm.go:118`) still uses the old
  two-line "Run 'hop config init' first" gate, now diverging from the refined
  `Resolve()` wording. R7 deliberately excludes `rm` from auto-init; this is
  pre-existing drift surfaced by — not introduced by — this change. Belongs in a
  follow-up, not here.
- **Deferred (regression risk vs. benefit):** `ensureGroupInTree` shares a ~20-line
  synthesize-`repos`-mapping block with `mergeScanIntoTree` (`yamled.go`).
  Extracting a shared helper touches working code for ~6 saved lines and the
  call sites have different `changed`-return needs; not clearly low-effort.
- **Nice-to-have (deferred to hydrate):** memory/spec docs still quote the old
  not-found message and don't mention auto-init — reconciled at hydrate.

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. The old two-line "Run 'hop config init' first, then re-run X" gate messages in `runAdd` (`config_add.go`) and `runConfigScan` (`config_scan.go`) were inline string literals, not shared helpers, and have been fully replaced by the auto-init wiring — they leave nothing dead behind. `config.Resolve()` remains live (read-commands: `resolve.go:42,258`, `config.go:69`, `config_scan.go:81` print mode, `config_rm.go:108`). The identical gate string in `config_rm.go:118` is intentionally retained (R7: `hop rm` is not auto-init). No now-unreachable branches: the old nested `werr`/`bootstrap`-fallback in add/scan was removed in full, not orphaned.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | clone's ensure-group runs only when `created==true` (fresh skeleton); on a pre-existing config a typo'd `--group` still hits `findGroup==nil → error`. EnsureGroup is clone-owned, AppendURL unchanged. | Directly resolves intake Assumption 9's residual ambiguity per the prompt's cleanest reading; satisfies both fresh-clone-works and typo-catching goals | S:95 R:60 A:80 D:75 |
| 2 | Certain | Announcement string is `created: <path>` (colon form, no cmdName prefix) to match existing `added:`/`wrote:`/`removed:` stderr lines | Intake said `created <path>`; codebase convention is the colon form; the apply-prompt directs matching the existing style | S:95 R:90 A:90 D:80 |
| 3 | Confident | `yamled.EnsureGroup` synthesizes the `repos` mapping when absent (mirrors `mergeScanIntoTree`) so it is safe on any skeleton/empty-doc input | Defensive and consistent with the existing merge path; the `repos: {}` skeleton already has the mapping, but synthesizing covers empty docs too | S:80 R:80 A:85 D:75 |

3 assumptions (2 certain, 1 confident, 0 tentative).
