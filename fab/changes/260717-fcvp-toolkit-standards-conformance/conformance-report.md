# Toolkit Standards Conformance Report — hop

**Standards audited against**: `shll v0.0.23` (the `shll` row of `shll version`; standards are versioned with the shll release)
**Audit date**: 2026-07-18
**hop version**: `v0.1.18-5-gf436a1c` (built binary; `hop v0.1.18` is the last release tag)

Standards were re-enumerated at apply time via `shll standards` and each body read via
`shll standards <name>`. The runtime list matched the intake snapshot exactly — four
standards, in this order:

| Standard | Scope | One-liner |
|----------|-------|-----------|
| `principles` | foundation | The ten toolkit CLI principles every tool is built against |
| `help-dump` | binary | Machine-readable help contract every tool must emit |
| `readme-extraction` | repo | README + docs/site structure standard for toolkit repos |
| `skill` | binary+repo | Agent skill bundle: `docs/site/skill.md` served by `<tool> skill` |

**Result summary**: 2 mechanical-contract violations found and **fixed here**
(help-dump `captured_at`; readme-extraction command-reference URL); 1 additive
principle gap **fixed here** (`hop rm --dry-run`); 3 gaps **deferred** with backlog
references (`hop rm` consent `[clc4]`; agent-entry files `[qner]`; `skill` adoption
`[armh]`). Tests green; help-dump command-tree contract re-verified after the change.

The `just build` / `go build` commands run from the repo root / `src`; the full Go
suite is `cd src && go test ./...`.

---

## 1. principles (foundation)

Each of the ten numbered principles assessed against hop's ACTUAL behavior — reading
`src/cmd/hop/*.go` and `src/internal/*`, running the built binary, and checking
`docs/specs/cli-surface.md` for the intended stream/exit-code contract.

| # | Principle | Verdict |
|---|-----------|---------|
| 1 | Non-interactive by default | **PASS** |
| 2 | stdout is data, stderr is diagnostics | **PASS** |
| 3 | Help is a published contract | **PASS** (see help-dump §2) |
| 4 | Fail fast with actionable errors | **PASS** |
| 5 | Visible mutation boundaries | **1 gap — fixed here (`--dry-run`); 1 deferred (`[clc4]`)** |
| 6 | Stateless, therefore retry-safe | **PASS** |
| 7 | Compose, don't reinvent | **PASS** |
| 8 | Graceful degradation | **PASS** |
| 9 | Bounded, high-signal output | **PASS** |
| 10 | Agent-discoverable documentation | **PARTIAL — README/site PASS; CLAUDE.md/AGENTS.md deferred (`[qner]`); `hop skill` deferred (`[armh]`, see §4)** |

**№1 Non-interactive by default — PASS.** Every fzf-spawning path is TTY-gated. A
controlling-terminal probe (`src/cmd/hop/tty.go::isTTY`, probing `/dev/tty`) guards each
picker seam; with no TTY the command fails fast with the distinct `errNoTTY` → **exit 3**
and the hint `hop: no TTY for interactive selection — pass a repo name or use ` +
"`hop ls --json`" (not a hang, not a mis-attributed 130). Verified live: `setsid hop
</dev/null` → exit 3 with that hint. A unique substring match short-circuits before the
guard, so an unambiguous name still resolves with no TTY. hop has no destructive
confirmation prompts at all today (the one destructive write, `hop rm`, is either an
interactive picker or an immediate `<name>` removal — see №5).

**№2 stdout is data / stderr is diagnostics — PASS.** Streams are split per-subcommand
(the split is documented in `docs/specs/cli-surface.md` § Stdout/stderr Conventions —
hop's spec is the toolkit template principle №2 names). Verified live: `hop ls`,
`hop <name> where`, `hop config where`, `hop config print`, `hop --help` all emit data
on stdout with empty stderr; status/errors/hints go to stderr. Machine-readability:
`hop ls --json` (and `--json --trees`) emit a stable JSON schema mirroring `wt list
--json`'s field names, with `schema_version`-style forward-compat on the sibling
help-dump contract. The other output surfaces emit a single atomic value (`where` →
one path, `config where` → one path, `config print` → raw comment-preserving YAML), so
a `--json` wrapper would add nothing a caller can't already consume — no gap.

**№3 Help is a published contract — PASS.** Help is layered: `rootLong` (`root.go`)
gives a summary, a grammar block, a cheat sheet, and a Notes block; per-subcommand `-h`
carries flags and examples. The machine tree is `hop help-dump` (hidden, walks the live
cobra tree, never regex-parses `-h`). See §2 for the help-dump mechanical audit (one
violation, fixed here).

**№4 Fail fast with actionable errors — PASS.** Errors are what/why/next (e.g. the
shell-only hints name the exact `eval "$(hop shell-init zsh)"` fix; the worktree
not-found error names `wt list` and `hop ls --trees`). Exit codes are documented and
coherent: `0` success, `1` operational, `2` usage, plus hop's documented extensions
`3` (no-TTY, from change `1x1u`) and `130` (fzf cancel) — the convention is centralized
in `main.go::translateExit` and documented in `docs/specs/cli-surface.md` § Exit Code
Conventions. Exit 3 is deliberately distinct from 130 so an agent can tell "no terminal"
from "user cancelled." The aggregation rule for the batch verbs is explicit
(`batch.go::runBatch`): per-repo `✓`/`✗`/`skip` lines, a final `summary:` line, and
**any failure → exit 1** (exit 0 iff `failed == 0`); a missing `git` aborts the batch
with a single hint.

**№5 Visible mutation boundaries — 1 gap fixed here, 1 deferred.** Read-vs-write is clear
from name+help (`where`/`ls`/`config print`/`config where` read; `add`/`rm`/`clone` write).
The gap: `hop rm` (and its hidden `hop config rm` alias) mutate `hop.yaml` — a destructive
registry write — but supported **no `--dry-run`** (verified: `hop rm alpha --dry-run` →
`unknown flag: --dry-run`). Principle №5 requires a destructive write to support an
accurate `--dry-run` that shares the real code path.
- **Fixed here**: added a `--dry-run` bool flag to both `hop rm [<name>]` and the hidden
  `hop config rm` alias. A dry-run resolves the target through the exact same path as a
  live removal and previews `would remove: <url>` + `dry-run: no changes written` on
  stderr, writing nothing (exit 0). It shares the live code path via the new
  `yamled.WouldRemoveURL` — the read-only front-half of `yamled.RemoveURL` (both call the
  same `removeURLFromTree` locate logic), so the "dry-run that drifts from the live path"
  failure mode the standard warns about cannot occur, and the forgiving not-found
  sentinels (`ErrURLNotFound`/`ErrGroupNotFound`) are identical on both paths. Files:
  `src/cmd/hop/config_rm.go`, `src/internal/yamled/yamled.go`, with tests in
  `src/cmd/hop/config_rm_test.go` and `src/internal/yamled/yamled_test.go`; the CLI
  surface is documented in `docs/specs/cli-surface.md` (new `hop rm` inventory row +
  behavioral scenarios). Verified live: `hop rm alpha --dry-run` → exit 0, correct
  preview, `hop.yaml` byte-for-byte unchanged.
- **Deferred to `[clc4]`**: the consent half of №5/№1 for the immediate `hop rm <name>`
  path (no `--yes`, no confirmation prompt). This is deferred, not fixed, because
  `hop rm <name>` is documented as immediate/non-interactive (docs/memory/cli/subcommands.md:
  "no confirmation prompt") and the picker path is already interactive consent — adding a
  mandatory prompt/`--yes` changes that contract and warrants a design decision
  (`hop rm` edits a reversible registry entry and never deletes on-disk data). Recorded
  as backlog `[clc4]`.

**№6 Stateless, therefore retry-safe — PASS.** Constitution article II (No Database)
mandates statelessness; every invocation re-reads `hop.yaml` and re-checks disk. Writes
are idempotent: verified live that re-`hop add`-ing an already-registered/no-remote repo
is an exit-0 no-op that leaves `hop.yaml` unchanged; `clone` classifies on-disk state
(`stateAlreadyCloned` → `skip:` + still registers); `config init` guards an existing
file. Retrying after a partial failure converges rather than double-applying.

**№7 Compose, don't reinvent — PASS.** `hop ls --trees` and the `/<wt>` worktree suffix
shell out to `wt list --json` rather than reading wt's internals; `hop <name> open`
delegates to `wt open` (positional-path form → wt's app menu). `hop update` wraps `brew`
(never parses formulas by hand). YAML is edited via a Go library (`internal/yamled`), not
hand-parsed. All subprocess calls route through `internal/proc` (Constitution I). One
nuance recorded, not a gap: hop's only peer composition is with `wt`, and it does not
probe `wt --help` for advertised flags before calling (the standard's probe pattern) —
but hop uses only wt's stable, long-standing surface (`list --json`, `open`) and degrades
gracefully when wt is absent (§8), so there is no flag-day risk to negotiate today.

**№8 Graceful degradation — PASS.** A missing optional peer is a clear message + exit,
never a crash: missing `wt` → `hop: wt: not found on PATH.` exit 1 (lazily, only when a
`--trees`/`open`/`/<wt>` path needs it — the first `wt list` failure aborts, subsequent
ones can't recur); missing `fzf` → the install hint exit 1; a per-repo `wt list` failure
in `hop ls --trees` degrades inline as `(wt list failed: <err>)` without aborting the
table. External tools are checked lazily, so subcommands that don't need a tool work
without it.

**№9 Bounded, high-signal output — PASS.** hop's surfaces are naturally bounded: `hop ls`
and the batch fan-out emit exactly one line per registered repo (a count the user
controls via `hop.yaml`); there is no unbounded log/stream surface that could dump ten
thousand lines. Batch output is data + per-repo status + a summary tail — no progress
spinners or decoration to suppress, so the absence of a `--quiet` flag is not a gap
(there is nothing chatter-like to strip; the output is already data+errors).

**№10 Agent-discoverable documentation — PARTIAL.** The README + `docs/site/` tree
conform to the readme-extraction standard (see §3, PASS after the one fix). The two
remaining №10 obligations are deferred:
- **CLAUDE.md/AGENTS.md** pointing at the toolkit standards — hop ships neither. Deferred
  to `[qner]`: authoring an agent-entry pointer file is documentation curation, not a
  mechanical fix, so it sits outside this change's small-additive boundary.
- **`hop skill`** — the most forward-leaning №10 obligation — is deferred per the `skill`
  standard's own phased-adoption clause; see §4 and `[armh]`.
№10 is a SHOULD, and the standard itself calls it "the toolkit's weakest principle today
by design"; hop's core discoverability (README, `docs/site/`, `hop ls --json`, `help-dump`)
is in place.

---

## 2. help-dump (binary)

Executed the standard's "Verifying conformance" checklist VERBATIM against the freshly
built binary (`just build` → `bin/hop`):

| Checklist item | Result |
|----------------|--------|
| `hop help-dump` exits 0, valid JSON to stdout only, stderr empty | **PASS** (exit 0, 0 stderr bytes, parses as JSON) |
| Envelope is `{tool, version, schema_version, root}` — **no `captured_at`** | **FAIL before → PASS after fix** |
| `completion`, `help`, and all hidden commands absent from the tree | **PASS** (top-level: add, clone, config, ls, rm, shell-init, update; hidden `config add`/`config rm`/`help-dump`/`--shim-plan` all self-filtered) |
| `version` reflects the built binary, not a literal | **PASS** (`v0.1.18-5-gf436a1c`, from `rootCmd.Version`) |
| A minimal test pins exit 0 + valid JSON + expected `tool`/`schema_version` | **PASS** (pre-existing tests + a new envelope-shape test) |

**Gap found — `captured_at` emitted (fixed here).** The `Doc` struct carried a
`CapturedAt string json:"captured_at"` field, emitting `"captured_at": ""` in the
envelope. The shll v0.0.23 standard states plainly: **"Do not emit `captured_at`. The
capture timestamp is owned by shll.ai … The puller stamps it after capture,"** and the
envelope shape is exactly `{tool, version, schema_version, root}`. An empty-string field
still violates the shape (a strict Zod consumer would reject the extra key). No CI wiring
injects it (a grep of `.github/` finds no help-dump publish step — the `[jr5f]` publish
job referenced an older frozen contract that included `captured_at`; the current standard
supersedes it).

- **Fixed here**: removed the `CapturedAt` field from the `Doc` struct and `buildHelpDoc`
  in `src/cmd/hop/help_dump.go`; updated the doc comments to state the standard's envelope
  and its explicit no-`captured_at` rule. Test updated in `src/cmd/hop/help_dump_test.go`:
  the old `captured_at == ""` assertion is replaced by `TestHelpDumpEnvelopeShape`, which
  asserts (against the raw JSON) that `captured_at` is absent AND the key set is exactly
  `{tool, version, schema_version, root}` — catching any re-introduction.

**Re-verification after the change** (intake §5 — the command tree was inspected because
the change touched CLI-surface docs, though it added only a flag): re-ran the full
checklist on the rebuilt binary. `hop help-dump` → exit 0, stderr empty, keys exactly
`{root, schema_version, tool, version}`, `captured_at` absent, forbidden nodes
(completion/help/help-dump) absent, version = built binary, top-level command tree
unchanged (`--dry-run` is a flag, not a command). **All items PASS.**

---

## 3. readme-extraction (repo)

Executed the standard's "Verifying conformance" checklist VERBATIM against `README.md`
and `docs/site/` (which contains `install.md` and `workflows.md` — neither a reserved
name):

| Checklist item | Result |
|----------------|--------|
| README top: `#` H1 → toolkit blockquote → contiguous badges → tagline prose | **PASS** (`# hop` → the exact canonical blockquote → three badge links → the tagline) |
| Tail ends before the first footer heading (`Contributing`/`Development`/`Building`/`License`/`Acknowledgements`) | **PASS** (README has none of these headings — the whole doc is site-worthy) |
| Grep relative targets (`](./`, `](../`, `](docs/`): each points into `docs/site/` from README, stays inside `docs/site/`, or is absolute | **PASS** (the only relative targets are `docs/site/install.md` / `docs/site/workflows.md` — the auto-rewritten README→docs/site form; `docs/specs/*` links are already absolute `https://github.com/...`) |
| All images absolute `https://…` | **PASS** (every `![…]` is a shields.io / absolute URL) |
| No ```` ```mermaid ```` fences destined for the site | **PASS** (none) |
| No `#gh-*-mode-only` fragments | **PASS** (none) |
| No `docs/site/` page named `overview`/`readme`/`commands` | **PASS** (`install`, `workflows`) |
| README cross-links its `docs/site/` pages and the absolute command-reference URL `https://shll.ai/hop/commands/` | **FAIL before → PASS after fix** |

**Gap found — non-canonical command-reference URL (fixed here).** The README's Reference
section linked the command reference as `https://shll.ai/tools/hop/commands/`. The
standard's rule 8 gives the canonical form: "point at the generated command reference
with the absolute URL `https://shll.ai/<tool>/commands/`" → `https://shll.ai/hop/commands/`.
The extra `/tools/` segment is a live broken link against the site's `/<tool>/…` slug
scheme (standards themselves render at `/shll/standards/…`, confirming no `/tools/`
prefix). All other checklist items PASS.

- **Fixed here**: `README.md` — changed the command-reference link to
  `https://shll.ai/hop/commands/`.

The `docs/site/` closure rules (no `..` escapes, external links absolute-by-author, all
images absolute, README→docs/site links natural) all hold — the two site pages contain no
relative link/image escapes.

---

## 4. skill (binary + repo)

**Deferred, not yet adopted.** hop ships no `hop skill` subcommand. Per the standard's own
adoption clause — "No tool ships `skill` today… A tool without a `skill` subcommand is not
yet in violation" (phased per-repo adoption, no seven-repo flag-day; principle №10 is a
SHOULD) — and the intake's pre-decided disposition, `hop skill` is **not implemented in
this change**. The deferral is tracked as backlog item **`[armh]`** (`hop skill` +
canonical `docs/site/skill.md` + the sync/drift-guard embedding pattern), which is the
reference target for this section.

No `hop skill` code was added.

---

## Dispositions summary

| Standard | Disposition |
|----------|-------------|
| principles №1–4, 6–9 | **PASS** (no change) |
| principles №5 `--dry-run` | **Fixed here** — `src/cmd/hop/config_rm.go`, `src/internal/yamled/yamled.go` (+ tests, spec) |
| principles №5 consent | **Deferred → `[clc4]`** |
| principles №10 CLAUDE.md/AGENTS.md | **Deferred → `[qner]`** |
| help-dump `captured_at` | **Fixed here** — `src/cmd/hop/help_dump.go` (+ test) |
| readme-extraction command URL | **Fixed here** — `README.md` |
| skill | **Deferred, not yet adopted → `[armh]`** |

**Files changed**: `src/cmd/hop/help_dump.go`, `src/cmd/hop/help_dump_test.go`,
`src/cmd/hop/config_rm.go`, `src/cmd/hop/config_rm_test.go`,
`src/internal/yamled/yamled.go`, `src/internal/yamled/yamled_test.go`, `README.md`,
`docs/specs/cli-surface.md`, `fab/backlog.md`.

**Tests**: `cd src && go test ./...` — all packages PASS; `go vet ./...` clean.
Commit hashes are added at ship time.
