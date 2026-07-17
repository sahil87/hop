# Plan: Toolkit Standards Conformance

**Change**: 260717-fcvp-toolkit-standards-conformance
**Intake**: `intake.md`

## Requirements

Requirements are derived from the runtime-enumerated shll standards (shll v0.0.23:
`principles`, `help-dump`, `readme-extraction`, `skill`) and the intake's fix policy
(§3: fix all mechanical-contract violations + small additive principle gaps; defer
restructuring). The audit itself is the primary deliverable; each requirement below
is a conformance obligation the audit either verifies as PASS or fixes.

### Standard: help-dump (binary)

#### R1: help-dump envelope MUST NOT emit `captured_at`
The `hop help-dump` JSON envelope MUST be exactly `{tool, version, schema_version, root}`.
The `captured_at` field MUST NOT be present — the standard (shll v0.0.23) assigns the
capture timestamp to the shll.ai puller, which stamps it after capture; a tool cannot
know its own capture time.

- **GIVEN** the current binary emits `{tool, version, captured_at, schema_version, root}`
- **WHEN** `hop help-dump` runs after the fix
- **THEN** the parsed JSON has exactly the keys `tool`, `version`, `schema_version`, `root`
- **AND** `captured_at` is absent from the top-level object
- **AND** exit code is 0, stdout is valid JSON, stderr is empty

#### R2: help-dump conformance test pins the envelope shape
The help-dump test suite MUST pin the conformant envelope: exit 0, valid JSON,
expected `tool`/`schema_version`, and the **absence** of `captured_at`.

- **GIVEN** the standard's "keep (or add) a minimal test" clause
- **WHEN** the test suite runs
- **THEN** a test asserts `captured_at` is absent from the emitted document
- **AND** the pre-existing tool/version/schema_version/filter assertions still hold

### Standard: readme-extraction (repo)

#### R3: README command-reference link uses the canonical absolute URL
The README's command-reference cross-link MUST be the absolute URL
`https://shll.ai/hop/commands/` (per rule 8: "point at the generated command
reference with the absolute URL `https://shll.ai/<tool>/commands/`"). The current
`https://shll.ai/tools/hop/commands/` carries a non-canonical `/tools/` segment
that 404s against the standard's slug scheme (`/<tool>/…`).

- **GIVEN** README line 248 links to `https://shll.ai/tools/hop/commands/`
- **WHEN** the README is corrected
- **THEN** the command-reference link reads `https://shll.ai/hop/commands/`
- **AND** all other readme-extraction checklist items remain PASS (head order,
  tail slice, absolute images, docs/site relative links auto-rewritten, no mermaid,
  no gh-mode fragments, no reserved page names)

### Standard: principles (foundation)

#### R4: Destructive registry writes support `--dry-run`
`hop rm` mutates `hop.yaml` (a destructive registry write). Per principle №5
(visible mutation boundaries), a destructive write MUST support `--dry-run` — an
accurate preview that shares the real code path and writes nothing. `hop rm`
currently rejects `--dry-run` (`unknown flag`). Add a `--dry-run` flag to both the
canonical `hop rm [<name>]` and the hidden `hop config rm` alias that resolves the
target(s) through the same resolution path but skips `yamled.RemoveURL`, reporting
what *would* be removed.

- **GIVEN** `hop.yaml` registers `alpha` and the user runs `hop rm alpha --dry-run`
- **WHEN** the command runs
- **THEN** stderr reports the would-remove target (e.g. `would remove: <url>` + `dry-run: no changes written`)
- **AND** `hop.yaml` is byte-for-byte unchanged on disk
- **AND** exit code is 0
- **AND** `hop rm --dry-run` (picker path) previews the selected entry without writing

#### R5: A CLI-surface fix updates docs/specs/cli-surface.md in the same change
Where a fix changes the CLI surface (the `hop rm` `--dry-run` flag), the canonical
contract `docs/specs/cli-surface.md` MUST document the new flag and its stream/exit
behavior in the same change (intake §3: "Where a fix touches the CLI surface, update
`docs/specs/cli-surface.md`").

- **GIVEN** R4 adds `--dry-run` to `hop rm`
- **WHEN** the spec is updated
- **THEN** the `hop rm` row / behavioral scenarios in cli-surface.md describe `--dry-run`
  (preview via the real resolution path, no write, exit 0)

#### R6: The remaining ten-principle audit is recorded, with deferrals referenced
Every principle assessed as PASS is recorded in the conformance report against hop's
actual behavior; every gap not fixed here (larger, restructuring, or SHOULD-level
documentation authoring) is recorded as a `fab/backlog.md` item and referenced by ID.

- **GIVEN** the ten principles assessed against the running binary and source
- **WHEN** the conformance report is written
- **THEN** each principle carries PASS (with evidence) or a dispositioned gap
- **AND** the mandatory-consent question on `hop rm` (principle №5/№1) is deferred to a backlog ID
- **AND** the missing CLAUDE.md/AGENTS.md agent-entry pointer (principle №10 SHOULD) is deferred to a backlog ID

### Standard: skill (binary + repo)

#### R7: skill standard reported deferred, not adopted; backlog reference exists
hop has no `hop skill` subcommand. Per the standard's own phased-adoption clause and
the intake's pre-decided disposition, `hop skill` MUST NOT be implemented in this
change. A `fab/backlog.md` item recording skill adoption MUST exist as the deferral
reference.

- **GIVEN** hop ships no `hop skill` subcommand
- **WHEN** the conformance report's `skill` section is written
- **THEN** it reads "deferred, not yet adopted" with a backlog ID reference
- **AND** no `hop skill` code is added

### Deliverable

#### R8: Conformance report captures the full audit, one section per standard
A `conformance-report.md` MUST exist in the change folder (destined for the PR body):
a header (shll version audited against, audit date 2026-07-18, hop version), then one
section per standard in `shll standards` order, each PASS (with verified checklist
items) or gaps (each dispositioned fixed-here or deferred-to-ID).

- **GIVEN** the audit is complete
- **WHEN** `conformance-report.md` is written
- **THEN** it carries the header (shll v0.0.23, 2026-07-18, hop v0.1.18-…) and four
  standard sections in enumeration order (principles, help-dump, readme-extraction, skill)
- **AND** every gap names the file(s) fixed or the backlog ID deferred to

### Non-Goals

- Implementing `hop skill` — explicitly deferred (R7).
- Adding a mandatory confirmation/`--yes` to `hop rm <name>` — changing its
  non-interactive contract is a restructuring, deferred (R6).
- Authoring CLAUDE.md/AGENTS.md — documentation authoring better human-curated, deferred (R6).
- Amending the constitution — the Toolkit Standards article lands via change `260717-zono`.
- Any command-tree change — no fix here adds/removes a command; `--dry-run` is a flag on
  an existing command, so the help-dump tree is unchanged (re-verification still run per intake §5).

### Design Decisions

1. **Drop `captured_at` entirely rather than keep-and-empty**: *Why*: the standard says
   "Do not emit `captured_at`" — an empty-string field still violates the envelope shape
   `{tool, version, schema_version, root}`. No CI wiring injects it today (grep of
   `.github/` finds no help-dump publish step), so removal is safe. *Rejected*: keeping
   `CapturedAt: ""` — still a spurious field a Zod `.strict()` consumer would reject.
2. **`--dry-run` on `hop rm`, defer `--yes`/consent**: *Why*: `--dry-run` is the small,
   additive, share-the-real-path fix principle №5 names directly; adding a mandatory
   prompt would break `hop rm <name>`'s documented non-interactive contract (a
   restructuring). *Rejected*: adding `--yes` + prompt now — out of the proportionality
   boundary (intake §3).

## Tasks

### Phase 1: Mechanical-contract fixes

- [x] T001 Remove the `CapturedAt` field from the `Doc` struct and `buildHelpDoc` in `src/cmd/hop/help_dump.go`; update the struct/field doc comments to describe the `{tool, version, schema_version, root}` envelope <!-- R1 -->
- [x] T002 Update `src/cmd/hop/help_dump_test.go`: replace the `captured_at == ""` assertion with an assertion that `captured_at` is absent from the raw JSON; keep the tool/version/schema_version/filter assertions <!-- R2 -->
- [x] T003 [P] Fix the README command-reference link in `README.md` from `https://shll.ai/tools/hop/commands/` to `https://shll.ai/hop/commands/` <!-- R3 -->

### Phase 2: Principle gap fix (additive flag)

- [x] T004 Add a `--dry-run` bool flag to both `newRmCmd()` and `newConfigRmCmd()` in `src/cmd/hop/config_rm.go`; thread it through `runRm` and `removeRepo` so a dry-run resolves the target(s) via the real path but skips `yamled.RemoveURL`, printing `would remove: <url>` (+ a `dry-run: no changes written` line) to stderr and exiting 0 <!-- R4 -->
- [x] T005 Add tests in `src/cmd/hop/config_rm_test.go` covering `hop rm <name> --dry-run` (no write, correct stderr, exit 0) and the picker `hop rm --dry-run` path (preview, no write) <!-- R4 -->
- [x] T006 Update `docs/specs/cli-surface.md`: document `--dry-run` on the `hop rm` row and add a behavioral scenario (preview via real resolution path, no write, exit 0) <!-- R5 -->

### Phase 3: Deferrals + deliverable

- [x] T007 Append three `fab/backlog.md` items (matching existing style): `[clc4]` `hop rm` mandatory consent/`--yes` deferral; `[qner]` CLAUDE.md/AGENTS.md agent-entry files pointing at toolkit standards; `[armh]` `hop skill` subcommand adoption <!-- R6 R7 -->
- [x] T008 Write `fab/changes/260717-fcvp-toolkit-standards-conformance/conformance-report.md` — header + one section per standard (principles, help-dump, readme-extraction, skill) with PASS/gaps and dispositions <!-- R8 -->

### Phase 4: Verification

- [x] T009 Rebuild the binary and re-run the help-dump verification checklist (exit 0, valid JSON stdout only, stderr empty, no `captured_at`, filters intact); note the re-verification in the report <!-- R1 R8 -->
- [x] T010 Run the full Go suite (`cd src && go test ./...`) and confirm green <!-- R2 R4 -->

## Execution Order

- T001 blocks T002 (test asserts against the changed struct)
- T004 blocks T005 (tests exercise the new flag)
- T001–T006 precede T008 (report records the fixes) and T009/T010 (verification)
- T003, T007 are independent

## Acceptance

### Functional Completeness

- [x] A-001 R1: `hop help-dump` emits an envelope with exactly `{tool, version, schema_version, root}` — `captured_at` absent
- [x] A-002 R2: A help-dump test asserts `captured_at` is absent; existing envelope/filter assertions still pass
- [x] A-003 R3: README command-reference link is `https://shll.ai/hop/commands/`
- [x] A-004 R4: `hop rm` and `hop config rm` accept `--dry-run`; a dry-run resolves the target and writes nothing
- [x] A-005 R5: `docs/specs/cli-surface.md` documents `hop rm --dry-run`
- [x] A-006 R6: The conformance report records the ten-principle audit; `[clc4]` and `[qner]` deferrals referenced
- [x] A-007 R7: The `skill` report section reads "deferred, not yet adopted" and references `[armh]`; no `hop skill` code added
- [x] A-008 R8: `conformance-report.md` exists with header + four standard sections in enumeration order

### Behavioral Correctness

- [x] A-009 R4: `hop rm alpha --dry-run` leaves `hop.yaml` byte-for-byte unchanged and exits 0
- [x] A-010 R1: help-dump command tree still excludes `completion`/`help`/hidden nodes after the change (no command-tree regression)

### Scenario Coverage

- [x] A-011 R4: A test exercises the dry-run no-write path (both `<name>` and picker forms)
- [x] A-012 R1: The help-dump verification checklist is re-run post-build and noted in the report

### Edge Cases & Error Handling

- [x] A-013 R4: `hop rm <name> --dry-run` on a non-existent registry entry previews nothing to remove without erroring (forgiving no-op, exit 0), consistent with the live path

### Code Quality

- [x] A-014 Pattern consistency: New code follows the `config_rm.go` factory/`runRm`/`removeRepo` shared-body pattern and per-path stderr-prefix convention
- [x] A-015 No unnecessary duplication: The `--dry-run` path reuses `resolveOne`/`pickRepo`/existing resolution seams rather than reimplementing resolution
- [x] A-016 Composition over inheritance: `--dry-run` is threaded as a parameter through the existing `runRm`/`removeRepo` composition, not a new branch of duplicated logic
- [x] A-017 No magic strings: dry-run stderr wording is defined as named constants consistent with the file's existing message style

### Security

- [x] A-018 R4: The `--dry-run` path performs no subprocess exec beyond the existing `resolveOne`/`pickRepo` seams (exec.CommandContext with argument slices only — Constitution I); it only *skips* the YAML write

## Notes

- Check items as reviewed: `- [x]`
- All acceptance items must pass before hydrate
- The conformance report is the PR body at ship; commit hashes are added at ship time

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Drop `captured_at` entirely (not keep-and-empty) | Standard says "Do not emit `captured_at`"; envelope is `{tool, version, schema_version, root}`; no CI injects it | S:95 R:85 A:95 D:90 |
| 2 | Certain | Fix `--dry-run` on `hop rm`; defer mandatory `--yes`/consent | `--dry-run` is the small additive fix principle №5 names; a mandatory prompt would break `hop rm <name>`'s documented non-interactive contract (restructuring, per intake §3 defer) | S:80 R:80 A:85 D:80 |
| 3 | Certain | Command-reference URL is `https://shll.ai/hop/commands/` (drop `/tools/`) | readme-extraction rule 8 gives the canonical form `https://shll.ai/<tool>/commands/`; standards themselves render at `/shll/standards/…` (no `/tools/`) | S:85 R:90 A:85 D:85 |
| 4 | Certain | Defer CLAUDE.md/AGENTS.md authoring to backlog `[qner]` rather than fix | Principle №10 is a SHOULD; authoring agent-entry pointer files is documentation curation, not a mechanical flag/stream/error fix — outside intake §3's small-additive boundary | S:75 R:90 A:80 D:75 |
| 5 | Confident | dry-run wording: `would remove: <url>` + `dry-run: no changes written` to stderr | Matches the file's existing `removed:`/`wrote:` stderr voice and principle №2 (status → stderr); exact wording is low-blast-radius and easily revised | S:65 R:90 A:75 D:70 |

5 assumptions (4 certain, 1 confident, 0 tentative).
