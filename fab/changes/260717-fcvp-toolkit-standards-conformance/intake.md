# Intake: Toolkit Standards Conformance

**Change**: 260717-fcvp-toolkit-standards-conformance
**Created**: 2026-07-18

## Origin

<!-- One-shot /fab-new invocation with a fully-specified task. No prior conversation. -->

> Task: Bring this repo and its tool into conformance with the sahil87 toolkit standards.
>
> Precondition: `shll standards` runs on this machine (if the subcommand is missing, run `shll update`; if it still fails, stop and report — do not proceed from memory or the website). This repo's constitution carries the Toolkit Standards article; this task is the conformance work it mandates.
>
> 1. Enumerate at runtime: run `shll standards`, then `shll standards <name>` for every listed entry. The list is authoritative — do not assume which standards exist or what they require.
> 2. Audit this repo against each standard. For mechanical contracts (machine help output, README/docs-site structure), execute the standard's own verification checklist verbatim. For the principles, assess each numbered principle against the tool's actual behavior — prompts and TTY handling, stdout/stderr separation, --json/--dry-run/--yes coverage, exit codes and error wording, idempotency, output volume.
> 3. Fix what is proportionate here: all mechanical-contract violations, and principle gaps that are small and additive (a missing flag, a misrouted stream, an unhelpful error). Larger gaps that would restructure the tool are NOT for this change — record each as a draft change or issue per this repo's convention and reference it.
> 4. Deliverable: one fab change whose PR body contains a conformance report — one section per standard with PASS or the gaps found, each gap dispositioned as fixed here (with the commit) or deferred to <ref>. Include the shll version audited against (`shll version`'s shll row), since standards are versioned with the shll release. Tests green; if the command tree changed, re-verify the machine-help contract afterward.
>
> Note on the "skill" standard specifically: if this repo has not yet implemented a `<tool> skill` subcommand, that is a known, deferred gap (per the toolkit's phased per-repo adoption — no seven-repo flag-day) — report it as "deferred, not yet adopted" rather than treating it as an in-scope fix for this change.

**Intake-time verification performed** (all commands run live on this machine on 2026-07-18):

- Precondition satisfied: `shll standards` runs and lists **four** standards: `principles` (foundation), `help-dump` (binary), `readme-extraction` (repo), `skill` (binary+repo). All four bodies were read via `shll standards <name>`.
- `shll version` reports `shll v0.0.23` (and `hop v0.1.18`).
- The Toolkit Standards constitution article is **not yet on main** — it lives on the in-flight branch `260717-zono-bind-constitution-toolkit-standards` (constitution v1.2.0 amendment, PR already opened per that change's history). The article text: standards enumerated by `shll standards` bind this repo without further constitution amendment.

## Why

The sahil87 toolkit's tools (`shll`, `hop`, `wt`, `tu`, `idea`, `run-kit`, `fab`) are operated at least as often by AI agents as by humans. The toolkit publishes versioned standards — ten CLI principles plus three mechanical contracts (help-dump, readme-extraction, skill) — precisely because an agent cannot squint at ambiguous output, answer a surprise prompt, or guess what a tool meant. The constitution amendment in flight (`260717-zono-bind-constitution-toolkit-standards`, constitution v1.2.0) binds hop to these standards; this change is the conformance work that article mandates.

If we don't audit: conformance drift accumulates silently — a prompt that hangs a CI run, a misrouted stream that breaks a parser, a README structure change that renders broken on shll.ai — and each violation is discovered by an agent failing in the field rather than by a checklist in a PR.

Why an audit-and-fix change rather than piecemeal fixes: the standards define their own verification checklists ("Verifying conformance" sections), so one systematic pass per standard, with every gap explicitly dispositioned (fixed here or deferred to a reference), produces an auditable record — the PR body conformance report — that future changes can diff against.

## What Changes

### 1. Runtime enumeration (audit input)

At apply time, re-run `shll standards` and `shll standards <name>` for every listed entry — the runtime list is authoritative, NOT this intake's snapshot. If the list differs from the four found at intake (`principles`, `help-dump`, `readme-extraction`, `skill`), audit what the runtime list says. Record the `shll` row of `shll version` at audit time for the report (v0.0.23 at intake).

### 2. Audit hop against each standard

**Mechanical contracts — execute each standard's own "Verifying conformance" checklist verbatim:**

- **help-dump** (binary): `hop help-dump` exits 0, valid JSON to stdout only, stderr empty; envelope is `{tool, version, schema_version, root}` with no `captured_at`; `completion`/`help`/hidden commands absent from the tree; `version` reflects the built binary, not a literal; a minimal test pins exit 0 + valid JSON + expected `tool`/`schema_version`. Baseline: hop already ships `help-dump` with tests (`src/cmd/hop/help_dump.go`, `help_dump_test.go`) — the audit verifies conformance to the current standard text, including the filter rules and the hidden-declaration requirement.
- **readme-extraction** (repo): README head order (`# hop` H1 → canonical toolkit blockquote → contiguous badge lines → tagline prose); slice tail ends at first footer heading (`Contributing`/`Development`/`Building`/`License`/`Acknowledgements`); grep for relative link targets (`](./`, `](../`, `](docs/`) — each either points into `docs/site/` from the README, stays inside `docs/site/`, or must be absolute; all images absolute `https://…`; no ```` ```mermaid ```` fences destined for the site; no `#gh-*-mode-only` fragments; no `docs/site/` page named `overview`/`readme`/`commands`; README cross-links its `docs/site/` pages and the absolute command-reference URL `https://shll.ai/hop/commands/`. Baseline: README head structure conforms on visual inspection; `docs/site/` contains `install.md` and `workflows.md` (neither name reserved); full checklist still to be executed.

**Principles (foundation) — assess each of the ten numbered principles against hop's actual behavior:**

| # | Principle | What to check in hop |
|---|-----------|----------------------|
| 1 | Non-interactive by default | Every prompt path (fzf pickers, any confirmation) TTY-gated with a flag escape; no-TTY behavior refuses fast naming the flag. Prior art: change `1x1u` established no-TTY exit 3 for picker paths. |
| 2 | stdout data / stderr diagnostics | Per-subcommand stream split (docs/specs/cli-surface.md is the toolkit's template for this); `--json` coverage where output is machine-consumed (`hop ls --json` exists; check other surfaces, e.g. `where`, `config print`). |
| 3 | Help is a published contract | Layered help (summary → flags → examples); `help-dump` present and hidden (overlaps mechanical audit). |
| 4 | Fail fast, actionable errors | what/why/next in errors; exit-code convention 0/1/2 documented per subcommand (note hop also uses exit 3 for no-TTY per `1x1u` — verify the convention is documented and coherent); aggregation rule for batch verbs (`--all pull` etc.) explicit. |
| 5 | Visible mutation boundaries | Read-vs-write clear from name+help; destructive writes (`hop rm`, `hop.yaml` mutations via `add`/`clone` registration) support `--dry-run` (sharing the real code path) and consent per №1 (`--yes` where warranted). `add -p` preview exists — check semantics. |
| 6 | Stateless, retry-safe | Constitution article II (No Database) already mandates this; verify idempotency of writes (`add` re-runs, `clone` when already cloned, `config init` when file exists). |
| 7 | Compose, don't reinvent | `hop ls --trees` composes `wt list --json`; capability probing where hop calls peers; no reimplementation. |
| 8 | Graceful degradation | Missing `wt`/`fzf` = skip or clear message, not crash; TTY-gated color/decoration. |
| 9 | Bounded, high-signal output | Unbounded surfaces (batch fan-out output, `ls`) have caps or are naturally bounded; quiet behavior preserves data+errors. |
| 10 | Agent-discoverable documentation | README + docs/site per readme-extraction (overlaps mechanical audit); CLAUDE.md/AGENTS.md pointing at standards rather than restating; `hop skill` — see §4 below. |

### 3. Fix policy (proportionality boundary)

- **Fix in this change**: all mechanical-contract violations found, plus principle gaps that are small and additive — a missing flag (e.g., a `--json` on an output surface, a `--yes`/`--dry-run` on a destructive write), a misrouted stream, an unhelpful error message, an undocumented exit code.
- **Defer**: gaps whose fix would restructure the tool (new subsystems, grammar changes, cross-repo coordination). Record each as a `fab/backlog.md` item (this repo's convention — 4-char ID, dated, precedent: `[cmp7]` "deferred from the grammar-shim refactor") or as a fab draft change (`/fab-draft`) when the gap is already change-shaped; reference the ID in the conformance report.
- Where a fix touches the CLI surface, update `docs/specs/cli-surface.md` in the same commit (the spec documents per-subcommand streams/exit codes — principle №2/№4's enforcement anchor names it).

### 4. The `skill` standard: pre-decided disposition

hop has no `hop skill` subcommand. Per the standard's own adoption clause ("No tool ships `skill` today… A tool without a `skill` subcommand is not yet in violation" — phased per-repo adoption, no seven-repo flag-day) and the task's explicit instruction, the report section for `skill` states **"deferred, not yet adopted"**. Do NOT implement `hop skill` in this change. If a backlog item for skill adoption does not already exist, add one and reference it as the deferral target.

### 5. Deliverable: conformance report in the PR body

One fab change (this one). The PR body carries a conformance report structured as:

- **Header**: shll version audited against (the `shll` row of `shll version`, e.g. `shll v0.0.23`), audit date, hop version.
- **One section per standard** (in the order `shll standards` lists them): `PASS` or the gaps found. Each gap carries a disposition: **fixed here** (with the commit hash) or **deferred** to `<ref>` (backlog ID or draft-change name).
- Tests green (`just test` / `go test ./...` from `src/`). If any fix changed the command tree, re-run the help-dump verification checklist afterward and note the re-verification in the report.

## Affected Memory

- `cli/subcommands`: (modify) — if flags, exit codes, error wording, or stream routing change on any subcommand; the file documents exit codes and the help-dump contract.
- `cli/agent-non-interactive-usage`: (modify) — if TTY handling, `--json`/`--yes`/`--dry-run` coverage, or no-TTY exit semantics change (it documents the `1x1u` guarantees this audit builds on).

Only files actually touched by fixes get memory updates; a PASS-only audit with no behavior change needs no memory edits (the report lives in the PR body, not memory).

## Impact

- **Audited (read)**: entire CLI surface (`src/cmd/hop/*.go`), `README.md`, `docs/site/**`, `docs/specs/cli-surface.md`, help/error/prompt paths in `src/internal/*`.
- **Potentially modified (writes, scoped by findings)**: `src/cmd/hop/*` (flag additions, stream fixes, error wording), `README.md` / `docs/site/**` (readme-extraction violations), `docs/specs/cli-surface.md` (spec updates alongside CLI fixes), `fab/backlog.md` (deferred-gap items), tests alongside every behavior change (`test-alongside` per code-quality.md).
- **Not modified**: `fab/project/constitution.md` (the article lands via the separate in-flight change `260717-zono`; no file overlap with this change), no new top-level subcommands (constitution article VI — and `hop skill` is explicitly deferred).
- **Dependencies**: `shll` binary on this machine (v0.0.23) — audit input only, no build-time dependency.

## Open Questions

- None. The task pre-decides scope boundaries, the skill-standard disposition, the deliverable shape, and the failure protocol; the remaining decisions (which specific gaps are "small and additive") are apply-stage decide-and-record judgments bounded by §3's fix policy.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope: fix all mechanical-contract violations + small additive principle gaps only; restructuring gaps are deferred with a reference | Verbatim in the task (item 3); proportionality boundary explicitly drawn | S:95 R:85 A:95 D:95 |
| 2 | Certain | `skill` standard reported as "deferred, not yet adopted"; no `hop skill` implementation here | Verbatim in the task note; the standard's own adoption clause says a tool without `skill` is not yet in violation | S:95 R:90 A:95 D:95 |
| 3 | Certain | Conformance report lives in the PR body: one section per standard, PASS/gaps with fixed-here-or-deferred dispositions, shll version header | Verbatim in the task (item 4) | S:95 R:90 A:95 D:95 |
| 4 | Certain | Standards are re-enumerated at runtime during apply; this intake's snapshot (4 standards, shll v0.0.23) is grounding, not authority | Verbatim in the task (item 1: "The list is authoritative — do not assume") | S:95 R:95 A:95 D:95 |
| 5 | Certain | Deferred gaps recorded as `fab/backlog.md` items (4-char ID, dated); a fab draft change used instead only when a gap is already change-shaped | Task says "draft change or issue per this repo's convention"; repo convention is backlog.md — precedent `[cmp7]` records a deferred-from-change item exactly this way; no GitHub-issue precedent found | S:70 R:90 A:80 D:75 |
| 6 | Certain | Proceed independently of the in-flight constitution PR (`260717-zono-bind-constitution-toolkit-standards`); do not amend the constitution in this change | The article mandates this work but the audit doesn't depend on its merge; that PR touches only `constitution.md` + its own change folder — zero file overlap | S:75 R:85 A:85 D:80 |
| 7 | Certain | Test gate is the existing Go suite (`go test ./...` under `src/`, CI workflow from PR #39); no new test infrastructure | "Tests green" in task; repo has an established CI test gate; help-dump contract already pinned by `help_dump_test.go` | S:80 R:85 A:90 D:85 |

7 assumptions (7 certain, 0 confident, 0 tentative, 0 unresolved).
