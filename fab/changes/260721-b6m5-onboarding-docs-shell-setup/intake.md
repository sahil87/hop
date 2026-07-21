# Intake: Onboarding Docs Shell-Setup Alignment

**Change**: 260721-b6m5-onboarding-docs-shell-setup
**Created**: 2026-07-22

## Origin

Created via promptless dispatch (Create-Intake Procedure, `{questioning-mode} = promptless-defer`) from a synthesized description of a user discussion held 2026-07-21/22. The user explicitly approved a three-item scope ("1-3") and explicitly rejected two other ideas (a `hop setup`/`hop doctor` subcommand; Homebrew caveats and empty-state hints — deferred/out of scope).

> hop's onboarding surfaces contradict each other and bury the real first-run story. The canonical happy path after installing is exactly two commands: `shll shell-setup` (wires the shell shim into the rc file, idempotently) then `hop add -r ~/code` (walks the code dir, builds hop.yaml from git remotes — auto-creates the config, no separate init step). But `hop -h`'s "Getting started" block (src/cmd/hop/root.go:19-22) recommends the OLD manual path: `hop config init` → hand-edit the yaml → install shim via eval. This directly contradicts README.md:112 and docs/site/install.md:73, which both say "`hop add -r` writes by default and auto-creates the config — there's no separate init step."

Approved scope: (1) rewrite the `Getting started` block in `src/cmd/hop/root.go` to the two-command story; (2) fix the no-config error message in `src/internal/config/resolve.go` to lead with `hop add -r ~/code`; (3) restructure `docs/site/install.md` §2 around `shll shell-setup`, fix the stale `shll shell-install` name, and add copy-pasteable TL;DR quick starts to both `docs/site/install.md` and `README.md`.

## Why

1. **The pain point**: hop's onboarding surfaces disagree with each other. `hop -h`'s "Getting started" block (src/cmd/hop/root.go:19-22) teaches the old manual path — `hop config init` → hand-edit `hop.yaml` → install the shim via eval — while README.md ("First run", line ~112) and docs/site/install.md §3 (line ~73) both teach the current reality: `hop add -r` writes by default and auto-creates the config, so there is no separate init step. A new user reading `hop -h` is steered into hand-editing YAML that hop would have generated for them. Similarly, the no-config error in `src/internal/config/resolve.go:37` — the message a fresh-machine user sees at the exact moment they need bootstrap guidance — leads with `hop add <dir>` (single-dir form) and `hop config init` instead of the canonical bootstrap form `hop add -r ~/code`. And docs/site/install.md references `shll shell-install`, which is only a legacy alias — the canonical command is `shll shell-setup` (verified against the shll repo: its docs/site/install.md says "Still works under the legacy alias `shll shell-install` — same command, unchanged behavior").

2. **The consequence if unfixed**: onboarding friction and drift. First-run users follow the worst of the three documented paths (manual YAML editing), the highest-value error message points away from the two-command happy path, and hop's docs keep propagating a stale shll command name — the exact per-tool/central-doc drift that the install-composition standard (Policy B) exists to prevent.

3. **Why this approach**: the fix is content-only alignment of existing surfaces (help text, one error string, two docs files) onto the already-true two-command story: `shll shell-setup` → `hop add -r ~/code`. No new CLI surface is added — a `hop setup`/`hop doctor` subcommand was explicitly rejected per Constitution Principle VI (minimal surface area); the existing surfaces cover the need. The recommended install path (`curl -fsSL https://shll.ai/install | sh -s -- hop`) installs shll alongside hop, so `shll shell-setup` is safe to recommend as primary — while `eval "$(hop shell-init zsh)"` must survive as the documented fallback because from-source installs don't get shll (install-composition Policy A: hop must never assume a sibling tool is installed).

## What Changes

### 1. Rewrite the `Getting started` block in `src/cmd/hop/root.go`

Current text (root.go lines 19-22, inside the `rootLong` constant):

```
Getting started:
  1. Run `hop config init` to create a starter hop.yaml.
  2. Edit it to list your repos (each entry: name + git URL + parent dir).
  3. For interactive use, install the shim: eval "$(hop shell-init zsh)"
```

Replace with the two-command story:

- **Step 1 — wire the shim**: `shll shell-setup` (idempotently wires the shell shim into the rc file). Note `eval "$(hop shell-init zsh)"` as the alternative for from-source installs — hop must never assume shll is present (install-composition standard, Policy A).
- **Step 2 — bootstrap the config**: `hop add -r ~/code` (walks the code dir, builds hop.yaml from git remotes; auto-creates the config — no separate init step).

The `hop config init` + hand-edit-yaml story is demoted/removed from "Getting started". `hop config init` remains listed in the cheat sheet (root.go line 49, `hop config init  bootstrap a starter hop.yaml`) as the manual alternative — it is not being removed from the CLI or from the help entirely, only from the recommended first-run path.

Notes on blast radius inside root.go:

- The runtime shim hints (`bareNameHint` line 73, `cdHint` line 78, `toolFormHintFmt` line 84) keep pointing at `eval "$(hop shell-init zsh)"` — they are binary-emitted runtime hints and the binary cannot assume shll is installed (Policy A). They are NOT in scope.
- `hop help-dump` derives its root `text` field from `cmd.Long` (help_dump.go `nodeText`), and `src/cmd/hop/help_dump_test.go:166-170` (`TestHelpDumpRootTextUsesLong`) asserts `doc.Root.Text` begins with the `rootLong` constant — it references the constant, not a string literal, so rewriting `rootLong`'s content does not break it. The help-dump JSON content changes as a consequence; that is expected and conformant (the help-dump standard governs the contract shape, not the prose).

### 2. Fix the no-config error message in `src/internal/config/resolve.go`

Current message (resolve.go:37, in `Resolve()`):

```go
return "", fmt.Errorf("hop: no hop.yaml found at %s. Run 'hop add <dir>' to register a repo (creates the config), or 'hop config init' for a starter.", p)
```

Rewrite it to lead with the bootstrap form `hop add -r ~/code` (recursive walk — the canonical fresh-machine command) rather than the current `hop add <dir>` / `hop config init` wording. This message fires at the exact moment a fresh-machine user needs it (every read-command on a missing config: `hop`, `hop ls`, `hop <name> where`, `hop config print`, `hop add -p` dry-runs — see docs/memory/cli/subcommands.md § auto-init-on-write). `hop config init` may be retained as a trailing alternative in the message; the lead recommendation is what changes. Keep the `hop:` prefix and the `no hop.yaml found at <path>` shape (tests and memory match on that substring).

**Tests asserting the current string** (must be updated to the new spec text per the Test Integrity constraint — tests conform to spec, spec is source of truth):

- `src/internal/config/resolve_test.go:53` — asserts the full exact message.
- `src/cmd/hop/config_test.go:1024-1034` — asserts `no hop.yaml found` and `'hop config init' for a starter` substrings (via `hop config print` error propagation).
- `src/cmd/hop/config_test.go:170` — asserts `no hop.yaml found at <path>` substring (unaffected if that shape is kept).
- `src/cmd/hop/config_add_test.go:54` — asserts the *absence* of `Run 'hop config init' first` (the old rm-style gate) — unaffected, but verify.

**Out of scope (adjacent)**: backlog item `[d3wq]` — aligning `hop rm`'s separate missing-config wording (`config_rm.go`) with `Resolve()`'s message — remains a separate backlog item; this change does not touch `config_rm.go`.

### 3. Restructure `docs/site/install.md` §2 + add TL;DR quick starts (install.md + README.md)

Three sub-parts:

**(a) Make `shll shell-setup` the recommended wiring path in install.md §2 ("Wire the shell shim").** Currently §2 leads with the manual rc-line instructions (`eval "$(hop shell-init zsh)"` / `bash`) and mentions `shll shell-install` only in the trailing "One-shot wiring for multiple shll tools" subsection (install.md:62). Invert the order: `shll shell-setup` is the recommended path (it wires every installed shll tool's shell integration and completions into the rc file in a single idempotent command); the manual `eval "$(hop shell-init zsh)"` rc-line instructions are demoted to the from-source-install fallback (from-source installs don't get shll — Policy A). The explanatory content of §2 (why the shim exists, how dispatch works, the three things `shell-init` prints) is preserved — only the recommended wiring mechanics are restructured.

**(b) Fix the stale command name.** `shll shell-install` (install.md:62, and the same stale reference in README.md:101) is only a legacy alias; the canonical command is `shll shell-setup` (verified against the shll repo: its docs/site/install.md says "Still works under the legacy alias `shll shell-install` — same command, unchanged behavior"). Both references are updated to `shll shell-setup`.

**(c) Add a copy-pasteable 3-line TL;DR quick start** at the top of `docs/site/install.md` AND in `README.md`:

```sh
curl -fsSL https://shll.ai/install | sh -s -- hop   # install hop (+ shll) via Homebrew
shll shell-setup                                    # wire the shell shim into your rc file
hop add -r ~/code                                   # walk your code dir, build hop.yaml from git remotes
```

(Comments illustrative — exact phrasing decided at apply.) In README.md the TL;DR extends the existing `## Install` section (which already carries the curl line) rather than adding a new top-level section.

**Constraints honored throughout**:

- **install-composition Policy B**: install documentation is centralized on shll.ai — per-tool docs must not carry per-formula `brew install` lines. The existing install.md curl-bootstrap section already conforms; no brew lines are added.
- **Deliberate judgment call from the discussion**: hop's docs say `shll shell-setup` plain and link to shll.ai for the `--trust-tap` variant — do NOT duplicate shll's tap-trust matrix in hop docs (that duplication is the drift Policy B exists to prevent).
- **Toolkit Standards (Constitution § Toolkit Standards)**: changes to CLI help output, README.md, or docs/site/ MUST be checked against `shll standards` — the relevant standards are `help-dump` (machine-readable help contract), `readme-extraction` (README + docs/site structure), and `install-composition` (Policies A and B). Verified present via `shll standards` enumeration; conformance check is an apply/review obligation.

### Explicitly rejected / out of scope

- A new `hop setup` or `hop doctor` subcommand — rejected per Constitution Principle VI (minimal surface area); existing surfaces cover the need.
- Homebrew tap formula caveats printing post-install next steps (discussion "idea 4") — deferred to a separate change (touches the homebrew-tap repo / release pipeline).
- Empty-config empty-state hints on `hop ls` / bare `hop` (discussion "idea 5") — not in this change.
- `config_rm.go` wording alignment — backlog `[d3wq]`, separate.
- Binary runtime shim hints (`bareNameHint`/`cdHint`/`toolFormHintFmt`) — unchanged (Policy A).

## Affected Memory

- `config/search-order`: (modify) quotes the `Resolve()` no-config error message verbatim (§ the not-found message, lines ~36-38) and explains its two-bootstrap-path pointing — must be updated to the new wording (lead with `hop add -r ~/code`).
- `cli/subcommands`: (modify) quotes the `Resolve()` message verbatim in the `hop config print` row and describes the read-command error contract (§ auto-init-on-write implications) — update the quoted string; no behavioral contract changes.

(No memory file governs README.md/docs/site prose or the root help "Getting started" block content; docs-only edits there need no further memory updates.)

## Impact

- **Code (content-only string changes, no logic)**: `src/cmd/hop/root.go` (the `rootLong` constant's "Getting started" block), `src/internal/config/resolve.go` (the `Resolve()` not-found message).
- **Tests**: `src/internal/config/resolve_test.go` (exact-string assertion at line 53), `src/cmd/hop/config_test.go` (substring assertions at lines 170, 1024-1034) — updated to the new spec text per the Test Integrity constraint. `src/cmd/hop/help_dump_test.go` asserts against the `rootLong` constant, not a literal — no update expected.
- **Docs**: `docs/site/install.md` (TL;DR at top; §2 restructure; stale `shll shell-install` → `shll shell-setup`), `README.md` (TL;DR in `## Install`; stale `shll shell-install` reference at line ~101).
- **Downstream artifacts**: `hop help-dump` JSON root text changes as a consequence of the `rootLong` rewrite (contract shape unchanged); shll.ai renders the help reference via its pull model — no push step in this change.
- **Standards conformance**: `help-dump`, `readme-extraction`, `install-composition` (`shll standards <name>`) must be checked during apply/review per Constitution § Toolkit Standards.
- **Repo layout note**: the Go module lives under `src/` (`src/cmd`, `src/internal`); run tests from `src/` (see CI: gofmt/vet/test from `src/`). Relevant test scope: `go test ./cmd/hop ./internal/config` from `src/`.

## Open Questions

- None — all material decisions were made and explicitly approved in the source discussion; remaining latitude (exact prose wording) is recorded as graded assumptions below.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Canonical first-run story is exactly two commands — `shll shell-setup` then `hop add -r ~/code` — and root.go's "Getting started" block is rewritten to it | Explicitly stated and approved in discussion ("user explicitly approved 1-3"); matches README/install.md's existing auto-create story | S:95 R:70 A:90 D:95 |
| 2 | Certain | `eval "$(hop shell-init zsh)"` survives as the documented from-source fallback everywhere `shll shell-setup` becomes primary | install-composition Policy A (hop must never assume shll is present) — constitution-bound standard, explicit in discussion | S:95 R:80 A:95 D:95 |
| 3 | Certain | TL;DR quick-start content is the 3-line sequence: the shll.ai curl install one-liner (piped to `sh -s -- hop`, as quoted in What Changes §3c) → `shll shell-setup` → `hop add -r ~/code` | Specified verbatim in discussion | S:95 R:85 A:90 D:95 |
| 4 | Certain | hop docs say `shll shell-setup` plain and link shll.ai for the `--trust-tap` variant — shll's tap-trust matrix is NOT duplicated in hop docs | Deliberate judgment call recorded in discussion; Policy B anti-drift rationale | S:90 R:85 A:85 D:90 |
| 5 | Certain | No per-formula `brew install` lines are added to hop docs | install-composition Policy B — install docs centralized on shll.ai; existing curl section already conforms | S:90 R:90 A:90 D:95 |
| 6 | Certain | No new subcommand (`hop setup`/`hop doctor` rejected); ideas 4 (tap caveats) and 5 (empty-state hints) stay out of scope | Explicit rejections in discussion; Constitution Principle VI | S:95 R:75 A:90 D:95 |
| 7 | Certain | Tests asserting the current strings (`resolve_test.go:53`, `config_test.go:170,1024-1034`) are updated to the new spec text | Constitution § Test Integrity: tests conform to spec, spec is source of truth | S:85 R:90 A:95 D:90 |
| 8 | Certain | Binary runtime shim hints (`bareNameHint`, `cdHint`, `toolFormHintFmt`) keep `eval "$(hop shell-init zsh)"` — untouched by this change | Policy A: binary-emitted hints cannot assume shll is installed; hints not named in approved scope | S:75 R:85 A:90 D:85 |
| 9 | Confident | Exact prose of the rewritten Getting-started block and the new `Resolve()` message is drafted at apply within the stated direction (shell-setup primary + shell-init fallback; error leads with `hop add -r ~/code`) | Direction fully specified, wording latitude is low-risk and easily revised | S:80 R:85 A:75 D:70 |
| 10 | Confident | New `Resolve()` message keeps the `hop: no hop.yaml found at <path>` lead shape and retains `hop config init` as a trailing alternative after the `hop add -r ~/code` lead | Tests/memory match on the shape; discussion demotes rather than erases config init ("may remain mentioned as a manual alternative elsewhere") | S:70 R:88 A:75 D:60 |
| 11 | Confident | `hop config init` stays listed in root.go's cheat sheet (demoted from Getting started, not deleted from help) | Discussion: "config init may remain mentioned as a manual alternative elsewhere, e.g. the cheat sheet" | S:70 R:90 A:75 D:65 |
| 12 | Confident | README.md:101's `shll shell-install` reference is also updated to `shll shell-setup` (discussion named only install.md's instance) | Same staleness, same canonical-name intent; leaving it would contradict the change's own fix | S:60 R:90 A:85 D:75 |
| 13 | Confident | README TL;DR lands inside the existing `## Install` section (extending the curl line into the 3-line quick start) rather than as a new top-level section | README already opens with `## Install` carrying line 1 of 3; readme-extraction standard favors existing structure | S:65 R:90 A:70 D:60 |
| 14 | Confident | Change type is `docs` (content-only help/error strings + docs; no behavior change) | Discussion: "likely docs/polish-leaning"; no logic changes anywhere in scope | S:65 R:95 A:80 D:70 |
| 15 | Confident | Backlog `[d3wq]` (`config_rm.go` wording alignment) remains out of scope | Adjacent but distinct surface; not in the approved 1-3 scope | S:55 R:85 A:70 D:65 |

15 assumptions (8 certain, 7 confident, 0 tentative, 0 unresolved).
