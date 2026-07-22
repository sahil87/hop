# Intake: Rewrite Root Help Text for Readability

**Change**: 260722-3j8c-rewrite-root-help-text
**Created**: 2026-07-22

## Origin

Promptless dispatch (`/fab-proceed`-style create-intake subagent) from a live conversation in which the user reviewed and approved both the restructuring plan and the exact target text below.

> `hop -h` output (the `rootLong` const in `src/cmd/hop/root.go:10-71`) is hard to read: the 30-row cheat sheet mixes grammar examples with subcommand listings that cobra's auto-generated "Available Commands" section already duplicates a screen below; long right-hand descriptions wrap past 80 columns; Grammar and Getting started are dense wrapped prose; the Notes section has overlapping bullets.

Interaction mode: conversational — the plan (six numbered changes) and the full replacement text were drafted, reviewed, and explicitly approved by the user before this intake was created. No questions were asked at intake time (promptless dispatch); all decisions below trace to that approval.

## Why

1. **The pain point**: `hop -h` currently front-loads a 30-row "Cheat sheet" that mixes two different kinds of content — grammar *examples* (`hop <name> git pull`) and plain subcommand *listings* (`hop clone <url>`, `hop config where`, `hop -v`). The subcommand rows duplicate cobra's auto-generated "Available Commands" section rendered one screen below, so the same information appears twice with different formatting. Several right-hand descriptions run past 80 columns (e.g., the `hop <name>` row's description embeds a full backticked eval command), the Grammar and Getting started sections are dense wrapped prose, and the Notes section has six bullets with overlapping content (two separate shim/parent-shell bullets).

2. **The consequence if unfixed**: the help surface — the primary orientation point for both humans and agents (toolkit principle №3: "Help is a published contract") — buries the actual novel content (the selection-first grammar) inside redundant listings, and the wrapping/duplication makes the layered-help structure (summary → grammar → examples → commands) hard to scan. This also propagates verbatim into the published `help-dump` root-node `text` rendered on shll.ai.

3. **Why this approach**: cut the duplication (cobra already lists subcommands), keep only what `rootLong` uniquely provides (grammar, examples of the selection-first forms, setup, caveats), and compress every section to scannable, 80-column-safe lines. A concrete-name alternative for the examples (`hop myrepo` instead of `hop <name>`) was offered and not taken — `<name>`-style placeholders stay.

## What Changes

Single-const edit: replace the value of `rootLong` in `src/cmd/hop/root.go` (currently lines 10–71). No other code changes; no behavior changes. Change type: docs/UX-text-only change to the CLI help surface.

### 1. Cut subcommand rows from the cheat sheet

Remove the 13 subcommand rows (clone ×4, ls ×2, add, rm, shell-init, config ×3, update, and the `-h`/`-v` rows) — cobra's auto-generated "Available Commands" section covers them. Replace with a one-line pointer to the subcommand list below (see target text: "Management subcommands (clone, ls, add, rm, config, ...) are listed below — see `hop <command> -h` for each.").

### 2. Collapse near-duplicate grammar rows; retitle "Examples:"

Collapse the near-duplicate rows (pull/push/sync × name/group/--all; the three tool-form rows `git pull`/`code .`/`p`; the three cd variants `hop <name>`/`hop <name>/<wt>`/`hop <name> cd`) — the 30-row "Cheat sheet:" becomes a 9-row "Examples:" section.

### 3. Shorten descriptions

Every example description becomes one clause fitting 80 columns total per line.

### 4. Reformat Grammar and Getting started

Grammar becomes aligned per-term lines (`<selection>` / `[action]`). Getting started becomes exactly two one-line steps with NO parenthetical sub-lines — the user explicitly requested their removal as noise. (The from-source shim fallback previously living there moves into Notes — see #5.)

### 5. Consolidate Notes from 6 bullets to 4

Merge the two shim/parent-shell bullets into one that also carries the shim install fallback `(shll shell-setup, or eval "$(hop shell-init zsh)")` — this preserves the from-source setup info dropped from Getting started. The `sync` definition (auto-commit, `git pull --rebase`, `git push`) moves into its Examples row. Keep: the plural-selection restriction, the fzf-prefill note, and the config-location note.

### 6. Keep placeholder style

Examples keep `<name>`-style placeholders (a concrete-name alternative was offered; the user did not opt in).

### Exact agreed target text for `rootLong`

The user approved this text verbatim. Backtick-escaping is to be handled in the Go const as the current code does (string concatenation with `+ "`" + ...` segments — see the existing `rootLong` const for the pattern):

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

### Verification

- Run `go test ./cmd/hop` from `src/`. `src/cmd/hop/help_dump_test.go` (`TestHelpDumpRootTextUsesLong`) asserts the help-dump root text begins with `rootLong` — it is content-agnostic (references the const, not literal text), so no test changes are expected.
- Toolkit Standards clause (constitution § Toolkit Standards) checked: the rewrite aligns with shll principles №3 (help is layered — summary, grammar, examples, then cobra's command list) and №9 (bounded, high-signal output). No help-dump schema impact — only the root node's `text` content changes; the envelope and tree shape are untouched.

## Affected Memory

- `cli/subcommands`: (modify) references the `rootLong` "Usage table" structure — e.g., "The `rootLong` Usage table (`root.go`) carries `hop add <dir>` and `hop rm [<name>]` rows" and "`rootLong` providing the `Usage:` table and `Notes:` block" — the cheat-sheet/Usage-table rows are removed and the section is retitled "Examples:", so these descriptions go stale and need updating at hydrate.

(`architecture/package-layout` mentions `rootLong` only as "help text" carried by the root node's `text` — still accurate, no modify needed.)

## Impact

- **Code**: `src/cmd/hop/root.go` — the `rootLong` const only (single-const edit). No behavior change; cobra wiring, hint constants, and all subcommands untouched.
- **Tests**: `src/cmd/hop/help_dump_test.go` unaffected (content-agnostic assertion against the const). Full package test run expected green with zero test edits.
- **Published help surface**: `hop -h` output and the `help-dump` root-node `text` (rendered on shll.ai) change content; the help-dump JSON schema/envelope is unchanged.
- **Specs**: `docs/specs/cli-surface.md` (§ around lines 458–462) describes `rootLong` as providing "the `Usage:` table and `Notes:` block" — specs are human-curated and are not modified by this change; the memory update at hydrate covers the machine-maintained side.

## Open Questions

- None — the plan and the exact replacement text were reviewed and approved by the user before intake creation.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is a single-const edit: only `rootLong` in `src/cmd/hop/root.go` changes | User-approved plan states this explicitly as the only code touch | S:95 R:90 A:95 D:95 |
| 2 | Certain | The replacement text is the approved draft, adopted verbatim | User reviewed and approved the exact target text; reproduced in full above | S:100 R:85 A:95 D:100 |
| 3 | Certain | Cut all 13 subcommand rows; replace with the one-line pointer to cobra's list | Discussed and approved — cobra's Available Commands section already lists them | S:95 R:90 A:90 D:95 |
| 4 | Certain | Getting started is exactly two one-line steps with no parenthetical sub-lines | User explicitly requested removal of the parentheticals as noise | S:100 R:90 A:95 D:100 |
| 5 | Certain | Notes shrink to 4 bullets; merged shim bullet carries the install fallback `(shll shell-setup, or eval "$(hop shell-init zsh)")` | Approved plan item 5 — preserves the from-source setup info dropped from Getting started | S:95 R:85 A:90 D:95 |
| 6 | Confident | Examples keep `<name>`-style placeholders | Concrete-name alternative was offered and the user did not opt in; trivially reversible text tweak | S:70 R:90 A:75 D:70 |
| 7 | Certain | Backtick escaping uses Go string concatenation matching the existing `rootLong` const style | Existing code pattern in `root.go` answers this deterministically | S:85 R:95 A:100 D:95 |
| 8 | Certain | No test changes; verify with `go test ./cmd/hop` from `src/` | `TestHelpDumpRootTextUsesLong` verified content-agnostic (asserts prefix `rootLong+"\n\n"` against the const itself) | S:85 R:90 A:95 D:90 |
| 9 | Confident | Affected memory is limited to `cli/subcommands` (modify) | Grep of `docs/memory/` shows only its Usage-table/cheat-sheet descriptions go stale; `architecture/package-layout` stays accurate | S:70 R:85 A:80 D:75 |
| 10 | Confident | `docs/specs/cli-surface.md`'s `rootLong` structure description is NOT updated in this change | Specs are human-curated (specs index: "written and maintained by humans"); change scope was fixed by the user as the single-const edit | S:60 R:85 A:75 D:70 |

10 assumptions (7 certain, 3 confident, 0 tentative, 0 unresolved).
