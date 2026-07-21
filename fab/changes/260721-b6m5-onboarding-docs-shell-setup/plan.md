# Plan: Onboarding Docs Shell-Setup Alignment

**Change**: 260721-b6m5-onboarding-docs-shell-setup
**Intake**: `intake.md`

## Requirements

### CLI Help: root "Getting started" block

#### R1: Two-command first-run story in `hop -h`
The `rootLong` constant's "Getting started" block in `src/cmd/hop/root.go` MUST teach the canonical two-command first-run story: (1) wire the shell shim via `shll shell-setup`, (2) bootstrap the config via `hop add -r ~/code`. The `hop config init` → hand-edit-yaml path MUST be removed from "Getting started". The block MUST note `eval "$(hop shell-init zsh)"` as the alternative for installs without shll (install-composition Policy A — hop never assumes a sibling is installed). `hop config init` MUST remain listed in the cheat sheet row (manual alternative — demoted, not deleted). The runtime shim hints (`bareNameHint`, `cdHint`, `toolFormHintFmt`) MUST NOT change.

- **GIVEN** a fresh install of hop
- **WHEN** the user runs `hop -h`
- **THEN** the "Getting started" block recommends `shll shell-setup` then `hop add -r ~/code`, with the `eval "$(hop shell-init zsh)"` fallback noted
- **AND** the cheat sheet still carries the `hop config init` row and the shim hint constants are byte-identical

### Config: no-config error message

#### R2: `Resolve()` not-found message leads with the bootstrap form
The not-found error in `src/internal/config/resolve.go` `Resolve()` MUST lead with `hop add -r ~/code` (the canonical fresh-machine bootstrap) instead of `hop add <dir>`. The message MUST keep the `hop: no hop.yaml found at <path>` lead shape (tests and memory match on that substring) and SHOULD retain `'hop config init' for a starter` as the trailing alternative. Tests asserting the old string (`src/internal/config/resolve_test.go:53` exact match; `src/cmd/hop/config_test.go:173` and `:1031` substring `Run 'hop add <dir>'`) MUST be updated to the new spec text (Constitution § Test Integrity). `src/cmd/hop/config_test.go:170` (shape substring) and `src/cmd/hop/config_add_test.go:54` (absence of the old rm-gate wording) remain valid without change. `config_rm.go`'s separate missing-config wording is OUT of scope (backlog `[d3wq]`).

- **GIVEN** a fresh machine with no `~/.config/hop/hop.yaml`
- **WHEN** any read-command runs (`hop`, `hop ls`, `hop config print`, `hop add -p`)
- **THEN** the error reads `hop: no hop.yaml found at <path>.` followed by a lead recommendation of `hop add -r ~/code` and a trailing `hop config init` alternative
- **AND** `go test ./cmd/hop ./internal/config` passes from `src/`

### Docs: install guide shim wiring

#### R3: `shll shell-setup` is the recommended wiring path; stale alias name fixed
`docs/site/install.md` §2 ("Wire the shell shim") MUST lead with `shll shell-setup` as the recommended wiring path (one idempotent command wiring every installed shll tool's shell integration + completions into the rc file), demoting the manual `eval "$(hop shell-init zsh)"` rc-line instructions to the fallback for installs without shll (from-source installs — Policy A). The explanatory content of §2 (why the shim exists, how dispatch works, the three things `shell-init` prints) MUST be preserved. Every `shll shell-install` reference (install.md:62, README.md:101) MUST be updated to the canonical name `shll shell-setup`. hop docs say `shll shell-setup` plain and link to shll.ai — shll's tap-trust matrix MUST NOT be duplicated in hop docs (Policy B anti-drift).

- **GIVEN** a reader of `docs/site/install.md` §2 who installed via the shll curl bootstrap
- **WHEN** they follow the recommended wiring path
- **THEN** they run `shll shell-setup` (single idempotent command); the manual eval lines appear as the no-shll fallback
- **AND** `grep -rn "shell-install" README.md docs/site/` returns nothing

#### R4: Copy-pasteable 3-line TL;DR quick start in install.md and README.md
A copy-pasteable 3-line quick start — curl install (`curl -fsSL https://shll.ai/install | sh -s -- hop`) → `shll shell-setup` → `hop add -r ~/code` — MUST appear at the top of `docs/site/install.md` and inside README.md's existing `## Install` section (extending the curl line already there; no new top-level README section). No per-formula `brew install` lines are added anywhere (install-composition Policy B). New links leaving the published set MUST be absolute `https://` URLs (readme-extraction standard).

- **GIVEN** a new user landing on README.md or docs/site/install.md
- **WHEN** they copy the quick-start block
- **THEN** the three commands (install → `shll shell-setup` → `hop add -r ~/code`) take them from nothing installed to a populated `hop.yaml`
- **AND** no `brew install` line is introduced in either file

### Non-Goals

- A new `hop setup`/`hop doctor` subcommand — rejected (Constitution Principle VI).
- Homebrew tap formula caveats; empty-state hints on `hop ls`/bare `hop` — deferred, separate changes.
- `config_rm.go` missing-config wording — backlog `[d3wq]`.
- Binary runtime shim hints (`bareNameHint`/`cdHint`/`toolFormHintFmt`) — unchanged (Policy A).
- `docs/memory/` updates (quoted `Resolve()` string in `config/search-order`, `cli/subcommands`) — hydrate's job, not apply's.

## Tasks

### Phase 2: Core Implementation

- [x] T001 Rewrite the "Getting started" block inside `rootLong` in `src/cmd/hop/root.go` to the two-command story (`shll shell-setup` → `hop add -r ~/code`), with the `eval "$(hop shell-init zsh)"` fallback noted; leave the cheat sheet's `hop config init` row and all hint constants untouched <!-- R1 -->
- [x] T002 Rewrite the `Resolve()` not-found message in `src/internal/config/resolve.go:37` to lead with `hop add -r ~/code`, keeping the `hop: no hop.yaml found at <path>` shape and the trailing `'hop config init' for a starter` alternative <!-- R2 -->
- [x] T003 Update the string assertions to the new spec text: exact-match `want` in `src/internal/config/resolve_test.go:53`; `Run 'hop add <dir>'` substrings in `src/cmd/hop/config_test.go:173` and `:1031`; verify `config_test.go:170` and `config_add_test.go:54` still hold unchanged <!-- R2 -->

### Phase 3: Docs

- [x] T004 [P] Restructure `docs/site/install.md`: add the 3-line TL;DR quick start at the top; invert §2 to lead with `shll shell-setup` (manual eval demoted to the no-shll/from-source fallback, explanatory content preserved); replace the stale `shll shell-install` reference <!-- R3, R4 -->
- [x] T005 [P] Update `README.md`: extend `## Install` with the 3-line TL;DR; replace the stale `shll shell-install` reference at line ~101 with `shll shell-setup` (absolute shll.ai link) <!-- R3, R4 -->

### Phase 4: Verification

- [x] T006 From `src/`: `gofmt -l .`, `go vet ./...`, `go test ./cmd/hop ./internal/config`; then sweep standards conformance — `grep -rn "shell-install" README.md docs/site/` empty, no new `brew install` lines, new external links absolute (readme-extraction/install-composition/help-dump) <!-- R1, R2, R3, R4 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `hop -h`'s "Getting started" recommends `shll shell-setup` then `hop add -r ~/code` (with the `eval "$(hop shell-init zsh)"` fallback); the `hop config init` step is gone from the block
- [x] A-002 R2: the `Resolve()` not-found message leads with `hop add -r ~/code`, keeps the `hop: no hop.yaml found at <path>` shape, and retains the trailing `hop config init` alternative
- [x] A-003 R3: install.md §2 leads with `shll shell-setup`; the manual eval lines survive as the no-shll fallback; §2's explanatory content (Unix constraint, three things shell-init prints, dispatch model) is preserved
- [x] A-004 R4: the 3-line TL;DR (curl → `shll shell-setup` → `hop add -r ~/code`) appears at the top of install.md and inside README.md's `## Install` section

### Behavioral Correctness

- [x] A-005 R1: cheat sheet still lists `hop config init  bootstrap a starter hop.yaml`; `bareNameHint`, `cdHint`, `toolFormHintFmt` are byte-identical
- [x] A-006 R2: `go test ./cmd/hop ./internal/config` passes from `src/`; assertions match the new message; `config_add_test.go`'s absence check still passes

### Removal Verification

- [x] A-007 R3: `grep -rn "shell-install" README.md docs/site/` returns no matches

### Scenario Coverage

- [x] A-008 R2: `TestResolveNotFound` exact-string test exercises the new message verbatim; `TestConfigPrintNoConfigErrors` covers read-command propagation

### Edge Cases & Error Handling

- [x] A-009 R4: no per-formula `brew install` lines exist in README.md or docs/site/ (Policy B); shll's tap-trust matrix is not duplicated — hop docs say `shll shell-setup` plain and link shll.ai

### Code Quality

- [x] A-010 Pattern consistency: rewritten help/error prose follows the surrounding style (backtick-quoted commands in `rootLong`, single-quoted commands in error strings); docs edits follow existing heading/voice conventions
- [x] A-011 No unnecessary duplication: install mechanics stay centralized (links to shll.ai rather than repeating shll's install/trust documentation)
- [x] A-012 No magic strings introduced: message changes stay in the existing constants/format strings; no new constants without need

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | New `Resolve()` message text: `hop: no hop.yaml found at <path>. Run 'hop add -r ~/code' to build it from your existing clones, or 'hop config init' for a starter.` | Intake fixes the lead form and shape, leaves exact prose to apply (intake Assumption 9/10); "build it from your existing clones" mirrors README/install.md's bootstrap language | S:75 R:88 A:80 D:70 |
| 2 | Confident | install.md's "One-shot wiring" trailing subsection is absorbed into the new §2 lead (shell-setup primary) rather than kept as a separate subsection | Keeping a separate "one-shot" subsection after shell-setup becomes the lead would duplicate it; §2's explanatory content is preserved as required | S:70 R:90 A:80 D:70 |
| 3 | Confident | README's "Shell integration" 💡 tip is reworded to name `shll shell-setup` (with an absolute shll.ai link) — same sentence, canonical name | The stale-name fix at README:101 is in approved scope; leaving the old anchor-style GitHub link would keep a stale deep link to a renamed section | S:70 R:90 A:85 D:75 |
| 4 | Confident | TL;DR comment phrasing follows the intake's illustrative block verbatim (`# install hop (+ shll) via Homebrew` / `# wire the shell shim into your rc file` / `# walk your code dir, build hop.yaml from git remotes`) | Intake marks comments "illustrative — exact phrasing decided at apply"; the quoted phrasing is accurate as-is | S:65 R:95 A:85 D:70 |

4 assumptions (0 certain, 4 confident, 0 tentative).
