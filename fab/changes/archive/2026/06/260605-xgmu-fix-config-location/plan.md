# Plan: Fix config location, remove env-var overrides

**Change**: 260605-xgmu-fix-config-location
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### Config: Single Fixed Resolution Path

#### R1: `configPath()` builds the single fixed location from `$HOME` only
A shared, unexported `configPath() (string, error)` helper SHALL return `filepath.Join(home, ".config", "hop", "hop.yaml")` where `home = os.Getenv("HOME")`. It SHALL consult NO other environment variable (no `$HOP_CONFIG`, no `$XDG_CONFIG_HOME`). When `$HOME` is empty it SHALL return an error `hop: $HOME is not set; cannot locate config`. The literal `filepath.Join` construction (NOT `os.UserConfigDir()`) guarantees an identical path on darwin and linux.

- **GIVEN** `$HOME=/home/u` and any values of `$HOP_CONFIG`/`$XDG_CONFIG_HOME`
- **WHEN** `configPath()` is called
- **THEN** it returns `/home/u/.config/hop/hop.yaml`, nil
- **AND** GIVEN `$HOME` unset, WHEN called, THEN it returns `"", error` mentioning `$HOME is not set`

#### R2: `Resolve()` stats the fixed path; returns it if present, else a not-found error
`Resolve() (string, error)` SHALL call `configPath()`, `os.Stat` the result, return the path when it exists, and otherwise return `hop: no hop.yaml found at <path>. Run 'hop config init' to create one.` (using `os.IsNotExist`). A stat error other than not-exist SHALL be wrapped as `hop: stat <path>: %w`. All `$HOP_CONFIG` / `$XDG_CONFIG_HOME` branching — including the set-but-missing hard error — SHALL be removed.

- **GIVEN** a file exists at `$HOME/.config/hop/hop.yaml`
- **WHEN** `Resolve()` is called
- **THEN** it returns that path, nil
- **AND** GIVEN no file there, WHEN called, THEN it returns `"", error` equal to `hop: no hop.yaml found at <path>. Run 'hop config init' to create one.`
- **AND** GIVEN `$HOP_CONFIG` points at a nonexistent file, WHEN called, THEN there is NO `$HOP_CONFIG`-specific hard error — only the fixed-path not-found error fires

#### R3: `ResolveWriteTarget()` stays distinct and wraps `configPath()` with no stat
`ResolveWriteTarget() (string, error)` SHALL remain a distinct exported function used by `hop config init` / `hop config where`. It SHALL return `configPath()` directly (no `os.Stat`), so `init`/`where` get the path regardless of file existence. The exported `ErrNoConfig` sentinel SHALL be retained.

- **GIVEN** `$HOME=/home/u` and no file on disk
- **WHEN** `ResolveWriteTarget()` is called
- **THEN** it returns `/home/u/.config/hop/hop.yaml`, nil (no error, no stat)

### Config: User-Facing Message Rewrites

#### R4: `WriteStarter` "already exists" message drops the `$HOP_CONFIG` reference
`config.go` `WriteStarter` SHALL return `hop config init: <path> already exists. Delete it first to recreate it.` on an existing target (no "or set $HOP_CONFIG to a different path").

- **GIVEN** a file already exists at the target
- **WHEN** `WriteStarter(target)` is called
- **THEN** the error is `hop config init: <target> already exists. Delete it first to recreate it.`

#### R5: `hop config init` success tip switches to symlink guidance
`cmd/hop/config.go` init success tip SHALL read: `Tip: to sync this config across machines, keep it in your dotfiles and symlink ~/.config/hop/hop.yaml to it.` (replacing the `$HOP_CONFIG` portability tip).

- **GIVEN** `hop config init` succeeds
- **WHEN** the stderr tip is printed
- **THEN** it is the symlink-based tip and contains no `$HOP_CONFIG` reference

#### R6: `hop add` (and `scan`/`rm`) missing-config message drops the `$XDG_CONFIG_HOME` literal
In the `config.Resolve()` failure branches of `config_add.go`, `config_scan.go`, and `config_rm.go`, the `werr` fallback literal `bootstrap = "$XDG_CONFIG_HOME/hop/hop.yaml"` SHALL be replaced — `ResolveWriteTarget()` no longer consults XDG, so the literal is misleading. The `werr` case now only fires when `$HOME` is unset; the fallback string SHALL be a plain description (`~/.config/hop/hop.yaml`) rather than an env-var literal.

- **GIVEN** `config.Resolve()` returns an error and `ResolveWriteTarget()` succeeds (the normal path)
- **WHEN** `hop add <dir>` / `hop config scan <dir>` / `hop rm` runs pre-bootstrap
- **THEN** the message reads `<cmd>: no hop.yaml found at <resolved fixed path>.` with the init hint — no `$XDG_CONFIG_HOME` literal anywhere

### Help & Docs: Drop env-var references

#### R7: `rootLong` help text reflects the single fixed path
`cmd/hop/root.go` `rootLong` SHALL drop the "Optional: set $HOP_CONFIG …" getting-started bullet and replace the config search-order Notes line with `Config lives at ~/.config/hop/hop.yaml.`

- **GIVEN** `hop --help`
- **WHEN** the output is rendered
- **THEN** it contains `Config lives at ~/.config/hop/hop.yaml.` and no `$HOP_CONFIG` / `$XDG_CONFIG_HOME` reference

#### R8: README reflects the single fixed path with symlink sync guidance
`README.md` line ~11 ("One config, every machine") and line ~101 (First run) SHALL be reworded away from "Dropbox / dotfiles / `$HOP_CONFIG`" and `$XDG_CONFIG_HOME` toward symlinking the fixed `~/.config/hop/hop.yaml`.

- **GIVEN** a reader of README
- **WHEN** they read the config sync guidance
- **THEN** it describes symlinking `~/.config/hop/hop.yaml`, with no `$HOP_CONFIG` / `$XDG_CONFIG_HOME` mention

#### R9: starter.yaml header comment switches to symlink guidance
`src/internal/config/starter.yaml` header comment SHALL replace the "Tip: set $HOP_CONFIG …" line with symlink guidance. This is comment-only — `TestStarterParses` MUST still pass (parses identically).

- **GIVEN** the embedded starter content
- **WHEN** `hop config init` writes it
- **THEN** the header tip references symlinking, parses cleanly, and contains no `$HOP_CONFIG`

### Tests: Migrate fixture-feeding, delete env-behavior tests

#### R10: Test fixtures feed via `$HOME=<tmpdir>` with the file at `<tmpdir>/.config/hop/hop.yaml`
Per Constitution "Test Integrity", tests that fed a fixture via `$HOP_CONFIG=<tmpfile>` SHALL be migrated to set `$HOME=<tmpdir>` and place the fixture at `<tmpdir>/.config/hop/hop.yaml`. The shared `writeReposFixture` helper in `testutil_test.go` SHALL be updated first. Tests asserting now-removed behavior (the `$HOP_CONFIG` set-but-missing hard error, `$HOP_CONFIG` precedence-wins, `$XDG_CONFIG_HOME` precedence) SHALL be deleted. No implementation is weakened to satisfy a fixture.

- **GIVEN** the updated test suite
- **WHEN** `go test ./internal/config/... ./cmd/hop/...` runs
- **THEN** all tests pass with zero `$HOP_CONFIG` / `$XDG_CONFIG_HOME` references remaining in `*.go`

### Non-Goals

- Altering `hop add`'s require-existing-config gate — that is Change B (auto-init); Change A only fixes path + wording.
- Migrating the user's on-disk config or adding any env-var fallback shim (clean break, per intake Assumption 6).
- Updating `docs/memory/` or `docs/specs/config-resolution.md` — those are hydrate-stage edits (noted in intake Affected Memory).

### Design Decisions

1. **Single fixed path via literal `filepath.Join`**: build `$HOME/.config/hop/hop.yaml` directly — *Why*: identical across darwin/linux, environment-independent — *Rejected*: `os.UserConfigDir()` (diverges to `~/Library/Application Support` on macOS); keeping `$XDG_CONFIG_HOME` (can differ between login and CI shells, reintroducing the bug).
2. **`ResolveWriteTarget()` kept distinct**: wraps `configPath()` but stays a separate no-stat function — *Why*: preserves the no-stat vs. stat seam `init`/`where` rely on (intake Assumption 10) — *Rejected*: merging into `Resolve()`.

## Tasks

### Phase 1: Core Resolver

- [x] T001 Collapse `src/internal/config/resolve.go`: add unexported `configPath()`; rewrite `Resolve()` to stat the fixed path with the new not-found message; rewrite `ResolveWriteTarget()` to return `configPath()` with no stat; remove all `$HOP_CONFIG`/`$XDG_CONFIG_HOME` branching and the set-but-missing hard error; keep `ErrNoConfig` exported; update the doc comments to describe single-fixed-path resolution. <!-- R1 R2 R3 -->

### Phase 2: Message Rewrites

- [x] T002 [P] Rewrite the `WriteStarter` "already exists" message in `src/internal/config/config.go` to `hop config init: %s already exists. Delete it first to recreate it.` <!-- R4 -->
- [x] T003 [P] Rewrite the `hop config init` success tip in `src/cmd/hop/config.go` to the symlink guidance. <!-- R5 -->
- [x] T004 [P] Replace the misleading `bootstrap = "$XDG_CONFIG_HOME/hop/hop.yaml"` fallback literal in the `werr` branch of `src/cmd/hop/config_add.go`, `src/cmd/hop/config_scan.go`, and `src/cmd/hop/config_rm.go` with `"~/.config/hop/hop.yaml"`. <!-- R6 -->

### Phase 3: Help & Docs

- [x] T005 [P] Update `src/cmd/hop/root.go` `rootLong`: drop the `$HOP_CONFIG` getting-started bullet (renumber the remaining steps), and replace the config search-order Notes line with `Config lives at ~/.config/hop/hop.yaml.` <!-- R7 -->
- [x] T006 [P] Reword `README.md` line ~11 ("One config, every machine") and the First-run paragraph (~line 101) toward symlinking `~/.config/hop/hop.yaml`; remove `$HOP_CONFIG` / `$XDG_CONFIG_HOME` / Dropbox references. <!-- R8 -->
- [x] T007 [P] Update the header comment in `src/internal/config/starter.yaml` to symlink guidance (comment-only). <!-- R9 -->

### Phase 4: Tests

- [x] T008 Update the shared helper `writeReposFixture` in `src/cmd/hop/testutil_test.go` to feed fixtures via `$HOME=<tmpdir>` + `<tmpdir>/.config/hop/hop.yaml` instead of `$HOP_CONFIG`. <!-- R10 -->
- [x] T009 Migrate `src/internal/config/resolve_test.go`: delete `$HOP_CONFIG`/`$XDG_CONFIG_HOME` behavior tests (set, set-but-missing, XDG, write-target-HOP_CONFIG, write-target-XDG); rewrite remaining tests against the fixed path; update the not-found message assertion. <!-- R10 R2 R3 -->
- [x] T010 Migrate `src/internal/config/config_test.go`: update the `TestWriteStarterRefusesOverwrite` assertion to match the new message (drop the `$HOP_CONFIG`-in-error check). <!-- R10 R4 -->
- [x] T011 Migrate `src/cmd/hop/config_test.go`: convert `$HOP_CONFIG`-fed fixtures to `$HOME`-fed; update the init-tip assertion (R5); replace `TestConfigPrintMissingFileErrors` (asserted `$HOP_CONFIG points to` hard error — now removed) with a no-config not-found assertion; fix `TestConfigPathSubcommandRemoved`, where/print/scan/add fixtures. <!-- R10 R2 R5 -->
- [x] T012 Migrate `src/cmd/hop/config_add_test.go`: convert `$HOP_CONFIG`-fed fixtures to `$HOME`-fed (the simple ones at the top + missing-config test). <!-- R10 -->
- [x] T013 Migrate `src/cmd/hop/integration_test.go`: convert the `HOP_CONFIG=`-in-`cmd.Env` subprocess fixtures to `HOME=<tmpdir>` with the fixture at `<tmpdir>/.config/hop/hop.yaml`. <!-- R10 -->
- [x] T014 Migrate `src/cmd/hop/repo_completion_test.go`: the `TestCompletionMissingConfigIsSilent` test sets `$HOP_CONFIG` to a nonexistent file — switch to an isolated `$HOME` with no config so the missing-config path fires. <!-- R10 R2 -->

### Phase 5: Verify

- [x] T015 Run `go build ./...`, `go vet ./...`, `gofmt -l`, and the scoped test packages, then the full suite; confirm zero `$HOP_CONFIG`/`$XDG_CONFIG_HOME` references remain in `*.go`. <!-- R10 -->

## Execution Order

- T001 blocks the Phase 4 test migrations (tests assert the new resolver behavior).
- T008 (helper) blocks T011–T014 (call sites route through `writeReposFixture` or mirror its pattern).
- Phase 2 / Phase 3 tasks ([P]) are independent of each other and of T001.
- T015 runs last.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `configPath()` exists, returns `filepath.Join($HOME, ".config", "hop", "hop.yaml")`, consults only `$HOME`, and errors when `$HOME` is empty.
- [x] A-002 R2: `Resolve()` returns the fixed path when present and the exact not-found message otherwise; no env-var branching remains.
- [x] A-003 R3: `ResolveWriteTarget()` is still a distinct exported function wrapping `configPath()` with no stat; `ErrNoConfig` still exported.
- [x] A-004 R4: `WriteStarter` "already exists" message is `hop config init: <path> already exists. Delete it first to recreate it.`
- [x] A-005 R5: `hop config init` success tip is the symlink guidance.
- [x] A-006 R6: `hop add`/`scan`/`rm` missing-config messages contain no `$XDG_CONFIG_HOME` literal.
- [x] A-007 R7: `hop --help` shows `Config lives at ~/.config/hop/hop.yaml.` and no env-var references.
- [x] A-008 R8: README config-sync guidance is symlink-based with no `$HOP_CONFIG`/`$XDG_CONFIG_HOME`/Dropbox.
- [x] A-009 R9: starter.yaml header comment is symlink guidance and `TestStarterParses` passes.

### Behavioral Correctness

- [x] A-010 R2: The `$HOP_CONFIG` set-but-missing hard error no longer fires — setting `$HOP_CONFIG` to a bad path produces only the fixed-path not-found error (or success if the fixed path exists).
- [x] A-011 R2: `$XDG_CONFIG_HOME`, even when set, does not move the resolved path.

### Removal Verification

- [x] A-012 R10: No production behavior depends on `$HOP_CONFIG` / `$XDG_CONFIG_HOME`; deleted behavior tests are gone (not commented out). The only remaining textual references are intentional: the `resolve.go` doc comment documenting what is *not* consulted, and the `TestResolveIgnoresLegacyEnvVars` regression guard that sets the removed vars to prove they are ignored.

### Scenario Coverage

- [x] A-013 R10: `go test ./internal/config/...` and `go test ./cmd/hop/...` pass; migrated fixtures feed via `$HOME`.

### Edge Cases & Error Handling

- [x] A-014 R1: `$HOME` unset yields the `$HOME is not set` error from `configPath()` (propagated by `Resolve`/`ResolveWriteTarget`).

### Code Quality

- [x] A-015 Pattern consistency: New code follows the `hop:`-prefixed `fmt.Errorf` error style and surrounding naming/structure conventions of `resolve.go`.
- [x] A-016 No unnecessary duplication: `Resolve()` and `ResolveWriteTarget()` share the single `configPath()` helper rather than re-deriving the path.
- [x] A-017 No magic strings: the path components are expressed via `filepath.Join(home, ".config", "hop", "hop.yaml")`, consistent with the codebase (the user-facing `~/.config/hop/hop.yaml` literal in messages mirrors existing message style).
- [x] A-018 Builds clean: `go build ./...`, `go vet ./...`, and `gofmt -l` report no issues.

## Notes

- Check items as you review: `- [x]`
- Memory (`config/search-order`, `config/init-bootstrap`, `config/index`, `cli/subcommands`, `architecture/package-layout`) and `docs/specs/config-resolution.md` are updated at hydrate, not apply.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Extend the `werr` XDG-literal fix (R6) to `config_scan.go` and `config_rm.go`, not just `config_add.go` (the only file the intake §4 named) | The identical misleading `bootstrap = "$XDG_CONFIG_HOME/hop/hop.yaml"` literal appears verbatim in all three files; the intake's stated goal is to drop the XDG literal everywhere it appears (Affected Memory + Assumption 7 "everywhere it appears"). Fixing only one would leave two stale references and fail R10's zero-references acceptance. Low blast radius, one obvious interpretation. | S:80 R:90 A:90 D:90 |
| 2 | Certain | Use `"~/.config/hop/hop.yaml"` (plain tilde description) as the `werr` fallback string in the three command files | The `werr` branch only fires when `$HOME` is unset; a plain human-readable path is the least-misleading filler and matches the intake's "plain description or be dropped" guidance. The normal path uses the real resolved fixed path from `ResolveWriteTarget()`. | S:85 R:90 A:85 D:80 |

2 assumptions (2 certain, 0 confident, 0 tentative).
