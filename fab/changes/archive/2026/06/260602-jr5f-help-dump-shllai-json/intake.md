# Intake: Help-dump CLI tree → help/hop.json → PR into shll.ai

**Change**: 260602-jr5f-help-dump-shllai-json
**Created**: 2026-06-02
**Status**: Draft

## Origin

Initiated via `/fab-new jr5f` from the `fab/backlog.md` item:

> [jr5f] 2026-06-02: Add a build-time 'help-dump' step that emits hop's CLI help tree as help/hop.json and PRs it into sahil87/shll.ai (the shll.ai landing site renders it as an expandable 'Command reference' on the hop tool page). CONTRACT (frozen — copy the reference sample at sahil87/shll.ai path help/wt.json): JSON shape is {tool, version, captured_at (ISO-8601 UTC), schema_version: 1, root: Node} where Node = {name, path (full invocation e.g. 'hop where'), short (one-line desc), usage, text (the RAW -h output byte-for-byte, newlines preserved), commands: Node[] (recursive; empty array = leaf)}. PRODUCER (hop is Cobra/Go, binary 'hop', main at src/cmd/hop): walk the cobra command tree programmatically (rootCmd.Commands() recursively), NOT regex-parsing -h text; per node capture cmd.Name / cmd.CommandPath() / cmd.Short / cmd.UseLine() and cmd.UsageString() (or Long+UsageString) as 'text'. FILTER OUT cobra's auto-generated 'completion' and 'help' subcommands and any cmd.Hidden==true. VERSION: read from the built binary (rootCmd.Version / ldflags) — do NOT hardcode. NOTE hop's pre-cobra -R / tool-form argv handling (extractDashR) lives outside the cobra tree — decide whether to document those in root 'text' manually or leave to the README. PUSH: in CI after build, run the dump, write help/hop.json, validate it parses, then open a PR into sahil87/shll.ai using the existing repo secret SHLLAI_TOKEN (contents + pull-request write) with auto-merge enabled (PR, not direct push to main, to avoid the multi-repo push race). This is hop's slice of a 7-tool rollout; the shll.ai site-side consumer (Astro loader + reference UI) is tracked separately in the shll.ai repo.

Interaction mode: one-shot, preceded by a `/fab-discuss` orientation and a backlog listing. The backlog item is itself a **frozen contract**, so the bulk of the design is pinned externally rather than decided here. During intake the codebase was inspected to confirm the producer is implementable as specified and the reference `help/wt.json` was fetched from `sahil87/shll.ai` to lock the exact output shape.

Key facts confirmed against the live codebase:
- Module `github.com/sahil87/hop`; Go source under `src/`; `main` package at `src/cmd/hop`; `go.mod` at `src/go.mod` (Go 1.22, cobra v1.8.1).
- `src/cmd/hop/main.go` declares `var version = "dev"`, overridden via `-ldflags "-X main.version=..."`, and assigns `rootCmd.Version = version`.
- `src/cmd/hop/root.go::newRootCmd()` wires 8 subcommands: `clone`, `pull`, `push`, `sync`, `ls`, `shell-init`, `config`, `update`. `config` has its own children (`init`, `where`, `print`, `scan`).
- The pre-cobra `-R` / tool-form handling lives in `extractDashR` (`src/cmd/hop/main.go`), **outside** the cobra tree — it is invisible to `rootCmd.Commands()`.
- `.github/workflows/release.yml` already cross-compiles (4 targets, `working-directory: src`), creates a GitHub Release, and updates the homebrew tap. The help-dump + PR is a new step appended after the build.
- The reference `help/wt.json` exists in `sahil87/shll.ai` and matches the contract's `{tool, version, captured_at, schema_version, root: Node}` shape exactly (verified by fetch).

## Why

**Problem.** The shll.ai landing site renders an expandable "Command reference" on each tool's page from a per-tool `help/<tool>.json` artifact. hop has no such artifact, so its tool page can show no command reference. This is hop's slice of a 7-tool rollout where every tool publishes the same JSON shape.

**Consequence if not done.** hop's tool page on shll.ai stays without a command reference, breaking parity with the other tools in the rollout. Maintaining the reference by hand would drift from the actual CLI surface (hop's help text is already non-trivial — see `rootLong` in `root.go`), defeating the point of a generated artifact.

**Why this approach.**
- **Programmatic cobra walk, not regex-parsing `-h`.** The contract mandates walking `rootCmd.Commands()` recursively and reading structured fields (`cmd.Name()`, `cmd.CommandPath()`, `cmd.Short`, `cmd.UseLine()`, `cmd.UsageString()`). This is robust to help-text formatting changes and is the same approach the reference tool (`wt`) used. The `text` field still carries the raw rendered help byte-for-byte (newlines preserved) so the site can display exactly what a terminal shows.
- **CI-time generation, PR not push.** Generating in CI right after the version-stamped build guarantees the captured `version` matches the released binary. Opening a PR into `shll.ai` with auto-merge (rather than pushing to `main`) avoids the multi-repo push race when all 7 tools publish around the same time.
- **Frozen contract.** The JSON shape, schema_version (1), filter rules, version sourcing, and push mechanism are all pinned by the backlog item and the existing `help/wt.json` reference. We copy, not redesign.

## What Changes

### A. New help-dump producer (Go, inside the hop binary)

A new subcommand emits the help tree as JSON to stdout. The recommended surface is a **hidden** cobra subcommand so it ships in the released binary (CI can invoke it) without cluttering the user-facing help, and — being `Hidden` — it filters itself out of its own output per the filter rule below.

Proposed surface (subject to confirmation in plan): `hop help-dump` (hidden), printing the JSON document to stdout. Alternatives considered: a hidden `--dump-help` root flag, or a standalone `cmd/help-dump` tool. The hidden-subcommand form is preferred because it reuses the already-constructed `rootCmd` (so it sees the real tree and the real `rootCmd.Version`) and matches cobra idioms already in the codebase.

**Walk algorithm** (programmatic, not regex):

```
buildNode(cmd *cobra.Command) Node:
  node.name    = cmd.Name()
  node.path    = cmd.CommandPath()          // e.g. "hop config scan"
  node.short   = cmd.Short
  node.usage   = cmd.UseLine()              // e.g. "hop config scan <dir> [flags]"
  node.text    = (cmd.Long != "" ? cmd.Long+"\n\n" : "") + cmd.UsageString()  // Long+Usage when Long is set, else Usage alone (uniform: root & subcommands)
  node.commands = []
  for child in cmd.Commands():
     if shouldSkipChild(child): continue
     node.commands.append(buildNode(child))
  return node

shouldSkipChild(c):
  return c.Name() in {"completion", "help"}  // cobra auto-generated
      || c.Hidden                            // includes help-dump itself
      || c.IsAdditionalHelpTopicCommand()    // defensive
```

**Top-level document:**

```go
type Doc struct {
    Tool          string `json:"tool"`           // "hop"
    Version       string `json:"version"`         // rootCmd.Version (ldflags); "dev" in local builds
    CapturedAt    string `json:"captured_at"`     // ISO-8601 UTC, e.g. "2026-06-02T00:00:00Z"
    SchemaVersion int    `json:"schema_version"`  // 1
    Root          Node   `json:"root"`
}

type Node struct {
    Name     string `json:"name"`
    Path     string `json:"path"`
    Short    string `json:"short"`
    Usage    string `json:"usage"`
    Text     string `json:"text"`
    Commands []Node `json:"commands"`   // empty array (not null) for leaves
}
```

Output requirements:
- Marshal with stable 2-space indentation (match the reference `help/wt.json`).
- `commands` MUST serialize as `[]` for leaves, never `null` — initialize the slice to a non-nil empty slice.
- `version` reads from `rootCmd.Version` (which is `main.version`, ldflag-injected). It MUST NOT be hardcoded. In a local `go run`/unstamped build this is `"dev"`; CI builds stamp the real tag.
- `captured_at` is ISO-8601 UTC, **injected by the CI step, not the producer**. The Go producer leaves `captured_at` empty (`""`); the CI step stamps it with a date-floored UTC value — `captured_at=$(date -u +%Y-%m-%dT00:00:00Z)` then `jq --arg t "$captured_at" '.captured_at=$t'`. This matches the `wt.json` reference (`00:00:00Z`) and keeps the dump deterministic per day. (Resolved during `/fab-clarify`.)
- `text` field is built **uniformly** for every node: `cmd.Long + "\n\n" + cmd.UsageString()` when `cmd.Long` is non-empty, else `cmd.UsageString()` alone. This mirrors the `wt.json` reference, where the root `text` carries the full long narrative followed by the usage block. (Resolved during `/fab-clarify`.)

**Reference shape** (from `sahil87/shll.ai` `help/wt.json`, abbreviated — hop's output mirrors this structure):

```json
{
  "tool": "wt",
  "version": "1.4.2",
  "captured_at": "2026-06-02T00:00:00Z",
  "schema_version": 1,
  "root": {
    "name": "wt",
    "path": "wt",
    "short": "Git worktree management — create, list, open, delete worktrees.",
    "usage": "wt [command]",
    "text": "Git worktree management — ...\n\nUsage:\n  wt [command]\n\nAvailable Commands:\n  create ...",
    "commands": [
      { "name": "create", "path": "wt create", "short": "Create a git worktree",
        "usage": "wt create [branch] [flags]", "text": "...", "commands": [] }
    ]
  }
}
```

### B. The `extractDashR` / tool-form blind spot

hop's `-R` and `<name> <tool>...` forms are handled in `extractDashR` **before** cobra parses argv, so they do not appear as cobra commands and the walk will not see them. Per the contract, this is an explicit decision point:

- **Option 1 (preferred):** leave `-R` / tool-form documentation to the README and the root command's existing `Long` text. The root node's `text` already includes the full `rootLong` (which documents `hop <name> -R <cmd>...` and `hop <name> <tool>...`), so they are *described* in the dump even though they are not separate nodes. No producer code needed.
- **Option 2:** manually append synthetic notes to the root node's `text`. Rejected as redundant given `rootLong` already covers them.

This is recorded as a Confident assumption (Option 1) — see Assumptions table.

### C. CI step: generate, validate, PR into shll.ai

Append a step to `.github/workflows/release.yml` after the cross-compile build (the linux-amd64 binary built there can run on the `ubuntu-latest` runner), before or after the GitHub Release step:

```yaml
      - name: Publish help reference to shll.ai
        env:
          SHLLAI_TOKEN: ${{ secrets.SHLLAI_TOKEN }}
        run: |
          set -euo pipefail
          # 1. Dump the help tree from the freshly built, version-stamped binary
          #    (producer leaves captured_at empty).
          ./dist/hop-linux-amd64/hop help-dump > /tmp/hop.raw.json
          # 2. Inject a date-floored UTC captured_at (matches the wt.json convention).
          captured_at=$(date -u +%Y-%m-%dT00:00:00Z)
          jq --arg t "$captured_at" '.captured_at=$t' /tmp/hop.raw.json > /tmp/hop.json
          # 3. Validate it parses (fail the release if malformed).
          jq -e '.tool=="hop" and .schema_version==1 and (.root|type=="object") and (.captured_at|test("Z$"))' /tmp/hop.json >/dev/null
          # 3. Clone shll.ai, drop the file on a fresh branch, open an auto-merge PR.
          git clone "https://x-access-token:${SHLLAI_TOKEN}@github.com/sahil87/shll.ai.git" /tmp/shllai
          cd /tmp/shllai
          branch="hop-help-${{ steps.version.outputs.version }}"
          git checkout -b "$branch"
          mkdir -p help
          cp /tmp/hop.json help/hop.json
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add help/hop.json
          git commit -m "hop help reference v${{ steps.version.outputs.version }}"
          git push origin "$branch"
          GH_TOKEN="$SHLLAI_TOKEN" gh pr create --repo sahil87/shll.ai \
            --base main --head "$branch" \
            --title "hop help reference v${{ steps.version.outputs.version }}" \
            --body "Automated CLI help reference dump from hop v${{ steps.version.outputs.version }}."
          GH_TOKEN="$SHLLAI_TOKEN" gh pr merge --repo sahil87/shll.ai "$branch" --auto --squash
```

(Exact placement, idempotency on re-run for the same tag, and whether `captured_at` is normalized by CI are plan-level details.)

Requirements:
- Uses the existing repo secret `SHLLAI_TOKEN` (must grant `contents:write` + `pull-requests:write` on `sahil87/shll.ai`). The current workflow does not reference this secret; its existence/scope is an external assumption (see Open Questions).
- PR + auto-merge, **never** direct push to `shll.ai` `main` (avoids the multi-repo push race across the 7-tool rollout).
- The dump must be validated (`jq -e` or equivalent) so a malformed artifact fails the release loudly.

### D. Local developer ergonomics (optional, plan-level)

A `just help-dump` recipe delegating to `scripts/help-dump.sh` (per the Thin Justfile constitution principle) lets a developer regenerate `help/hop.json` locally for inspection. Optional — include only if it doesn't expand scope materially.

## Affected Memory

- `cli/subcommands`: (modify) document the new hidden `help-dump` subcommand (surface, hidden status, stdout JSON contract).
- `build/release-pipeline`: (modify) document the new CI step — help-dump → validate → PR-into-shll.ai with auto-merge, the `SHLLAI_TOKEN` secret, and the PR-not-push rationale.
- `architecture/package-layout`: (modify) note where the help-dump producer lives (new file under `src/cmd/hop/`) and that it walks the cobra tree, skipping `completion`/`help`/hidden.

## Impact

- **Code (new/modified):**
  - New producer file, e.g. `src/cmd/hop/help_dump.go` (+ `help_dump_test.go`) — the cobra subcommand and walk.
  - `src/cmd/hop/root.go` — register the hidden `help-dump` subcommand in `newRootCmd()`.
  - Possibly `src/cmd/hop/main.go` — none expected; `rootCmd.Version` is already set.
- **CI:** `.github/workflows/release.yml` — new "Publish help reference" step; relies on `jq` (preinstalled on `ubuntu-latest`) and `gh` (preinstalled).
- **Scripts/justfile (optional):** `scripts/help-dump.sh` + `just help-dump`.
- **External dependency:** `sahil87/shll.ai` repo must accept PRs from a token with the right scopes; secret `SHLLAI_TOKEN` must exist on the `hop` repo.
- **Constitution alignment:** Security First (use `gh`/`git` with explicit args, no shell-string injection of untrusted input — version comes from the trusted tag); Wrap Don't Reinvent (shell out to `git`/`gh`, marshal via `encoding/json`); Thin Justfile (logic in `scripts/`, recipe is a one-liner); Minimal Surface Area (the new subcommand is **hidden**, so it does not expand the user-facing surface — this is the justification the constitution requires for a new top-level subcommand).
- **Cross-platform:** producer is pure Go (`encoding/json`, `cobra`), builds on all 4 targets. CI runs the linux-amd64 build on the linux runner.

## Open Questions

- ~~Does the `SHLLAI_TOKEN` secret already exist on the `hop` repo with `contents:write` + `pull-requests:write` scopes for `sahil87/shll.ai`?~~ **Resolved at intake** — user confirmed the secret exists with the required scopes. The CI step assumes it is present. <!-- clarified: SHLLAI_TOKEN exists on hop repo with contents+PR write scopes on shll.ai — user confirmed -->.
- ~~Producer surface: hidden `hop help-dump` subcommand vs. a hidden root flag vs. a separate tool?~~ **Resolved** — hidden `hop help-dump` cobra subcommand. <!-- clarified: producer surface is a hidden `hop help-dump` subcommand — user confirmed -->
- ~~`captured_at`: generated by the Go producer at dump time, or normalized/injected by CI?~~ **Resolved** — producer leaves it empty; CI injects a date-floored UTC value via `jq` (`date -u +%Y-%m-%dT00:00:00Z`), matching the `wt.json` reference. <!-- clarified: captured_at injected by CI as date-floored UTC — user chose -->
- ~~`text` field content: `cmd.UsageString()` alone, or `Long + "\n\n" + UsageString()`?~~ **Resolved** — `Long + "\n\n" + UsageString()` when `Long` is non-empty, else `UsageString()` alone, applied uniformly to all nodes. <!-- clarified: text = Long+UsageString everywhere — user chose -->
- Should the CI step run on every `v*` tag release (recommended) — confirmed by "in CI after build" in the contract.

## Clarifications

### Session 2026-06-03

| # | Action | Detail |
|---|--------|--------|
| 9 | Changed | `captured_at` — CI injects date-floored UTC (`date -u +%Y-%m-%dT00:00:00Z` via `jq`); producer leaves it empty |
| 10 | Changed | `text` — `cmd.Long + "\n\n" + UsageString()` when Long is non-empty, else UsageString alone; uniform across all nodes |

### Session 2026-06-03 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 6 | Confirmed | Hidden `hop help-dump` cobra subcommand |
| 7 | Confirmed | `-R` / tool-form left to README + root `Long` text |
| 8 | Confirmed | CI step appended to `release.yml`, on `v*` tags after build |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | JSON shape is `{tool, version, captured_at, schema_version:1, root:Node}` with `Node={name,path,short,usage,text,commands[]}` | Frozen by the backlog contract AND verified against the live `help/wt.json` reference fetched from shll.ai | S:98 R:90 A:95 D:98 |
| 2 | Certain | Producer walks `rootCmd.Commands()` recursively reading structured cobra fields, NOT regex-parsing `-h` | Explicitly mandated by the contract; codebase confirms cobra tree is walkable | S:98 R:80 A:95 D:95 |
| 3 | Certain | Filter out `completion`, `help`, and any `cmd.Hidden==true` subcommands | Explicitly mandated by the contract | S:98 R:85 A:98 D:98 |
| 4 | Certain | `version` is read from `rootCmd.Version` / ldflags, never hardcoded | Mandated by contract; `main.go` confirms `var version` + `-X main.version` ldflag wiring | S:98 R:90 A:98 D:98 |
| 5 | Certain | CI publishes via PR into `sahil87/shll.ai` with auto-merge using `SHLLAI_TOKEN`, never a direct push to main | Mandated by contract (avoids multi-repo push race) | S:95 R:70 A:90 D:95 |
| 6 | Certain | Producer surface is a hidden `hop help-dump` cobra subcommand | Clarified — user confirmed. Reuses the constructed `rootCmd` (real tree + real Version), matches codebase idioms, and being Hidden it self-filters and keeps the user-facing surface unchanged (satisfies Minimal Surface Area) | S:95 R:75 A:80 D:70 |
| 7 | Certain | `-R` / tool-form (`extractDashR`) are left to README + root `Long` text, not synthesized as nodes | Clarified — user confirmed. They live outside the cobra tree; `rootLong` already documents them, so they appear in the root `text` | S:95 R:75 A:80 D:75 |
| 8 | Certain | The new CI step is appended to the existing `release.yml`, triggered on `v*` tags after the cross-compile build | Clarified — user confirmed. release.yml is the only build-and-publish workflow and runs on `v*` tags | S:95 R:80 A:85 D:80 |
| 9 | Certain | `captured_at` is left empty by the Go producer; the CI step injects a date-floored UTC value (`date -u +%Y-%m-%dT00:00:00Z`) via `jq` | Clarified — user chose CI-injected date-floored UTC. Matches the `wt.json` reference convention and keeps the dump deterministic per day | S:95 R:75 A:55 D:50 |
| 10 | Certain | Each node's `text` = `cmd.Long + "\n\n" + UsageString()` when `Long` is non-empty, else `UsageString()` alone — applied uniformly to root and subcommands | Clarified — user chose Long+UsageString everywhere. Mirrors the `wt.json` reference (root carries the full narrative); one uniform rule | S:95 R:75 A:60 D:55 |
| 11 | Certain | `SHLLAI_TOKEN` exists on the hop repo with `contents:write` + `pull-requests:write` scopes for `sahil87/shll.ai` | Clarified — user confirmed at intake. The CI step assumes the secret is present and correctly scoped | S:95 R:40 A:90 D:90 |

11 assumptions (11 certain, 0 confident, 0 tentative, 0 unresolved).
