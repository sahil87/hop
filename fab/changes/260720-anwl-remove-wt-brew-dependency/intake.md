# Intake: Remove wt Homebrew Dependency

**Change**: 260720-anwl-remove-wt-brew-dependency
**Created**: 2026-07-20

## Origin

One-shot `/fab-new` invocation with a fully-specified description:

> Remove the wt Homebrew dependency from hop and make the wt-missing hint actionable. Two changes: (1) delete the line `depends_on "sahil87/tap/wt"` from `.github/formula-template.rb` (this template regenerates the tap formula on release); (2) change the `wtMissingHint` const in `src/cmd/hop/resolve.go` from `"hop: wt: not found on PATH."` to `"hop: wt is not installed. Install it: brew install sahil87/tap/wt"`. Audit context: all wt call sites already route through internal/proc with ErrNotFound handling and are test-covered (ls_test.go and open_test.go reference the const or a substring, so no test changes are expected — verify). Rationale: toolkit-wide decision to remove inter-tool brew dependencies; wt becomes a runtime-probed optional tool, so the hint must tell users how to install it.

Intake-time verification (grep audit) confirmed both edit targets exist and corrected one detail of the audit context: the tests referencing the const are `ls_test.go` (substring via `strings.Contains`, lines 187 and 362) and `resolve_test.go` (verbatim equality, line 472) — **not** `open_test.go`, which carries no reference. All references use the `wtMissingHint` symbol, never a hardcoded literal, so the "no test changes" expectation holds.

## Why

1. **Problem**: The published Homebrew formula for hop declares `depends_on "sahil87/tap/wt"`, forcing every `brew install sahil87/tap/hop` to also install wt. This couples the toolkit's tools at the package-manager level. A toolkit-wide decision was made to remove inter-tool brew dependencies — each tool installs standalone, and cross-tool integrations are probed at runtime.
2. **Consequence of not fixing**: hop keeps hard-requiring wt at install time, contradicting the toolkit standard; users who never use worktree features pay the wt install cost, and the toolkit's tools cannot be versioned/installed independently.
3. **Why this approach**: hop already treats wt as a runtime-probed tool — every wt call site routes through `internal/proc`, which returns `proc.ErrNotFound` when the binary is absent, and every caller fails fast with the shared `wtMissingHint` constant. The only gap once the brew dependency is removed is discoverability: the current hint (`hop: wt: not found on PATH.`) states the problem but not the remedy. With wt no longer auto-installed, the hint must tell the user how to install it.

## What Changes

### 1. Formula template: drop the wt dependency

Delete this single line from `.github/formula-template.rb`:

```ruby
  depends_on "sahil87/tap/wt"
```

The template is the source that regenerates `Formula/hop.rb` in `sahil87/homebrew-tap` on each tagged release. No tap-side manual edit is in scope — the already-published formula keeps the dependency until the next release regenerates it from this template.

### 2. wt-missing hint: make it actionable

In `src/cmd/hop/resolve.go` (currently line 21), change the constant value:

```go
// before
const wtMissingHint = "hop: wt: not found on PATH."

// after
const wtMissingHint = "hop: wt is not installed. Install it: brew install sahil87/tap/wt"
```

The constant's doc comment (lines 17–20) describes it as "the exact stderr line printed when `wt` is needed but not [installed]" — update the comment only if its wording refers to the old message text; the exit-code/behavior contract is unchanged.

The constant is shared by three surfaces (all unchanged in behavior — exit 1, message to stderr):
- `open.go:33` — `hop <name> open` when wt is missing
- `resolve.go:185` — the `<name>/<wt>` worktree-resolution branch
- `ls.go:111` and `ls.go:209` — `hop ls --trees` (text and `--json` modes) fail-fast on first `proc.ErrNotFound`

### 3. Tests: verify no changes needed

All test references use the const symbol, so the value change propagates automatically:
- `src/cmd/hop/ls_test.go:187,362` — `strings.Contains(stderr.String(), wtMissingHint)`
- `src/cmd/hop/resolve_test.go:472` — `withCode.msg != wtMissingHint` (verbatim equality)

The apply task is to run the relevant tests (`go test ./cmd/hop/ -run 'TestLs|TestResolve|TestOpen'` from `src/`, or the full package) and confirm green with zero test-file edits. If any test unexpectedly hardcodes the old literal, fix the test to use the const (spec/implementation is authoritative per the constitution's Test Integrity rule).

## Affected Memory

- `cli/subcommands`: (modify) row for the `wt` external-tool failure message (line ~190) records the old wording AND claims "Mitigated: wt is declared as a Homebrew formula dependency" — both become stale; the mitigation flips to "runtime-probed optional tool; hint carries the install command"
- `cli/match-resolution`: (modify) exit-code table (line ~43) records the old `hop: wt: not found on PATH.` wording verbatim
- `build/release-pipeline`: (modify) formula-template section (line ~96) documents the `depends_on "sahil87/tap/wt"` line and its rationale — entry becomes a removal note

Specs also reference the old wording/dependency (`docs/specs/cli-surface.md` lines 22, 64, 447; `docs/specs/architecture.md`), but specs are human-curated pre-implementation artifacts — flag for the user rather than auto-editing during hydrate.

## Impact

- **Code**: 1-line delete in `.github/formula-template.rb`; 1 const value change (plus possibly its doc comment) in `src/cmd/hop/resolve.go`. No control-flow, exit-code, or API changes.
- **Tests**: none expected (verify-only task).
- **Packaging**: next tagged release publishes a `Formula/hop.rb` without the wt dependency; existing brew installs are unaffected until they upgrade.
- **Users**: `brew install sahil87/tap/hop` no longer pulls wt; users hitting a wt-requiring command (`hop <name> open`, `hop <name>/<wt>`, `hop ls --trees`) without wt installed now get an actionable install command on stderr.
- **Memory/docs**: 3 memory files to update at hydrate; 2 spec files flagged for human follow-up.

## Open Questions

- (none)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Delete `depends_on "sahil87/tap/wt"` from `.github/formula-template.rb`; no tap-side manual edit | Explicit instruction; template regenerates the tap formula on release, verified present at intake | S:95 R:90 A:95 D:95 |
| 2 | Certain | New hint value is exactly `hop: wt is not installed. Install it: brew install sahil87/tap/wt` | Exact string supplied verbatim in the description | S:95 R:95 A:95 D:95 |
| 3 | Certain | No test-file changes — all references use the `wtMissingHint` symbol | Verified by grep at intake: ls_test.go uses Contains(const), resolve_test.go uses verbatim equality with const; no hardcoded literals; the description's mention of open_test.go was inaccurate (it has no reference) but the conclusion holds | S:90 R:95 A:95 D:90 |
| 4 | Confident | Update the const's doc comment only if it restates the old wording; keep the exit-1/stderr contract description | Comment currently describes behavior, not the literal text; low-stakes and easily adjusted at apply | S:60 R:90 A:80 D:75 |
| 5 | Confident | Spec files (`cli-surface.md`, `architecture.md`) referencing the old wording are flagged for the user, not auto-edited | Specs are human-curated per docs/specs/index.md ownership note; hydrate owns memory only | S:55 R:85 A:75 D:70 |

5 assumptions (3 certain, 2 confident, 0 tentative, 0 unresolved).
