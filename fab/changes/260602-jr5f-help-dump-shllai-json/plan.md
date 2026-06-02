# Plan: Help-dump CLI tree → help/hop.json → PR into shll.ai

**Change**: 260602-jr5f-help-dump-shllai-json
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### Producer: Hidden `help-dump` subcommand

#### R1: Hidden cobra subcommand emits the help tree as JSON to stdout
hop SHALL expose a hidden cobra subcommand `hop help-dump` that prints a JSON document describing the CLI command tree to stdout. The command MUST be marked `Hidden: true` so it does not appear in user-facing help and so it self-filters out of its own output.

- **GIVEN** a built `hop` binary
- **WHEN** `hop help-dump` is invoked
- **THEN** a JSON document is written to stdout and the process exits 0
- **AND** the subcommand does not appear in `hop --help` output

#### R2: Document shape and field order
The emitted document SHALL be a `Doc` object with fields, in order: `tool` (string `"hop"`), `version` (string), `captured_at` (string), `schema_version` (int `1`), `root` (Node). It MUST marshal with 2-space indentation.

- **GIVEN** the producer runs
- **WHEN** the document is marshaled
- **THEN** the JSON has top-level keys `tool`, `version`, `captured_at`, `schema_version`, `root`
- **AND** `tool == "hop"` and `schema_version == 1`
- **AND** indentation is 2 spaces

#### R3: Node shape derived from structured cobra fields
Each `Node` SHALL be `{name, path, short, usage, text, commands}`, populated from structured cobra fields (NOT regex-parsing `-h` output): `name = cmd.Name()`, `path = cmd.CommandPath()`, `short = cmd.Short`, `usage = cmd.UseLine()`, `commands` = recursive children. The `commands` field MUST serialize as `[]` (never `null`) for leaf nodes — the slice is initialized non-nil.

- **GIVEN** a cobra command node
- **WHEN** its Node is built
- **THEN** name/path/short/usage are read from the corresponding cobra accessors
- **AND** a leaf node serializes `commands` as `[]`, and the JSON bytes contain no `"commands":null`

#### R4: Uniform `text` field rule
For every node (root and subcommands alike), `text` SHALL equal `cmd.Long + "\n\n" + cmd.UsageString()` when `cmd.Long != ""`, else `cmd.UsageString()` alone.

- **GIVEN** a command with a non-empty `Long`
- **WHEN** its Node is built
- **THEN** `text` begins with the `Long` narrative followed by a blank line and the usage block
- **GIVEN** a command with an empty `Long`
- **WHEN** its Node is built
- **THEN** `text` equals `cmd.UsageString()` alone

#### R5: Child filtering during the walk
The recursive walk SHALL skip any child whose `Name()` is `"completion"` or `"help"`, any child with `Hidden == true` (which drops `help-dump` itself), and defensively any child where `IsAdditionalHelpTopicCommand()` is true.

- **GIVEN** the root command tree (with auto-generated `completion`/`help` and the hidden `help-dump`)
- **WHEN** the walk builds `root.commands`
- **THEN** `clone` is present and `completion`, `help`, and `help-dump` are absent

#### R6: Version sourced from the live root, never hardcoded
`version` SHALL be read from `rootCmd.Version` (which is `main.version`, ldflag-injected), never hardcoded. The producer MUST build the document from the SAME live root instance that has all subcommands wired and `Version` set (reachable via `cmd.Root()` inside RunE).

- **GIVEN** an unstamped local build where `rootCmd.Version == "dev"`
- **WHEN** `hop help-dump` runs
- **THEN** the document's `version` is `"dev"`
- **GIVEN** the producer walks the tree
- **THEN** the root node and all wired subcommands are present (built from the live root)

#### R7: `captured_at` left empty by the producer (CI injects it)
The producer SHALL leave `captured_at` as the empty string `""`. It MUST NOT call `time.Now()`, keeping the dump deterministic and testable. CI injects a date-floored UTC value.

- **GIVEN** the producer runs
- **WHEN** the document is marshaled
- **THEN** `captured_at == ""`

### CI: generate, validate, PR into shll.ai

#### R8: Release workflow publishes the help reference via auto-merge PR
`.github/workflows/release.yml` SHALL, after the cross-compile build, run the linux-amd64 binary's `help-dump`, inject a date-floored UTC `captured_at` via `jq`, validate the JSON, then clone `sahil87/shll.ai`, write `help/hop.json` on a fresh branch `hop-help-${version}`, commit, push, open a PR, and enable auto-merge (squash). It MUST use `SHLLAI_TOKEN` and never push directly to `shll.ai` main.

- **GIVEN** a `v*` tag release runs the workflow
- **WHEN** the publish step executes
- **THEN** `hop help-dump` output is captured, `captured_at` is set via `jq` to `$(date -u +%Y-%m-%dT00:00:00Z)`
- **AND** the JSON is validated with `jq -e '.tool=="hop" and .schema_version==1 and (.root|type=="object") and (.captured_at|test("Z$"))'`
- **AND** a PR is opened against `sahil87/shll.ai` and set to auto-merge (squash), authenticated with `SHLLAI_TOKEN`

### Local ergonomics (optional)

#### R9: Local `just help-dump` recipe
hop MAY provide a `just help-dump` recipe delegating to `scripts/help-dump.sh` that builds/runs the binary and pretty-prints the JSON, consistent with the Thin Justfile principle (one-line recipe, logic in the script).

- **GIVEN** a developer working locally
- **WHEN** they run `just help-dump`
- **THEN** the script builds hop and prints the pretty JSON help tree to stdout

### Non-Goals

- Synthesizing `extractDashR` / `-R` / tool-form invocations as separate nodes — they live outside the cobra tree; `rootLong` already documents them, so they appear in the root node's `text` (intake assumption 7).
- Site-side consumer (Astro loader + reference UI) in `shll.ai` — tracked separately.
- Idempotency hardening for re-running the same tag (plan-level detail; the branch name is tag-scoped which is sufficient).

### Design Decisions

1. **Hidden subcommand over root flag / standalone tool**: reuses the already-constructed `rootCmd` so it sees the real tree and real `rootCmd.Version`, matches cobra idioms in the codebase, and being `Hidden` keeps the user-facing surface unchanged (satisfies Minimal Surface Area). — *Why*: real version + real tree without extra wiring. — *Rejected*: hidden root flag (less idiomatic), standalone `cmd/help-dump` tool (cannot easily reach the wired root + version).
2. **Producer leaves `captured_at` empty; CI stamps it**: keeps the Go dump deterministic/testable and matches the `wt.json` `00:00:00Z` convention. — *Why*: determinism + single source of date-flooring. — *Rejected*: `time.Now()` in producer (non-deterministic, untestable).
3. **Build doc from `cmd.Root()` inside RunE**: guarantees the same live root with `Version` and all subcommands wired, rather than a fresh bare command.

## Tasks

### Phase 2: Core Implementation

- [x] T001 Create `src/cmd/hop/help_dump.go`: define `Doc`/`Node` structs with the exact json tags and field order, `buildNode`/`shouldSkipChild` walk helpers, `buildHelpDoc(root *cobra.Command) Doc`, and `newHelpDumpCmd()` (Hidden cobra command whose RunE marshals `buildHelpDoc(cmd.Root())` with 2-space indent to `cmd.OutOrStdout()`). <!-- R1 R2 R3 R4 R5 R6 R7 -->
- [x] T002 Register the hidden subcommand in `src/cmd/hop/root.go` `newRootCmd()` via `cmd.AddCommand(newHelpDumpCmd())`. <!-- R1 R6 -->

### Phase 3: Integration & Edge Cases

- [x] T003 Add `src/cmd/hop/help_dump_test.go` (matching `runArgs`/`runCmd` style): assert (a) doc marshals with `tool=="hop"`, `schema_version==1`, `version` reflecting `rootCmd.Version`, `captured_at==""`; (b) walk excludes `completion`/`help`/`help-dump` and includes `clone`; (c) leaf `commands` serialize as `[]` (no `"commands":null` in bytes); (d) a known subcommand node has expected name/path/short and `text` contains its usage. <!-- R1 R2 R3 R4 R5 R6 R7 -->
- [x] T004 Append a "Publish help reference to shll.ai" step to `.github/workflows/release.yml` after the Cross-compile step, per intake section C (dump → jq inject captured_at → jq -e validate → clone shll.ai → branch `hop-help-${version}` → copy to `help/hop.json` → commit → push → `gh pr create` → `gh pr merge --auto --squash`), using `env: SHLLAI_TOKEN` and `GH_TOKEN="$SHLLAI_TOKEN"`. <!-- R8 -->

### Phase 4: Polish (optional)

- [x] T005 [P] Add `scripts/help-dump.sh` (build hop, run `help-dump`, pretty-print via `jq`) and a one-line `just help-dump` recipe delegating to it, per the Thin Justfile principle. <!-- R9 -->

## Execution Order

- T001 blocks T002 and T003 (they reference the new symbols / wired command)
- T004 is independent of the Go tasks but depends on `help-dump` existing conceptually (T001/T002)
- T005 is independent and optional

## Acceptance

### Functional Completeness

- [x] A-001 R1: `hop help-dump` is a hidden cobra subcommand registered in `newRootCmd()`, prints JSON to stdout, exits 0, and is absent from `hop --help`.
- [x] A-002 R2: The document has top-level keys `tool`,`version`,`captured_at`,`schema_version`,`root` in order, `tool=="hop"`, `schema_version==1`, marshaled with 2-space indentation.
- [x] A-003 R3: Each Node is `{name,path,short,usage,text,commands}` from cobra accessors; leaf `commands` serializes as `[]` and the JSON contains no `"commands":null`.
- [x] A-004 R4: `text == Long+"\n\n"+UsageString()` when `Long` is non-empty, else `UsageString()` alone, uniformly across nodes.
- [x] A-005 R5: The walk excludes `completion`, `help`, and hidden commands (including `help-dump`); `clone` is present.
- [x] A-006 R6: `version` equals `rootCmd.Version` (not hardcoded); the document is built from the live root with all subcommands wired.
- [x] A-007 R7: `captured_at == ""` from the producer; no `time.Now()` call in the producer.
- [x] A-008 R8: `release.yml` has a publish step that dumps, injects `captured_at` via `jq`, validates via `jq -e`, and opens an auto-merge squash PR into `sahil87/shll.ai` using `SHLLAI_TOKEN`, never a direct push to main.
- [x] A-009 R9: A `just help-dump` recipe delegates to `scripts/help-dump.sh` which builds/runs hop and pretty-prints the JSON (optional — mark N/A if skipped).

### Scenario Coverage

- [x] A-010 R5: A test asserts `clone` present and `completion`/`help`/`help-dump` absent in `root.commands`.
- [x] A-011 R3: A test asserts the marshaled bytes contain no `"commands":null`.
- [x] A-012 R6: A test asserts `version` reflects `rootCmd.Version`.

### Edge Cases & Error Handling

- [x] A-013 R7: A test asserts `captured_at == ""` in producer output (determinism).

### Code Quality

- [x] A-014 Pattern consistency: New code follows the `newXxxCmd()` factory style, error handling, and file naming of surrounding `src/cmd/hop/` files.
- [x] A-015 No unnecessary duplication: Reuses cobra accessors and existing test helpers (`runArgs`/`runCmd`) rather than reimplementing.
- [x] A-016 Readability: Producer functions are focused and small (no god functions >50 lines), no magic strings without named constants where reasonable.

### Security

- [x] A-017 R8: The producer is pure Go (`encoding/json` + cobra), no subprocess; the CI step shells out to `git`/`gh`/`jq` only with the trusted tag-derived version (no untrusted user input interpolated into shell), per Security First.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Producer struct/field design (Doc/Node json tags + order) and walk filter exactly mirror the frozen intake contract and verified `wt.json` reference | Frozen by intake assumptions 1-3; verified against live reference | S:98 R:90 A:95 D:98 |
| 2 | Certain | Build doc from `cmd.Root()` inside RunE to reach the wired root + Version | Intake design decision 6 / fact 6; cobra `cmd.Root()` returns the executing root which has `Version` set in `main()` | S:95 R:80 A:90 D:90 |
| 3 | Certain | CI step appended after the Cross-compile step in `release.yml`, using `dist/hop-linux-amd64/hop` and `steps.version.outputs.version` | Intake section C + fact 7; matches the existing homebrew step's version usage | S:95 R:80 A:90 D:85 |
| 4 | Confident | Include the optional `scripts/help-dump.sh` + `just help-dump` recipe (low-effort, matches Thin Justfile principle and existing script/recipe patterns) | Intake section D marks it optional; existing `build.sh`/`install.sh` + one-line recipes make it consistent and cheap | S:75 R:90 A:85 D:80 |
| 5 | Confident | `help-dump.sh` pretty-prints via `jq` (already a CI dependency and common dev tool) and builds via `scripts/build.sh` then runs `bin/hop help-dump` | No explicit spec for the script body; mirrors existing build flow and keeps it simple | S:70 R:90 A:80 D:75 |

5 assumptions (3 certain, 2 confident, 0 tentative, 0 unresolved).
