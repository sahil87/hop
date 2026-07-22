# Plan: Rewrite Root Help Text for Readability

**Change**: 260722-3j8c-rewrite-root-help-text
**Intake**: `intake.md`

## Requirements

### CLI Help Surface: `rootLong` rewrite

#### R1: `rootLong` value is the user-approved replacement text, adopted verbatim
The value of the `rootLong` const in `src/cmd/hop/root.go` MUST be exactly the following user-approved replacement text — no rewording, no re-wrapping, no added or dropped lines:

```
hop — locate, open, and operate on repos from hop.yaml.

Grammar: hop <selection> [action]
  <selection>   a repo name (substring; fzf on ambiguity), a <name>/<wt>
                worktree, a group name, or --all (every cloned repo)
  [action]      builtin verb (cd, where, open), batch verb (pull, push,
                sync), or any PATH tool / shell alias (git pull, code ., p).
                Omitted: cd into the selection.

Getting started:
  1. Wire the shell shim:  shll shell-setup
  2. Build the config:     hop add -r ~/code

Examples:
  hop                     fzf picker, print the selection
  hop <name>              cd into the repo (needs the shell shim)
  hop <name>/<wt>         same, rooted at worktree <wt>
  hop <name> where        print the repo's absolute path
  hop <name> open         open the repo in an app (wt's menu)
  hop <name> code .       run any PATH tool in the repo dir
  hop <name> sync         auto-commit, git pull --rebase, git push
  hop <group> pull        batch verb across a group's cloned repos
  hop --all sync          batch verb across every cloned repo

Management subcommands (clone, ls, add, rm, config, ...) are listed
below — see `hop <command> -h` for each.

Notes:
  - cd and tool-form run in your parent shell via the shim (shll shell-setup,
    or eval "$(hop shell-init zsh)"). Without it, use cd "$(hop <name> where)".
  - A group or --all accepts only pull/push/sync; cd, open, where, and
    arbitrary tools are single-repo.
  - Ambiguous or no-match queries open fzf prefilled with your query.
  - Config lives at ~/.config/hop/hop.yaml.
```

- **GIVEN** the approved replacement text above
- **WHEN** `hop -h` is run (or the `help-dump` root node's `text` is read)
- **THEN** the help output begins with this text byte-for-byte, followed by cobra's auto-generated sections (Usage, Available Commands, Flags)

#### R2: Backtick escaping preserves the existing Go string-concatenation pattern
Backtick characters inside the approved text (the `` `hop <command> -h` `` segment in the Management-subcommands pointer) MUST be produced via the file's existing string-concatenation escaping pattern (raw-string segments joined with `+ "`" + …` — a Go raw string literal cannot contain a backtick). The concatenated const value MUST be byte-identical to the approved text in R1.

- **GIVEN** the approved text contains backticks around `hop <command> -h`
- **WHEN** the const is authored in `src/cmd/hop/root.go`
- **THEN** the backticks are contributed by interpreted-string segments (`+ "`" + …`) exactly as the current `rootLong` const does, and the resulting runtime string equals the R1 text byte-for-byte

#### R3: Single-const production edit — no behavior changes, tests conform to the new spec
The production change MUST touch only the `rootLong` const value in `src/cmd/hop/root.go`. Cobra wiring, hint constants (`bareNameHint`, `cdHint`, `toolFormHintFmt`), all subcommands, and all behavior stay untouched. `go test ./cmd/hop` (run from `src/`) MUST pass. `TestHelpDumpRootTextUsesLong` asserts against the const itself (content-agnostic — no edit). Test assertions that hard-code removed cheat-sheet rows MUST be updated to assert the new discoverability location (cobra's Available Commands section) per the constitution's Test Integrity clause — tests conform to the spec, never the other way around. *(Discovered at apply: the intake's "zero test edits" expectation held for the const-referencing test but not for two content-coupled assertions — see Assumptions #3.)*

- **GIVEN** the `rootLong` const has been replaced per R1/R2
- **WHEN** `go test ./cmd/hop` is run from the `src/` directory
- **THEN** all tests pass (excluding failures already present on the clean tree)
- **AND** `git diff` shows production changes only to the `rootLong` const value, plus test-assertion updates confined to content-coupled help checks (`TestUpdateAppearsInHelp`, `TestIntegrationTopLevelAddRm`'s help subtest)

### Non-Goals

- No update to `docs/specs/cli-surface.md` — specs are human-curated; not modified by this change (intake assumption #10)
- No memory updates during apply — the `cli/subcommands` description refresh is hydrate's job
- No change to the `help-dump` JSON schema/envelope — only the root node's `text` content changes
- No switch to concrete example names — `<name>`-style placeholders stay (user did not opt in)

## Tasks

### Phase 2: Core Implementation

- [x] T001 Replace the value of the `rootLong` const in `src/cmd/hop/root.go` (currently lines 10–71) with the approved text from R1, escaping the single backticked segment (`hop <command> -h`) via the existing `+ "`" + …` concatenation pattern; touch nothing else in the file <!-- R1, R2 -->

### Phase 3: Integration & Edge Cases

- [x] T002 Run `go test ./cmd/hop` from the `src/` directory; update any test assertions hard-coded to removed cheat-sheet rows so they assert cobra's Available Commands section instead (Test Integrity: tests conform to spec); confirm the suite is green apart from failures already present on the clean tree, and confirm the production diff is confined to the `rootLong` const <!-- R3 -->
- [x] T003 Build the binary and run `hop -h`; diff the leading `rootLong` portion of the rendered output against the approved draft to confirm a byte-for-byte match (where representable in terminal output) <!-- R1 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `hop -h` output (and the `help-dump` root node `text`) begins with the approved replacement text from R1, byte-for-byte

### Behavioral Correctness

- [x] A-002 R2: The `rootLong` const uses the existing raw-string + `+ "`" + …` concatenation pattern for the backticked `hop <command> -h` segment, and the concatenated value equals the R1 text exactly

### Scenario Coverage

- [x] A-003 R3: `go test ./cmd/hop` passes from `src/` (excluding the two pre-existing clean-tree failures, `TestConfigAddNonConventionInventsGroup` / `TestRecursiveAddNonConventionInventsGroup` — macOS temp-dir symlink artifact, unrelated); the production diff is confined to the `rootLong` const value in `src/cmd/hop/root.go`; test edits are confined to the two content-coupled help assertions and preserve their discoverability intent

### Code Quality

- [x] A-004 Pattern consistency: the edited const follows the file's existing style — raw-string literal, interpreted-string concatenation only where backticks occur, no trailing newline before the closing backtick
- [x] A-005 No unnecessary duplication: no new constants, helpers, or files introduced; existing const simply revalued

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | The const value carries no trailing newline (ends at `hop.yaml.`) | The intake's fenced block is newline-agnostic at its end; the existing `rootLong` const ends without a trailing newline and cobra's Long convention matches — adopting the same ending keeps the `rootLong+"\n\n"` test prefix and rendered output stable | S:80 R:95 A:95 D:90 |
| 2 | Certain | Only the `hop <command> -h` segment needs backtick concatenation; every other approved line is raw-string-safe | Verified by inspection of the approved text — it contains exactly one backticked span; all quotes/em-dashes/`$(...)` are legal inside a Go raw string | S:90 R:95 A:100 D:95 |
| 3 | Certain | Two content-coupled help assertions (`TestUpdateAppearsInHelp` in `update_test.go`, the `add and rm appear in hop --help` subtest in `integration_test.go`) are updated to assert cobra's Available Commands Shorts instead of the removed cheat-sheet rows | Discovered at apply — they hard-coded `hop update` / `hop add <dir>` / `hop rm [<name>]` from the old cheat sheet, which the approved text (the spec) deliberately removes; constitution § Test Integrity mandates updating tests to the spec, and their discoverability intent is preserved at the new location. Falsifies intake assumption #8's "zero test edits" expectation for these two tests only; `TestHelpDumpRootTextUsesLong` was content-agnostic as predicted | S:85 R:90 A:95 D:90 |
| 4 | Certain | The two clean-tree test failures (`TestConfigAddNonConventionInventsGroup`, `TestRecursiveAddNonConventionInventsGroup`) are out of scope and left untouched | Verified pre-existing by stashing the change and re-running: they fail identically on an unmodified tree (macOS `/var → /private/var` temp-dir symlink defeats the `~`-substitution expectation) — an environment artifact unrelated to help text; fixing them here would violate the approved single-const scope | S:90 R:95 A:90 D:90 |

4 assumptions (4 certain, 0 confident, 0 tentative).
