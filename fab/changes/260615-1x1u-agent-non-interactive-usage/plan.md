# Plan: Make hop usable by non-interactive agents (+ shim hardening)

**Change**: 260615-1x1u-agent-non-interactive-usage
**Type**: feat
**Intake**: [intake.md](./intake.md) (all 11 design decisions Certain; #12 deferred to hydrate)

## Requirements

Derived from the intake's four scope items and 11 Certain assumptions. Each requirement is GIVEN/WHEN/THEN where a scenario applies.

### R1 — Self-contained shim (the bug fix; intake Item 1, assumption #1)

- R1.1 `posixInit` (`src/cmd/hop/shell_init.go`) MUST inline the body of `_hop_passthrough` DIRECTLY into the `PASSTHROUGH)` arm of `hop()`, preserving the `WT_CD_FILE` cd side-channel VERBATIM (mktemp fallback to `command hop "$@"`, `-s` non-empty test, `rc != 0` early return, trailing `rm -f`, final conditional `cd -- "$target"`).
- R1.2 The standalone `_hop_passthrough()` top-level function definition MUST be removed — after the change `posixInit` emits a single capturable `hop()` unit (plus the `h()` alias), with no sibling top-level function for a snapshotter to drop.
- R1.3 The `__complete*` arm, `CD`/`RUN_IN_PARENT` arms, defensive `*)` fallback, `h()` alias, and per-shell cobra completion suffix MUST stay unchanged.
- R1.4 GIVEN a snapshotting shell that captures `hop()` WHEN any PASSTHROUGH command runs THEN it MUST NOT fail with `_hop_passthrough: command not found` (the bug being fixed).

### R2 — `hop ls --json` (intake Item 2, assumptions #4, #9)

- R2.1 Add a `--json` bool flag to `hop ls` producing machine-readable output for BOTH default and `--trees` modes (they compose).
- R2.2 Default `--json`: a JSON array of `{name, path, url, group}` repo objects (mirrors `wt list --json` field naming).
- R2.3 `--json --trees`: each repo object additionally carries a nested `worktrees` array of `{name, path, dirty *bool omitempty, unpushed *int omitempty}` (pointer-field + omitempty so "not computed" is distinguishable from zero/clean).
- R2.4 GIVEN a non-cloned repo (`--trees`) THEN its object has `cloned: false` and omits `worktrees`.
- R2.5 GIVEN a per-repo `wt list` failure (`--trees`) THEN its object carries an `error` string field; the array MUST NEVER be aborted (matches text mode's never-abort contract).
- R2.6 GIVEN the FIRST `wt list` invocation hits `proc.ErrNotFound` (wt missing) THEN keep existing fail-fast: `wtMissingHint` to stderr, exit 1, NO JSON emitted.
- R2.7 GIVEN an empty repo list THEN `--json` emits `[]` (valid JSON), not empty output.
- R2.8 Ordering MUST preserve YAML source order (via `repos.FromConfig`).

### R3 — TTY-aware fzf guard (intake Item 3, assumptions #5, #7, #8, #11)

- R3.1 Add `golang.org/x/term` to `src/go.mod` + `src/go.sum`.
- R3.2 Introduce a package-level seam `var isTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }` in `cmd/hop` (swappable for tests; mirrors idea's `IsTTY`).
- R3.3 Guard ALL fzf-spawning paths: `resolve.go::resolveByName` immediately before `pickResolve(...)`, and `config_rm.go::pickRepo` immediately before `pickOne(...)`.
- R3.4 WHEN `!isTTY()` at a guarded path THEN return a typed sentinel `errNoTTY` instead of spawning fzf.
- R3.5 `errNoTTY` MUST map to a NEW DISTINCT exit code **3** in `main.go::translateExit` AND `shim_plan.go::shimResolveErr`, with stderr line: ``hop: no TTY for interactive selection — pass a repo name or use `hop ls --json` ``.
- R3.6 The guard covers bare `hop`, ambiguous/zero-match named resolution, `hop clone` (no name), and `hop rm` (no name) — single guard point per seam.

### R4 — `HOP_WRAPPER` hint suppression (intake Item 4, assumptions #6, #10)

- R4.1 Shim side: `posixInit` MUST `export HOP_WRAPPER=1` (hop-specific, NOT `WT_WRAPPER`).
- R4.2 Binary side: in `root.go`, before returning `bareNameHint` / `cdHint` / `toolFormHintFmt`, wrap the hint-emission in `if os.Getenv("HOP_WRAPPER") != "1" { ... }` — suppress the hint TEXT only.
- R4.3 The exit-2 (usage) code MUST be KEPT regardless of `HOP_WRAPPER` (mirror wt's "suppress the hint, not the error").

### Cross-cutting constraints

- C1 (Constitution I): shim stays logic-free (no `eval` of binary output); inlined PASSTHROUGH keeps the no-injection property.
- C2 (Constitution IV): mirror wt/idea precedents, do not invent.
- C3: Module rooted at `src/go.mod` — run go commands from inside `src/`.
- C4: Test Integrity — tests conform to the spec, never the reverse.
- C5: Build/vet/test MUST pass (`cd src && go build ./... && go vet ./... && go test ./...`).

## Tasks

- [x] T1 [Item 1] Inline `_hop_passthrough` body into the `PASSTHROUGH)` arm of `hop()` in `shell_init.go::posixInit`, remove the standalone `_hop_passthrough()` definition, update the doc comments, and add `export HOP_WRAPPER=1` (item 4 shim side). (R1.1–R1.3, R4.1)
- [x] T2 [Item 1/4] Update `shell_init_test.go`: flip `TestShellInitZshPassthroughUsesUnifiedCDChannel` to assert NO standalone `_hop_passthrough` definition and that the PASSTHROUGH arm still references `WT_CD_FILE` / mktemp / `-s` test / `rm -f`; flip `TestShellInitZshOmitsLegacyShape` (WT_WRAPPER fragment) and add an assertion that `export HOP_WRAPPER=1` IS present. (R1.2, R1.4, R4.1, C4)
- [x] T3 [Item 3] Add `golang.org/x/term` dependency: run `go get golang.org/x/term` inside `src/`, update `go.mod`/`go.sum`. (R3.1)
- [x] T4 [Item 3] Add the `isTTY` seam in `cmd/hop` (new small file `tty.go` or top of `resolve.go`), the `errNoTTY` sentinel, and guard `resolveByName` (before `pickResolve`) and `pickRepo` (before `pickOne`). (R3.2–R3.4, R3.6)
- [x] T5 [Item 3] Map `errNoTTY` → exit 3 in `main.go::translateExit` and `shim_plan.go::shimResolveErr`, emitting the no-TTY stderr line. (R3.5)
- [x] T6 [Item 3] Add tests injecting `isTTY=false`: `resolve_test.go` (resolveByName no-TTY → errNoTTY, never spawns fzf; shimResolveErr → 3; translateExit → 3) and `config_rm_test.go` (pickRepo no-TTY → exit 3, never spawns fzf). (R3.4, R3.5, C4)
- [x] T7 [Item 2] Implement `hop ls --json` in `ls.go`: `--json` flag, repo/worktree JSON structs (mirroring wt field names), `runLsJSON` (default) and `runLsTreesJSON` (--trees) producing the schema, edge states (non-cloned → `cloned:false`, per-repo failure → `error`, first ErrNotFound → fail-fast, empty → `[]`), preserving source order. (R2.1–R2.8)
- [x] T8 [Item 2] Add `ls_test.go` cases: default `--json`, `--json --trees`, non-cloned, per-repo wt failure, first-ErrNotFound fail-fast, empty `[]`. (R2.1–R2.8, C4)
- [x] T9 [Item 4] In `root.go`, wrap `bareNameHint` / `cdHint` / `toolFormHintFmt` emission in `if os.Getenv("HOP_WRAPPER") != "1"`, keeping exit-2. (R4.2, R4.3)
- [x] T10 [Item 4] Add root hint-suppression tests (bare_name_test.go or root): `HOP_WRAPPER=1` suppresses the hint text but exit code stays 2. (R4.2, R4.3, C4)
- [x] T11 [verify] `cd src && go build ./... && go vet ./... && go test ./...` — scope to `cmd/hop` first, then full. (C5)

## Execution Order

T1 → T2 (item 1 + item 4 shim side). T3 → T4 → T5 → T6 (item 3, dep chain: dep first, then seam+guard, then exit mapping, then tests). T7 → T8 (item 2). T9 → T10 (item 4 binary side). T11 last (full verification). T1, T3, T7, T9 are independent across items and could interleave; tests follow their implementation tasks.

## Acceptance

- [x] A1 `hop shell-init zsh` emits a single `hop()` function with the inlined PASSTHROUGH body (WT_CD_FILE verbatim) and NO standalone `_hop_passthrough()`; `export HOP_WRAPPER=1` present. (R1, R4.1)
- [x] A2 `hop ls --json` emits `[{name,path,url,group}, ...]`; empty list → `[]`. (R2.2, R2.7)
- [x] A3 `hop ls --json --trees` nests `worktrees` with omitempty `dirty`/`unpushed`; non-cloned → `cloned:false` no worktrees; per-repo failure → `error` field, array survives; first ErrNotFound → fail-fast exit 1 no JSON. (R2.3–R2.6)
- [x] A4 With `isTTY=false`, bare `hop` / ambiguous-name / `hop rm` (no name) / `hop clone` (no name) return exit 3 with the no-TTY stderr line and never spawn fzf. (R3.4–R3.6)
- [x] A5 `HOP_WRAPPER=1` suppresses `bareNameHint`/`cdHint`/`toolFormHintFmt` text while keeping exit 2. (R4.2, R4.3)
- [x] A6 `go build ./... && go vet ./... && go test ./...` pass from `src/`. (C5)

## Assumptions

(Apply-time SRAD assumptions recorded here. Intake decisions #1–#11 are settled and not re-litigated; #12 is hydrate-time.)

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| A-1 | Certain | `isTTY` seam lives in a new tiny `src/cmd/hop/tty.go` file (one var + the `golang.org/x/term`/`os` imports) rather than appended to `resolve.go` | Keeps the new dependency import localized and the seam discoverable; mirrors the file-per-concern convention (`wt_list.go` houses the `listWorktrees` seam). Intake #11 only fixes the seam *form* (a `cmd/hop` package-level var), not the file. | S:88 R:78 A:85 D:80 |
| A-2 | Certain | The no-TTY guard checks `os.Stdin.Fd()` only (not stderr) | Intake §Item 3 default snippet is `term.IsTerminal(int(os.Stdin.Fd()))`; fzf reads its selection list from stdin, so stdin is the fd that matters for "can the user pick". Single-fd check matches idea's `IsTTY(os.Stdout)` simplicity. | S:90 R:75 A:82 D:80 |
| A-3 | Certain | JSON structs use a dedicated `lsRepoJSON` / `lsWorktreeJSON` shape with `encoding/json` `MarshalIndent` (2-space) mirroring wt's emission, not reusing `repos.Repo`/`WtEntry` directly | `repos.Repo` has no json tags and exposes `Dir`; `WtEntry` uses value `Dirty bool`/`Unpushed int` (not the pointer+omitempty wt schema requires). A purpose-built output struct is the clean mirror of wt's `listEntry`. | S:90 R:70 A:85 D:78 |
| A-4 | Certain | `dirty`/`unpushed` in `--json --trees` are ALWAYS populated (non-nil) because hop's `WtEntry` always carries them (wt list --json without --status omits them, but hop unmarshals into value fields defaulting to false/0) | hop's `listWorktrees` calls `wt list --json` (no `--status`), so wt omits dirty/unpushed and hop's value-typed `WtEntry` defaults them to false/0. Mapping value→pointer always yields non-nil. The omitempty pointer schema is still the correct mirror of wt's contract (forward-compatible if hop ever adds --status); for now the fields are present-with-zero. This matches the text-mode behavior (`formatTreesRow` reads `e.Dirty`/`e.Unpushed` as zero). Documented so reviewers don't expect omission. | S:80 R:65 A:75 D:70 |

4 apply-time assumptions (4 certain, 0 confident, 0 tentative). All low blast-radius; none re-open a settled intake decision.
