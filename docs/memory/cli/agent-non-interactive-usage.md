---
description: "Driving `hop` from an AI agent or CI with no TTY: the four 1x1u guarantees — partial-capture-immune single-function shim, `hop ls --json` registry-as-data, no-TTY exit 3 (distinct from 130), and `HOP_WRAPPER` hint suppression; recommended invocation patterns and exit codes"
type: memory
---
# Agent / Non-Interactive Usage

How to drive `hop` from an AI coding agent or a CI script — i.e. with **no human at a TTY** and (often) under a per-function shell snapshotter. Introduced by change `1x1u` (`260615-1x1u-agent-non-interactive-usage`), which made four cooperating changes that together turn `hop` from a human-at-a-terminal tool into one a non-interactive caller can use reliably. Each item mirrors an existing precedent in the sibling tools `wt` / `idea` (Constitution IV — wrap, don't reinvent).

This is an **orientation note**: the authoritative details live in the per-domain memory files; this page ties them together for the agent audience. Cross-links are given inline.

## The four guarantees

| # | Guarantee | Mechanism | Authoritative doc |
|---|---|---|---|
| 1 | The shim survives **partial capture** | `posixInit` emits a SINGLE self-contained `hop()` — the PASSTHROUGH cd side-channel body is inlined into the `case` arm; the standalone `_hop_passthrough()` sibling was removed | [architecture/package-layout § single self-contained hop()](/architecture/package-layout.md#design-decision-posixinit-is-a-single-self-contained-hop--partial-capture-immunity-change-1x1u), [subcommands § shell-init emitted text](/cli/subcommands.md#hop-shell-init-shell-emitted-text) |
| 2 | Repos are **enumerable as data** | `hop ls --json` (and `hop ls --json --trees`) emit a stable JSON schema, mirroring `wt list --json` field names | [subcommands § hop ls --json](/cli/subcommands.md#hop-ls---json-machine-readable-output) |
| 3 | The no-TTY failure is **diagnosable** | A TTY guard on every fzf-spawning path returns the `errNoTTY` sentinel → **exit 3** (distinct from 130 fzf-cancel), with an actionable stderr hint | [match-resolution § Cancellation, missing fzf, and no-TTY](/cli/match-resolution.md#cancellation-missing-fzf-and-no-tty), [subcommands § exit-code convention](/cli/subcommands.md#exit-code-convention) |
| 4 | Shell-only hints are **suppressed for wrappers** | The shim exports `HOP_WRAPPER=1`; the binary suppresses the "install the shim" hint TEXT (keeping exit 2) when it sees it | [subcommands § HOP_WRAPPER hint suppression](/cli/subcommands.md#hop_wrapper--shell-only-hint-suppression) |

## Why each one matters for an agent

1. **Partial-capture immunity.** `hop` is a shell function emitted by `hop shell-init`. Claude Code's shell-snapshot mechanism captures functions per-function — it captured `hop()` but NOT its former sibling `_hop_passthrough()`, so every PASSTHROUGH command (`where`, `ls`, `add`, `rm`, `clone`, `config`, `pull`/`push`/`sync`, `--help`, `--version`) died with `_hop_passthrough: command not found` while the binary was correct the whole time. This is a NEW facet of the documented "stale shim" bug class (the older facet — name-drift between shim and binary — was structurally fixed in change `gyo0`). Inlining the body into `hop()` is the only form provably immune: there is no second top-level definition object for a snapshotter to drop. The `WT_CD_FILE` cd side-channel semantics are preserved verbatim.

2. **Data interface.** Before this change the only machine-readable surface was the hidden `help-dump` (a help-tree, not a repo inventory); `hop ls` was human-padded columns. `hop ls --json` gives agents a registry inventory (`{name, path, url, group}` per repo, in YAML source order) and `--json --trees` nests per-worktree status. Edge states are representable IN JSON rather than as inline human text — see the schema table in the subcommands doc. Use `hop ls --json` to discover repos, then drive other verbs by exact name.

3. **No-TTY diagnosis.** Bare `hop`, ambiguous/zero-match name resolution, and `hop rm`/`hop clone` with no name all reach fzf. With no controlling TTY, fzf would exit 1 and get folded into `errFzfCancelled` → exit 130 — a confusing "user cancelled" for what is really "no terminal to pick with". The guard fails fast **before** spawning fzf with the distinct `errNoTTY` → exit 3 and the hint `hop: no TTY for interactive selection — pass a repo name or use ` + "`hop ls --json`". A **unique** substring match still resolves with no TTY (it short-circuits before the guard), so agents can resolve unambiguous names directly.

4. **Hint suppression.** Shell-only verbs (`hop <name>`, `hop <name> cd`, tool-form) correctly error (exit 2) for a human without the shim, pointing them at `eval "$(hop shell-init zsh)"`. For a caller that has the wrapper present that hint is noise. The shim exports `HOP_WRAPPER=1`; the binary's `shellOnlyHintErr` (`root.go`) suppresses the hint TEXT while **keeping exit 2** (the form is still one the binary genuinely cannot fulfill — cd/tool-form run in the parent shell).

## Recommended agent invocation patterns

- **Enumerate**: `hop ls --json` (registry as data) or `hop ls --json --trees` (with per-repo worktree state). Always valid JSON — an empty registry emits `[]`.
- **Resolve a path**: `hop <name> where` (prints the absolute path to stdout; reached via PASSTHROUGH, scriptable). Pass an exact/unambiguous name to avoid the picker. A no-name or ambiguous query with no TTY exits 3, not 130.
- **Do NOT rely on**: bare `hop` (the picker — exit 3 with no TTY), tool-form `hop <name> <tool>` (shell-only — the binary returns exit 2; with `HOP_WRAPPER` set the hint text is suppressed but the exit code stays 2). Scripts run tools themselves after resolving the path via `hop <name> where`.

## Exit codes an agent should distinguish

| Code | Meaning for an agent |
|---|---|
| 0 | success |
| 1 | application error (or `errSilent` — a hint already on stderr) |
| 2 | usage error, incl. shell-only forms the binary can't honor directly |
| **3** | **no TTY** for an interactive selection — name the repo, or use `hop ls --json` |
| 130 | the user cancelled fzf (Esc / Ctrl-C) — not reachable from a no-TTY caller |

Full table and triggers: [subcommands § exit-code convention](/cli/subcommands.md#exit-code-convention).

## Cross-references

- Shim shape + `tty.go` seam: [architecture/package-layout](/architecture/package-layout.md)
- `--json` schema, `HOP_WRAPPER`, exit codes: [cli/subcommands](/cli/subcommands.md)
- The TTY guard on the fzf seams: [cli/match-resolution](/cli/match-resolution.md)
