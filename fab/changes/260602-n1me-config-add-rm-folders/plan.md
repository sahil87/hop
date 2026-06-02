# Plan: Config Add / Remove Folders

**Change**: 260602-n1me-config-add-rm-folders
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### yamled: RemoveURL primitive

#### R1: Remove a URL from a named group, comment-preserving and atomic
`yamled.RemoveURL(path, group, url string) error` SHALL load the YAML file at
`path` as a `yaml.Node` tree, locate `repos.<group>`, drop the scalar node whose
value equals `url` from the group's URL sequence (handling both the flat-list
shape and the `urls:`-map shape), and write the result back atomically via the
existing `atomicWrite`. Comments in unmodified portions SHALL be preserved
(yaml.v3 round-trip).

- **GIVEN** a `hop.yaml` with a flat-list group `default` containing two URLs and an inline comment
- **WHEN** `RemoveURL(path, "default", <first-url>)` is called
- **THEN** the file is rewritten with only the second URL, the comment on the surviving line is preserved, and the function returns `nil`
- **AND** the same behavior holds for a `urls:`-map-shaped group, preserving the `dir:` field

#### R2: Removing the last URL leaves an empty-group placeholder
RemoveURL SHALL NOT delete the group node when its last URL is removed; the
now-empty group node SHALL remain (`default: []` for flat groups, or
`mygroup: {dir: ..., urls: []}` for map-shaped groups).

- **GIVEN** a group containing exactly one URL
- **WHEN** that URL is removed
- **THEN** the group key remains present with an empty sequence and the function returns `nil`

#### R3: Forgiving not-found semantics
When the `url` is not present in the group, OR the `group` is absent, RemoveURL
SHALL be a no-op that returns a detectable sentinel error the caller can match
(`ErrURLNotFound` / `ErrGroupNotFound`) and surface as a message + exit 0. The
file SHALL NOT be modified in either not-found case.

- **GIVEN** a `hop.yaml` that does not contain the target URL (or group)
- **WHEN** RemoveURL is called
- **THEN** the file is byte-for-byte unchanged and the returned error satisfies `errors.Is(err, ErrURLNotFound)` (or `ErrGroupNotFound`)

### CLI: hop config add <dir>

#### R4: Validate the directory argument
`hop config add <dir>` SHALL validate `<dir>` via `filepath.Clean` →
`filepath.EvalSymlinks` → `os.Stat` is-dir, reusing scan's validation idiom. On
failure it SHALL print `hop config add: '<dir>' is not a directory.` to stderr
and exit 2.

- **GIVEN** a `<dir>` that does not exist or is a regular file
- **WHEN** `hop config add <dir>` runs
- **THEN** stderr carries the not-a-directory message and the exit code is 2

#### R5: Require hop.yaml precondition
`hop config add` SHALL require an existing `hop.yaml` (mirroring scan's
precondition + message style, pointing at `hop config init`). Missing config →
exit 1.

- **GIVEN** no resolvable `hop.yaml`
- **WHEN** `hop config add <dir>` runs
- **THEN** stderr says no hop.yaml found and to run `hop config init`, and the command exits 1 (errSilent)

#### R6: Classify the single dir and register a normal repo (writes by default)
`hop config add` SHALL classify the single canonical dir via the existing
`internal/scan` classification (first-match-wins: worktree → skip, normal repo
(`.git` dir + remote) → register, bare repo → skip). For a normal repo it SHALL
resolve the remote URL via `git remote get-url` (through `internal/proc` only,
5s timeout, git lazy-required), build a one-element `yamled.ScanPlan` via the
existing `buildScanPlan` (convention → `default`; else invented group from the
slugified parent dir), and apply it via `yamled.MergeScan` (atomic,
comment-preserving). It SHALL write by default (no print-default like scan).

- **GIVEN** a directory that is a normal git repo whose convention path matches `<code_root>/<org>/<name>`
- **WHEN** `hop config add <dir>` runs
- **THEN** the repo's URL is merged into the `default` group of `hop.yaml` and a confirming message is printed
- **AND** a non-convention repo lands in an invented group keyed off the slugified parent dir basename

#### R7: Forgiving non-git-dir handling
When the single dir is a plain directory (no `.git`, not bare), `hop config add`
SHALL print a clear "not a git repo" message and exit 0 (forgiving — not an
error). Worktree and bare-repo classifications SHALL likewise print a skip
message and exit 0.

- **GIVEN** a plain directory with no `.git`
- **WHEN** `hop config add <dir>` runs
- **THEN** stderr carries a "not a git repo" message, hop.yaml is unchanged, and the exit code is 0

#### R8: Idempotent re-add
When the resolved URL is already registered anywhere in `hop.yaml`, `hop config
add` SHALL dedup (no write), print an "already registered" message, and exit 0.

- **GIVEN** a repo whose URL is already present in hop.yaml
- **WHEN** `hop config add <dir>` runs
- **THEN** no write occurs, an "already registered" message is printed, and the exit code is 0

### CLI: hop config rm [--stale]

#### R9: Interactive single-select removal via fzf
`hop config rm` (no arg) SHALL load repos via `repos.FromConfig`, build picker
lines via the existing `buildPickerLines`, pipe them to `fzf.Pick` (single-select
only), map the selected line back to its source `Repo` by the path column (the
approach `resolve.go` uses), and remove that repo's URL from its group in
`hop.yaml` via `yamled.RemoveURL`.

- **GIVEN** a hop.yaml with registered repos and a user fzf selection
- **WHEN** `hop config rm` runs and the user picks a line
- **THEN** the selected repo's URL is removed from its group, a confirming message is printed, and the command exits 0

#### R10: --stale pre-filters to repos missing from disk
`hop config rm --stale` SHALL pre-filter candidates to repos whose resolved
`Path` does not exist on disk (`os.Stat`) before opening the picker. When zero
candidates are stale it SHALL print a friendly "nothing stale" message and exit
0 without invoking fzf.

- **GIVEN** some registered repos exist on disk and some do not
- **WHEN** `hop config rm --stale` runs
- **THEN** only the missing-from-disk repos are offered in the picker
- **AND** when no repo is stale, no picker opens, a "nothing stale" message is printed, and the exit code is 0

#### R11: fzf-missing and cancel handling
`hop config rm` SHALL reuse the existing `fzfMissingHint` + `errFzfMissing`
handling. fzf user cancellation (Esc/Ctrl-C, exit 130) SHALL be a no-op exiting
0 (mapped via the existing `errFzfCancelled` sentinel through `translateExit`).

- **GIVEN** fzf is not on PATH
- **WHEN** `hop config rm` runs
- **THEN** the `fzfMissingHint` is printed and the command exits 1 (errSilent)
- **AND** an Esc/Ctrl-C cancellation is a no-op with exit code 130

### CLI: wiring

#### R12: Register add/rm under config
`newConfigCmd` SHALL add `newConfigAddCmd()` and `newConfigRmCmd()` alongside the
existing four subcommands, and the `config` Short SHALL mention add and rm.

- **GIVEN** `hop config --help`
- **WHEN** it is rendered
- **THEN** both `add` and `rm` subcommands appear in the listing

### Non-Goals

- A `--dry-run` / print-mode flag on `config add` — explicitly deferred (Assumption 7).
- Multi-select removal (`fzf -m` → `[]string`) — single-select v1; `fzf.Pick` unchanged (Assumption 9).
- A non-interactive `--stale --yes` headless prune — deferred (Assumption 12).
- A URL-accepting `config add <url>` mode — covered by `clone`'s auto-registration (Assumption 3).
- Worktree-aware staleness — `--stale` checks the repo's own resolved Path only (Assumption 8).

### Design Decisions

1. **Reuse scan's classification for a single dir**: `config add` calls a new
   thin `scan.ClassifyOne(path)` entry point that wraps the unexported
   `classifyDir` + `inspectRepo` logic — *Why*: `classifyDir` is unexported in
   `package scan`; adding the smallest exported single-dir seam avoids
   reimplementing classification in `package main` and keeps Constitution I/IV
   (git via proc, wrap-don't-reinvent). *Rejected*: exporting `classifyDir`
   raw (leaks the `dirClass` enum and skips the remote-inspection step the CLI
   needs) or duplicating classification in config_add.go.
2. **fzf seam for testability**: introduce a package-level `var pickOne = fzf.Pick`
   in config_rm.go so tests can inject a fake selection — *Why*: mirrors the
   `listWorktrees` / `runInteractive` seam idiom already in the codebase, and
   `resolve.go` calls `fzf.Pick` directly with no cmd-level seam, making its fzf
   path unit-untestable. *Rejected*: leaving `config rm` fully untestable like
   `resolve.go`'s picker path.

## Tasks

### Phase 1: Core primitive

- [x] T001 Add `yamled.RemoveURL(path, group, url string) error` plus `ErrURLNotFound` sentinel to `src/internal/yamled/yamled.go`, mirroring `AppendURL`'s tree round-trip + `atomicWrite`; handle flat-list and `urls:`-map shapes; leave empty-group placeholder on last removal; return sentinels on not-found without writing. <!-- R1 R2 R3 -->
- [x] T002 [P] Add RemoveURL tests to `src/internal/yamled/yamled_test.go`: flat-list removal, `urls:`-map removal, last-URL-leaves-empty-placeholder, comment preservation, URL-not-found sentinel + file unchanged, group-not-found sentinel + file unchanged. <!-- R1 R2 R3 -->

### Phase 2: Single-dir classification seam

- [x] T003 Add an exported single-dir entry point `scan.ClassifyOne(ctx, canonicalPath, opts) (Found, skipReason string, isRepo bool, err error)` (or equivalent minimal shape) to `src/internal/scan/scan.go` that reuses `classifyDir` + `inspectRepo` without the walk scaffolding. <!-- R6 R7 -->
- [x] T004 [P] Add `ClassifyOne` tests to `src/internal/scan/scan_test.go`: worktree → skip, normal repo → Found with URL (fake runner), bare repo → skip, plain dir → not-a-repo, no-remote → skip. <!-- R6 R7 -->

### Phase 3: config add

- [x] T005 Create `src/cmd/hop/config_add.go`: `newConfigAddCmd()` + `runConfigAdd(cmd, userArg)` — validate dir (reuse shared `validateConfigDir`), require hop.yaml (reuse scan's precondition message), `ClassifyOne`, dispatch by classification (skip/not-git → message + exit 0), build one-element plan via `buildScanPlan`, dedup → "already registered" exit 0, else `MergeScan` write-by-default + confirming message. <!-- R4 R5 R6 R7 R8 -->
- [x] T011 (rework) Consolidate dir validation: extract `validateConfigDir(userArg, cmdName, stderr)` in config_scan.go; both `config scan` and `config add` call it (removes the verbatim `validateAddDir`/`validateScanDir` duplication — A-018). Align `config rm`'s missing-config / load-error preconditions with scan/add's prefixed `errSilent` voice. <!-- R4 R5 R9 -->
- [x] T006 [P] Add config add tests to `src/cmd/hop/config_test.go` (or a new config_add_test.go): dir-validation failure (exit 2), missing hop.yaml (exit 1), non-git dir (forgiving exit 0), normal-repo convention → default, non-convention → invented group, already-registered idempotency. Use the existing `withFakeGitRunner` + `makeRepoDir` seams. <!-- R4 R5 R6 R7 R8 -->

### Phase 4: config rm

- [x] T007 Create `src/cmd/hop/config_rm.go`: `newConfigRmCmd()` with `--stale` flag + `runConfigRm(cmd, stale)` — load repos, optional stale filter (`os.Stat` Path missing), zero-stale friendly message exit 0, `buildPickerLines`, `var pickOne = fzf.Pick` seam, map selected line back to Repo by path column, `RemoveURL`, RemoveURL not-found forgiving message, fzf-missing → fzfMissingHint+errSilent, cancel → errFzfCancelled. <!-- R9 R10 R11 -->
- [x] T008 [P] Add config rm tests to `src/cmd/hop/config_rm_test.go`: stale filter (helper `staleRepos`), line→repo map-back, RemoveURL integration via injected `pickOne`, zero-stale message, fzf-missing handling, fzf-cancel no-op. <!-- R9 R10 R11 -->

### Phase 5: Wiring

- [x] T009 Wire `newConfigAddCmd()` and `newConfigRmCmd()` into `newConfigCmd` in `src/cmd/hop/config.go` and update the `config` Short to mention add/rm. <!-- R12 -->
- [x] T010 [P] Extend `TestConfigSubcommandsListedUnderConfigHelp` in `src/cmd/hop/config_test.go` to assert add and rm appear in `config --help`. <!-- R12 -->

## Execution Order

- T001 blocks T005, T007 (RemoveURL primitive used by config rm; yamled compiled first).
- T003 blocks T005 (ClassifyOne used by config add).
- T005, T007 block T009 (factories must exist before wiring).
- `[P]` test tasks run alongside their sibling implementation task.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `yamled.RemoveURL` removes the matching URL from both flat-list and `urls:`-map groups and writes atomically. (yamled.go:120-181 via atomicWrite; TestRemoveURLFlatList/MapGroup)
- [x] A-002 R2: Removing a group's last URL leaves an empty-group placeholder (group key retained, empty sequence). (yamled.go:179 drops only the scalar; TestRemoveURLLastLeavesEmpty{Flat,Map}Group, both re-parse via config.Load)
- [x] A-003 R3: URL-not-found and group-not-found return matchable sentinel errors and leave the file unchanged. (yamled.go:169/174 wrap ErrURLNotFound, :148/157/162 wrap ErrGroupNotFound, no write on the not-found branch; TestRemoveURLNotFound/GroupNotFoundIsForgiving assert file == original)
- [x] A-004 R4: `hop config add` rejects a non-directory arg with `hop config add: '<dir>' is not a directory.` and exit 2. (config_add.go:61-64 + shared validateConfigDir in config_scan.go; TestConfigAddDirNotADirectory)
- [x] A-005 R5: `hop config add` with no hop.yaml prints the init-pointer message and exits 1. (config_add.go:67-75; TestConfigAddMissingHopYaml)
- [x] A-006 R6: `hop config add` registers a normal repo's URL by default (convention → default, non-convention → invented group) via MergeScan. (config_add.go:102-117; TestConfigAddConventionRepoLandsInDefault / NonConventionInventsGroup)
- [x] A-007 R7: `hop config add` on a plain (non-git) dir prints a "not a git repo" message and exits 0 without writing. (config_add.go:94-99 + addSkipMessage:142-147; TestConfigAddNonGitDirIsForgiving asserts file unchanged)
- [x] A-008 R8: `hop config add` on an already-registered URL is a no-op with an "already registered" message, exit 0. (config_add.go:106-109 via planIsEmpty; TestConfigAddAlreadyRegisteredIsIdempotent asserts no write)
- [x] A-009 R9: `hop config rm` removes the fzf-selected repo's URL from its group via RemoveURL. (config_rm.go:101; TestConfigRmRemovesSelectedURL)
- [x] A-010 R10: `hop config rm --stale` offers only missing-from-disk repos; zero-stale prints a friendly message and exits 0 with no picker. (config_rm.go:79-89 + staleRepos:118-126; TestConfigRmStaleOnlyOffersMissingRepos / StaleNothingStale asserts picker not invoked)
- [x] A-011 R11: `hop config rm` reuses fzfMissingHint/errFzfMissing and treats cancel (130) as a no-op. (config_rm.go:93-98 + pickRepo:135-143; main.go:137 maps errFzfCancelled→130; TestConfigRmFzfMissing / FzfCancelIsNoOp)
- [x] A-012 R12: `add` and `rm` are registered under `config` and listed in `config --help`. (config.go:17 + Short:15; TestConfigSubcommandsListedUnderConfigHelp asserts both Shorts)

### Behavioral Correctness

- [x] A-013 R6: All git invocations in config add route through `internal/proc` with a 5s timeout (Constitution I); no direct `os/exec`. (config_add.go:85 → scan.ClassifyOne → inspectRepo → runGit wraps a 5s context.WithTimeout (scan.go:369-371) and the gitRunner is proc.RunCapture-bound (config_scan.go:122); no os/exec in the diff)
- [x] A-014 R9: config rm maps the selected line back to a Repo by the unique path column (not by name), matching resolve.go. (config_rm.go:148-157 splits on tab and matches parts[1] == rs[i].Path; TestConfigRmMapBackByPathOnNameCollision exercises a name collision)

### Edge Cases & Error Handling

- [x] A-015 R7: Worktree and bare-repo classifications in config add are forgiving skips (message + exit 0), not errors. (config_add.go:94-99 treats every !isRepo case uniformly via addSkipMessage and returns nil; ClassifyOne worktree/bare paths unit-tested in scan_test.go. Note: no CLI-level worktree/bare add test — only the plain-dir CLI path is exercised end-to-end — but the code path is uniform and the classification is covered at the scan layer.)
- [x] A-016 R3: config rm surfaces RemoveURL's not-found sentinel as a message + exit 0 (forgiving). (config_rm.go:103-106 errors.Is on ErrURLNotFound/ErrGroupNotFound → message + return nil. Note: no dedicated CLI test for this branch; sentinel behavior is unit-tested at the yamled layer and the CLI branch is straightforward.)

### Code Quality

- [x] A-017 Pattern consistency: New code follows the naming, error-message voice (`hop config add: ...`), and structural patterns of config_scan.go / clone.go / resolve.go. (addCmdName/rmCmdName prefixes, `added:`/`wrote:`/`removed:` lines, pickOne seam mirrors resolve.go's fzf path; pickRepo mirrors resolveByName's tab-split map-back almost verbatim)
- [x] A-018 No unnecessary duplication: `classifyDir`, `buildScanPlan`, `buildPickerLines`, `slugifyGroupName`, `atomicWrite`, and validation idioms are reused, not reimplemented. (Rework T011: `validateAddDir` removed; both `config scan` and `config add` now call the shared `validateConfigDir(userArg, cmdName, stderr)` in config_scan.go. All named utilities reused.)
- [x] A-019 No god functions (>50 lines) without clear reason in config_add.go / config_rm.go. (runConfigAdd and runConfigRm are each ~45 non-comment lines — linear pipelines matching runConfigScan's structure)
- [x] A-020 Named constants used for the CLI message prefixes (`addCmdName`, `rmCmdName`) rather than repeated string literals. (config_add.go:21, config_rm.go:20)

### Security

- [x] A-021 R6: The user-provided `<dir>` is validated (Clean → EvalSymlinks → Stat) before any subprocess; the canonical path is what is passed to git (Constitution I). (shared validateConfigDir runs Clean→EvalSymlinks→Stat and returns the resolved path; config_add.go:61 captures canonicalDir and :85 passes it to ClassifyOne, so git only ever sees the canonical path)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant. (`validateAddDir` duplicates `validateScanDir`, but that is a duplication to consolidate — see the A-018 finding — not existing code rendered redundant by the new code.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Add `scan.ClassifyOne` exported single-dir entry point rather than exporting `classifyDir` raw | `classifyDir` is unexported and returns a private `dirClass` enum; the CLI also needs the remote-inspection step that `inspectRepo` performs. The smallest reusable seam is a single-dir classify-and-inspect that mirrors Walk's per-dir branch. Intake Assumption 6 anticipated lifting a seam. | S:80 R:75 A:80 D:75 |
| 2 | Confident | Introduce `var pickOne = fzf.Pick` seam in config_rm.go for test injection | Mirrors the established `listWorktrees` / `runInteractive` seam idiom; `resolve.go` calls `fzf.Pick` directly and its picker path is untestable. A seam lets config rm's map-back + RemoveURL integration be exercised without a real fzf. | S:75 R:80 A:80 D:70 |
| 3 | Certain | RemoveURL not-found uses typed sentinels (`ErrURLNotFound`, reuse `ErrGroupNotFound`) the caller matches via errors.Is | Intake Assumption 10 says forgiving (message + exit 0); typed sentinels match `AppendURL`'s existing `ErrGroupNotFound` idiom and let the CLI distinguish no-op from real errors. | S:90 R:85 A:90 D:85 |
| 4 | Certain | config rm removes the URL from the repo's own group (RemoveURL takes the Repo.Group), not a global search | Each URL belongs to exactly one group (config.validateUniqueURLs), and the selected Repo carries its Group; passing it is exact and avoids ambiguity. | S:90 R:85 A:90 D:90 |
| 5 | Certain | config add confirming-message + skip-message wording follows scan/clone voice (`hop config add: ...`, `skip:` / `wrote:` style) | Intake requires mirroring scan's message style; matching the existing voice keeps UX consistent. | S:90 R:90 A:85 D:85 |

5 assumptions (3 certain, 2 confident, 0 tentative).
