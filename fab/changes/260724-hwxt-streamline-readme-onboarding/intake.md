# Intake: Streamline README Onboarding

**Change**: 260724-hwxt-streamline-readme-onboarding
**Created**: 2026-07-24

## Origin

Promptless dispatch (`/fab-proceed`-style create-intake, `{questioning-mode} = promptless-defer`) from a live conversation in which the user reviewed an onboarding analysis of `README.md` and `docs/site/install.md` and approved acting on it ("do both... run the current repo changes"). Synthesized description:

> The repo's onboarding path has three friction points. (1) Missing "reload your shell" step (highest impact): after `shll shell-setup` writes the rc file, the current shell has no `hop` function or `h` alias — a user pasting the README's 3-line install block and typing `h web` gets "command not found" (and `hop web` hits the binary's shell-only exit-2 hint). Neither README.md nor docs/site/install.md mentions opening a new terminal or `exec zsh` anywhere. (2) No first-success/verify moment: nothing after setup says "type this, you should see that". (3) Setup content spread out and duplicated: README's `## Shell integration` and `## First run` re-explain steps 2–3 of the install block in depth, nearly verbatim duplicating docs/site/install.md §2–3, ~80–110 lines below the install block. install.md is already the canonical deep dive (changes #63 "Centralize Install Docs (Policy B)" and #64 "Onboarding Docs Shell-Setup Alignment" — respect that direction).

Key decisions from the conversation: fix all three friction points; docs-only (`README.md`, `docs/site/install.md`); exact wording NOT drafted with the user — apply has latitude, biased toward brevity and the existing README voice; explicitly out of scope: the shields.io badge "grammar diagram" rows in `## The mental model`, any change to the `-h` help text (just shipped in change 3j8c / PR #65), and the shll.ai overview page (separate repo).

## Why

1. **The happy path breaks at the last step.** README's `## Install` is a 3-line paste (`curl … | sh -s -- hop`; `shll shell-setup`; `hop add -r ~/code`). `shll shell-setup` only *writes* the rc file — the running shell never sources it, so the `hop` function, `h` alias, and completions don't exist yet. A new user who types `h web` immediately after gets `command not found: h`; `hop web` hits the binary's shim-missing exit-2 hint. Neither README.md nor docs/site/install.md says "open a new terminal" or `exec zsh` anywhere (verified by reading both files in full). This is the highest-impact fix: the first thing every new user does after install currently fails silently-confusingly.
2. **No first-success moment.** Nothing after setup says "type this, you should see that". A verify step (`hop ls` → your repo list proves the config bootstrap worked; `h <partial>` → cd proves the shim works) turns "I think it's installed" into a confirmed success in two lines.
3. **Duplication drift risk + broken reading order.** README `## Shell integration` (~line 92) and `## First run` (~line 105) re-explain install steps 2–3 in depth, nearly verbatim duplicating install.md §2–4 — and they sit ~80–110 lines below the install block with `## Why hop?`, `## The mental model`, and `## Other ways to install` interleaved. Two full copies of the same explanation will drift (this repo just spent changes #63/#64 centralizing install docs precisely to stop that). Trimming the README copies to pointers keeps the README the hub (per the readme-extraction standard) and install.md the single deep dive.

If not fixed: every new user's first command fails, and the duplicated sections re-diverge from install.md over time, undoing #63/#64.

## What Changes

Docs-only. Two files: `README.md`, `docs/site/install.md`. The three fixes, in priority order:

### 1. Add the "reload your shell" step (README + install.md)

**README `## Install`** — immediately after the 3-line install block (currently lines 11–15) and its explanatory paragraph: add a reload instruction, e.g. open a new terminal or `exec zsh` (bash: `exec bash`), stating why in a clause — `shll shell-setup` edits your rc file, so the shim (the `hop` function, `h` alias, completion) only exists in shells started after it. One or two lines; can be folded into the verify step below as its first line.

**docs/site/install.md** — two touchpoints (both, briefly):
- `## TL;DR` (lines 7–15): add the reload as the natural fourth beat of the happy path (a line in or right after the code block).
- `## 2. Wire the shell shim` (§2, lines 41–51): after the `shll shell-setup` paragraph, one sentence noting the wiring takes effect in new shells — reload (`exec zsh`) or open a new terminal. The from-source manual-eval subsection has the same property (an rc-file edit doesn't affect the running shell) — a single well-placed sentence covering §2 as a whole is fine.

### 2. Add a first-success/verify moment (README, near the install block)

Right after the reload step in README `## Install`: a tight "you should see" beat — a couple of lines, not a new tour. Shape (apply drafts final wording):

```sh
hop ls          # lists every repo from hop.yaml — proves the config bootstrap worked
h <partial>     # e.g. `h web` — substring-match and cd straight into a repo — proves the shim works
```

`hop ls` proves the config bootstrap (`hop add -r`) worked; `h <partial-name>` proves the shim works. Mirror in install.md only where it reads naturally (e.g. a line in `## Next steps` or after §3) — README is the primary site for this beat; do not build a parallel tour in install.md.

### 3. Trim README's duplicated setup sections to pointers

README `## Shell integration` (lines 92–103) and `## First run` (lines 105–118) shrink to a short paragraph each — or merge into one short section (apply's choice) — that states the one-line essence and points to `docs/site/install.md` for depth (link written naturally as `docs/site/install.md` per the readme-extraction standard).

**Content-parity map** (verified at intake by reading both files in full — every fact in the two README sections already exists in install.md):

| README content | install.md location |
|---|---|
| `eval "$(hop shell-init zsh/bash)"` rc lines | §2 "Installed from source? Wire the shim manually" |
| Shim installs `hop` function + `h` alias + tab completion | §2 "What the shim installs" |
| Dispatch model (binary classifies: cd / run-in-parent / pass-through) | §2 "How dispatch works (why the shim is safe)" |
| `shll shell-setup` handles all shll tools at once, idempotent | §2 opening paragraphs |
| `hop add -r ~/code` / `-p` preview / auto-creates config | §3 "First run: bootstrap `hop.yaml` from disk" |
| Depth 3 default, `--depth N`, group auto-derivation, `-g`, skip rules | §3 |
| `hop config init` annotated starter | §3 |
| Config path `~/.config/hop/hop.yaml`, `hop config where`, dotfiles symlink pattern | §4 "Where `hop.yaml` lives, and syncing it across machines" |

**Hard rule**: do not delete information that exists ONLY in the README — apply MUST re-verify this map line-by-line before trimming; anything found to be unique moves INTO install.md rather than vanishing. (The intake-time read found nothing unique; the only README-specific element is the internal `[Gotchas](#gotchas)` cross-link, which the trimmed text may keep or drop.) Note the trimmed section(s) are also link targets of nothing external (checked: no other file links to `README.md#shell-integration` or `#first-run`), but README-internal references (e.g. `## Gotchas` mentions the shim) should still read coherently after the trim.

### Standards conformance (binds all edits)

Constitution § Toolkit Standards binds README.md and docs/site/. Per `shll standards readme-extraction` (read in full at intake):

- README head/tail structure untouched (H1 → toolkit blockquote → badges → prose; no footer headings introduced above content).
- Links from README into the published set written naturally as `docs/site/<path>.md`; links to anything outside the published set (source, `docs/specs/`) absolute `https://…`.
- No relative images; no `docs/site/` links behind badges or reference-style definitions.
- README stays the hub: keeps cross-links to its `docs/site/` pages and the absolute commands URL `https://shll.ai/hop/commands/` (the existing `## Reference` section already does this — don't break it).
- install.md stays inside the closed `docs/site/` set: relative links only to other `docs/site/` pages; everything else absolute.

## Affected Memory

None — docs-only change to `README.md` and `docs/site/install.md` prose; no spec-level behavior changes. (Established convention: no memory file governs README/docs-site prose — see changes of6z and b6m5.)

## Impact

- **Files**: `README.md` (add ~4–6 lines near `## Install`; shrink `## Shell integration` + `## First run` from ~27 lines to ~4–8), `docs/site/install.md` (add ~3–4 lines across TL;DR and §2, possibly 1 line in Next steps). Net README shrinks; install.md grows slightly.
- **No Go code, no tests, no CI.** `change_type: docs`.
- **Verification**: markdown-only — the readme-extraction "Verifying conformance" bullets: grep for relative targets (`](./`, `](../`, `](docs/`) and confirm each points into `docs/site/` (from README) or stays inside `docs/site/` (between tree pages) or is absolute; no relative images; no `#gh-*-mode-only`; README hub links intact. No test suite applies.
- **Risk**: low — reversible prose edits on a branch; the only real hazard is deleting README-unique content, guarded by the hard rule + parity map above.

## Open Questions

None — direction fixed by the approved analysis; wording latitude explicitly delegated to apply.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Docs-only scope: `README.md` + `docs/site/install.md`, `change_type: docs`, no code/tests | Explicit constraint in the approved description | S:95 R:90 A:95 D:95 |
| 2 | Certain | The three approved fixes (reload step, verify moment, dedup-trim) are the change, in that priority order | Discussed — user reviewed the analysis and approved "do both... run the current repo changes" | S:90 R:85 A:90 D:90 |
| 3 | Certain | Out of scope: badge "grammar diagram" rows, `-h` help text (change 3j8c / PR #65), shll.ai overview page | Explicitly enumerated as offered-but-not-opted-into | S:95 R:95 A:95 D:95 |
| 4 | Certain | All edits conform to `shll standards readme-extraction` (natural `docs/site/<path>.md` links, absolute external links, README-as-hub, head/tail structure) | Constitution § Toolkit Standards binds these surfaces; standard read in full at intake | S:90 R:85 A:95 D:90 |
| 5 | Certain | Verification is markdown-only conformance checking (the standard's bullets, relative-link greps); no test suite runs | Explicit constraint in the approved description | S:90 R:90 A:90 D:90 |
| 6 | Confident | Exact wording is drafted at apply, biased toward brevity and the existing README voice | Latitude explicitly delegated ("the apply stage has latitude on wording") | S:80 R:85 A:75 D:70 |
| 7 | Confident | The description's "TL;DR and/or §2" for install.md resolves to BOTH: a reload beat in `## TL;DR` and one sentence in `## 2. Wire the shell shim` | Both touchpoints are cheap one-liners; the TL;DR is the happy path (where the break lives) and §2 is the deep dive — trivially reversible prose | S:70 R:85 A:80 D:65 |
| 8 | Confident | Verify step lives primarily in README `## Install`; install.md mirrors it only where natural (no parallel tour) | Description says "near the install block… keep it tight"; install.md already has a Next-steps close | S:70 R:85 A:75 D:65 |
| 9 | Confident | Trim shape (one merged short section vs. two short paragraphs) is apply's choice | Description explicitly offers both ("or merge into one short section") | S:70 R:80 A:80 D:60 |
| 10 | Confident | No README-unique content exists in the two trimmed sections (per the intake-time parity map); apply re-verifies line-by-line and moves anything unique into install.md instead of deleting | Intake-time full read of both files found complete coverage in install.md §2–4; hard no-information-loss rule recorded in What Changes | S:75 R:70 A:85 D:75 |

10 assumptions (5 certain, 5 confident, 0 tentative, 0 unresolved).
