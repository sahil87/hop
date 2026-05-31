# Plan: Add --skip-brew-update flag to update command

**Change**: 260531-dx50-skip-brew-update-flag
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

<!-- Sequential work items for the apply stage. Checked off [x] as completed. -->

### Phase 1: Core Implementation

<!-- Primary functionality. Order by dependency — earlier tasks are prerequisites for later ones. -->

- [x] T001 Add `skipBrewUpdate bool` parameter to `update.Run` in `src/internal/update/update.go`. New signature: `func Run(currentVersion string, skipBrewUpdate bool, out, errOut io.Writer) error`. Wrap ONLY the existing `brew update --quiet` block (~L63–L72) in `if !skipBrewUpdate { ... }`; leave the isBrewInstalled short-circuit, wrapper lines, brewLatestVersion call, up-to-date short-circuit, and `brew upgrade` foreground call byte-for-byte identical. Update the `Run` doc comment to mention `skipBrewUpdate`. <!-- A-001, A-003, A-004, A-005 -->
- [x] T002 Introduce minimal unexported package-level seam in `src/internal/update/update.go` so brew invocations are observable in tests without refactoring `internal/proc`: `var brewRun = func(ctx, name, args...) ([]byte, error) { return proc.Run(...) }`, `var brewRunForeground = func(ctx, dir, name, args...) (int, error) { return proc.RunForeground(...) }`, and `var brewInstalledCheck = isBrewInstalled`. Route `Run`/`brewLatestVersion` through these vars. Defaults preserve production behavior (Constitution I — explicit arg slices via internal/proc). <!-- A-006 -->

### Phase 2: Integration

<!-- Wire components together. -->

- [x] T003 Wire a real cobra bool flag `--skip-brew-update` (default false) in `src/cmd/hop/update.go` via `cmd.Flags().BoolVar(...)`, matching the clone.go/sync.go convention (local `var skipBrewUpdate bool` inside `newUpdateCmd`, assign `cmd`, register flag, return cmd). Retain `cobra.NoArgs`. Pass the flag into `update.Run(version, skipBrewUpdate, cmd.OutOrStdout(), cmd.ErrOrStderr())`. Preserve the `errors.Is(err, proc.ErrNotFound) -> errSilent` mapping. Help text explains it skips the internal `brew update` tap-metadata refresh while the version check and upgrade still run. <!-- A-002 -->

### Phase 3: Testing

<!-- Subprocess-invocation assertions and signature migration. -->

- [x] T004 In `src/internal/update/update_test.go`, add a test that overrides `brewInstalledCheck` (→ true), `brewRun`, and `brewRunForeground` with a recorder appending normalized command strings ("brew update", "brew info ...", "brew upgrade ..."), with the `brew info --json=v2` stub returning valid JSON reporting a NEWER stable version than the currentVersion. Restore overrides via `t.Cleanup`/defer. Assert: `Run("v0.0.1", true, ...)` records an entry starting "brew upgrade" and NO entry starting "brew update"; `Run("v0.0.1", false, ...)` records both "brew update" and "brew upgrade". <!-- A-007, A-008 -->
- [x] T005 Update `TestRunNonBrewInstall` in `src/internal/update/update_test.go` to the new signature: `Run("v0.0.3", false, &stdout, &stderr)`. Leave its `isBrewInstalled()` skip guard behavior intact so it still passes regardless of environment. <!-- A-009 -->

### Phase 4: Verification

- [x] T006 From `src/`: run `go build ./...`, `go test ./internal/update/...`, and `go vet ./internal/update/... ./cmd/...`. All MUST pass. <!-- A-010 -->

## Acceptance

### Functional Completeness

- [ ] A-001 Skip semantics: When `skipBrewUpdate == true`, `update.Run` skips ONLY `brew update --quiet`; `brew info --json=v2`, the `Already up to date` short-circuit, and `brew upgrade <formula>` all still execute. `isBrewInstalled` short-circuit and wrapper messages unaffected.
- [ ] A-002 Flag declaration: `hop update` exposes a boolean `--skip-brew-update` flag via cobra `BoolVar`, default `false`, with `cobra.NoArgs` retained and descriptive help text stating it skips the internal `brew update` tap-metadata refresh while version check and upgrade still run.
- [ ] A-003 Default behavior preserved: With `skipBrewUpdate == false`, `brew update --quiet` runs first (gated by `if !skipBrewUpdate`), then version check, short-circuit, upgrade — no output, ordering, error-handling, or exit-code change in the default path.
- [ ] A-004 Signature threading: `update.Run` signature is `Run(currentVersion string, skipBrewUpdate bool, out, errOut io.Writer) error`; the single call site in `cmd/hop/update.go` passes the flag value; the `proc.ErrNotFound` → `errSilent` mapping is preserved in `RunE`.
- [ ] A-005 Output routing preserved: The `Run` doc comment's documented split (wrapper messages → `out`/`errOut`; subprocess streams → `internal/proc`) is preserved; the guard introduces no new writes to `out`/`errOut` and does not alter `brew info`/`brew upgrade` stream routing.

### Behavioral Correctness

- [ ] A-006 Test seam is minimal and unexported, defaults to real `proc.Run`/`proc.RunForeground`/`isBrewInstalled`; `internal/proc` is NOT refactored; production behavior is byte-identical (Constitution I explicit-arg-slice convention preserved).

### Scenario Coverage

- [ ] A-007 Skip path test: a test asserts `Run("v0.0.1", true, ...)` records "brew upgrade" and NOT "brew update", against a recorder reporting a newer available version, with no brew/network/Cellar dependency.
- [ ] A-008 Default path regression test: a test asserts `Run("v0.0.1", false, ...)` records both "brew update" and "brew upgrade".
- [ ] A-009 Existing non-brew test: `TestRunNonBrewInstall` calls `Run("v0.0.3", false, &stdout, &stderr)`, retains its isBrewInstalled skip guard, and still passes.

### Edge Cases & Error Handling

- [ ] A-010 Verification gate: `go build ./...`, `go test ./internal/update/...`, and `go vet ./internal/update/... ./cmd/...` (from `src/`) all pass with no failures.

### Code Quality

- [ ] A-011 Pattern consistency: New code follows naming and structural patterns of surrounding code (clone.go/sync.go flag wiring, update.go comment/error-handling style).
- [ ] A-012 No unnecessary duplication: Existing utilities reused where applicable; no magic strings/numbers without named constants; no god functions introduced.

### Security

- [ ] A-013 Constitution I (Security First): subprocess args remain explicit slices routed through `internal/proc`; the seam defaults point at `proc.Run`/`proc.RunForeground`; no shell strings introduced.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
