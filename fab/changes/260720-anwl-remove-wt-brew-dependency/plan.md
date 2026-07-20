# Plan: Remove wt Homebrew Dependency

**Change**: 260720-anwl-remove-wt-brew-dependency
**Intake**: `intake.md`

## Requirements

### Packaging: Formula template

#### R1: Formula template SHALL NOT declare a wt dependency
The `.github/formula-template.rb` template SHALL NOT contain the line `depends_on "sahil87/tap/wt"`. The template MUST otherwise remain byte-identical (placeholders, on_macos/on_linux blocks, install/test blocks unchanged) so the release workflow's `sed` substitution contract is unaffected.

- **GIVEN** the release workflow regenerates `Formula/hop.rb` from `.github/formula-template.rb` on a tagged release
- **WHEN** the next `v*` tag is pushed
- **THEN** the published formula carries no `depends_on "sahil87/tap/wt"` line
- **AND** `brew install sahil87/tap/hop` no longer pulls wt

### CLI: wt-missing hint

#### R2: wt-missing hint MUST carry the install command
The `wtMissingHint` constant in `src/cmd/hop/resolve.go` MUST have the exact value `hop: wt is not installed. Install it: brew install sahil87/tap/wt`. The exit-code/stderr contract is unchanged: every wt-requiring surface (`open.go:33`, `resolve.go` worktree-resolution branch, `ls.go` `--trees` text and `--json` fail-fast) still prints the constant to stderr and exits 1. The constant's doc comment SHALL be updated only where it restates the old wording ("not on PATH"); the shared-across-surfaces contract description stays.

- **GIVEN** wt is not installed (any wt call site hits `proc.ErrNotFound`)
- **WHEN** the user runs `hop <name> open`, `hop <name>/<wt>`, or `hop ls --trees` (with or without `--json`)
- **THEN** stderr carries exactly `hop: wt is not installed. Install it: brew install sahil87/tap/wt`
- **AND** the process exits 1

#### R3: No test-file edits — verification only
Test files MUST NOT be edited: all test references use the `wtMissingHint` symbol (`ls_test.go:187,362` via `strings.Contains`; `resolve_test.go:472` via verbatim equality), so the value change propagates automatically. The apply task SHALL run the project's CI checks (`gofmt -l .`, `go vet ./...`, `go test ./...` from `src/`) and confirm green with zero test-file edits. If a test unexpectedly hardcodes the old literal, the test is fixed to use the const (Test Integrity: spec/implementation is authoritative).

- **GIVEN** the const value change in `resolve.go`
- **WHEN** `gofmt -l .`, `go vet ./...`, and `go test ./...` run from `src/`
- **THEN** all pass with no test file modified

### Non-Goals

- No tap-side manual edit to the already-published `Formula/hop.rb` — the next tagged release regenerates it from the template.
- No control-flow, exit-code, or API changes to any wt call site.
- No spec edits (`docs/specs/cli-surface.md`, `docs/specs/architecture.md` reference the old wording — flagged for human follow-up, per intake).
- Memory updates (`cli/subcommands`, `cli/match-resolution`, `build/release-pipeline`) are hydrate's scope, not apply's.

## Tasks

### Phase 2: Core Implementation

- [x] T001 [P] Delete the line `  depends_on "sahil87/tap/wt"` (and its following blank line, keeping exactly one blank line between the header block and `on_macos`) from `.github/formula-template.rb` <!-- R1 -->
- [x] T002 [P] In `src/cmd/hop/resolve.go`, change `const wtMissingHint = "hop: wt: not found on PATH."` to `const wtMissingHint = "hop: wt is not installed. Install it: brew install sahil87/tap/wt"`; update the doc comment's "needed but not on PATH" phrasing to match the new "not installed" wording, keeping the shared-across-surfaces contract description <!-- R2 -->

### Phase 3: Integration & Edge Cases

- [x] T003 Run CI checks from `src/`: `gofmt -l .` (empty output), `go vet ./...`, `go test ./...` — confirm green with zero test-file edits (`git status` shows only `resolve.go` and `formula-template.rb` modified) <!-- R3 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `.github/formula-template.rb` contains no `depends_on` line; the rest of the template (placeholders, blocks) is unchanged
- [x] A-002 R2: `wtMissingHint` in `src/cmd/hop/resolve.go` equals exactly `hop: wt is not installed. Install it: brew install sahil87/tap/wt`

### Behavioral Correctness

- [x] A-003 R2: All four wt-missing surfaces (`open.go`, `resolve.go` worktree branch, `ls.go` text + JSON `--trees`) still print the constant to stderr and exit 1 — no call-site code changed

### Scenario Coverage

- [x] A-004 R3: `go test ./...` from `src/` passes; `ls_test.go` and `resolve_test.go` assertions pass against the new value via the const symbol

### Edge Cases & Error Handling

- [x] A-005 R3: No test file was edited (git diff touches only `.github/formula-template.rb` and `src/cmd/hop/resolve.go`)

### Code Quality

- [x] A-006 Pattern consistency: the new hint mirrors the sibling `fzfMissingHint` shape (`hop: <tool> is not installed. Install it: <command>`)
- [x] A-007 No unnecessary duplication: message stays a single shared constant; no literal duplicated at call sites
- [x] A-008 Magic strings: install command lives only in the named constant, not inline

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Also remove the blank line following the deleted `depends_on` line, leaving one blank line between the header and `on_macos` | Intake specifies only the line delete; leaving a double blank line would be untidy Ruby formatting and the template's structure is otherwise single-blank-separated | S:70 R:95 A:90 D:85 |
| 2 | Confident | Reword the doc comment's "needed but not on PATH" to "needed but not installed" (keeping the shared-surfaces contract text) | Intake assumption 4: update only where the comment restates the old wording; "not on PATH" restates it, the rest describes behavior | S:65 R:95 A:85 D:80 |

2 assumptions (0 certain, 2 confident, 0 tentative).
