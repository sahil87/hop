# Intake: Bind Constitution to sahil87 Toolkit Standards

**Change**: 260717-zono-bind-constitution-toolkit-standards
**Created**: 2026-07-18

## Origin

One-shot `/fab-new` invocation with a fully-specified task description. The user provided the exact article text, its placement, the version-bump instruction, and an explicit anti-enumeration constraint — no conversational refinement was needed.

> Task: Amend this repo's fab constitution to bind it to the sahil87 toolkit standards. This repo is part of the sahil87 toolkit. The toolkit publishes binding, producer-facing standards — CLI design principles plus mechanical contracts (machine-readable help output, README/docs-site structure, and others over time). They are canonically authored in the sahil87/shll repository's docs/site/standards/ tree, rendered on https://shll.ai, and readable offline via the `shll standards` command. This change adds a constitution article so every future pipeline run in this repo loads and enforces the obligation. (1) Add a Toolkit Standards article under Additional Constraints in fab/project/constitution.md. (2) Bump the constitution's Last Amended date (and version, per this file's own governance line). (3) Deliberate constraint: do NOT copy standard names, counts, or per-standard URLs into the constitution — `shll standards` is the enumeration, and the article must stay correct as standards evolve. Ship per this repo's normal flow (docs-type fab change → PR). Nothing else is in scope — no conformance fixes in this change.

## Why

1. **Problem**: hop is part of the sahil87 toolkit, whose binding producer-facing standards (CLI design principles, machine-readable help output, README/docs-site structure, and more over time) live canonically in the sahil87/shll repository's `docs/site/standards/` tree. Nothing in this repo's constitution references them, so fab pipeline runs in this repo have no standing obligation to check changes against those standards.
2. **Consequence if unfixed**: CLI-surface, help-output, README, and docs-site changes ship without being checked against the toolkit standards, and the tool drifts out of conformance — each drift becoming a later, larger reconciliation change.
3. **Why this approach**: the constitution is loaded by every fab skill's always-load layer (`_preamble.md` § Context Loading Layer 1), so a constitution article is the one place a standing obligation is guaranteed to reach every future pipeline run. Pointing at `shll standards` as the live enumeration (rather than copying names/counts/URLs) keeps the article evergreen — standards added or revised in shll bind this repo without further constitutional amendment.

## What Changes

### New article in `fab/project/constitution.md`

Add the following article under the existing `## Additional Constraints` section (the section already exists, holding Test Integrity and Cross-Platform Behavior; the new article is appended after Cross-Platform Behavior, matching the file's `###` article structure):

```markdown
### Toolkit Standards

This tool is part of the sahil87 toolkit and MUST conform to the toolkit's published standards. The standards are enumerated by running `shll standards` — each entry names what it governs; read one with `shll standards <name>`. Before changing the CLI surface, help output, README.md, or docs/site/, the change MUST be checked against the standards governing that surface. If shll is unavailable, the canonical sources are the sahil87/shll repository's docs/site/standards/ tree (rendered on https://shll.ai). Standards added or revised there bind this repo without further amendment to this constitution.
```

<!-- assumed: em-dashes (—) substituted for the raw "--" in the user's supplied text, matching the file's existing punctuation style -->

### Governance line bump

Current governance line:

```markdown
**Version**: 1.1.0 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-05-03
```

Becomes:

```markdown
**Version**: 1.2.0 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-07-18
```

MINOR bump (1.1.0 → 1.2.0): a new article/section is added; no existing principle is changed or removed. Ratified date is unchanged.

### Deliberate constraints (binding on apply)

- Do NOT copy standard names, counts, or per-standard URLs into the constitution. `shll standards` is the enumeration; the article must stay correct as standards evolve. The only references permitted in the article are the `shll standards` command, the sahil87/shll repo's `docs/site/standards/` tree, and https://shll.ai — exactly as in the article text above.
- No conformance fixes in this change — the article creates the obligation; auditing/fixing hop's actual CLI surface, help output, README, or docs against the standards is out of scope.

## Affected Memory

None — this change edits a fab governance file (`fab/project/constitution.md`), not source behavior. No `docs/memory/` domain (architecture, build, cli, config) documents constitution content.

## Impact

- `fab/project/constitution.md` — one new `###` article under `## Additional Constraints`, plus the governance line bump. Single-file docs change.
- No source code, tests, README, or docs/site changes.
- Downstream effect: every future fab pipeline run in this repo loads the constitution (always-load layer) and inherits the standards-check obligation for CLI-surface/help/README/docs-site changes.

## Open Questions

None.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Article text and placement used verbatim as supplied (new `### Toolkit Standards` appended under existing `## Additional Constraints`) | User provided exact text and placement; section already exists in the file | S:95 R:90 A:95 D:95 |
| 2 | Certain | No standard names, counts, or per-standard URLs copied into the constitution | Explicit user constraint, stated with rationale (article must stay evergreen) | S:95 R:85 A:95 D:95 |
| 3 | Certain | Last Amended → 2026-07-18 (today); Ratified unchanged | Standard governance-line semantics; user instructed the date bump | S:85 R:90 A:90 D:90 |
| 4 | Confident | Version bump is MINOR: 1.1.0 → 1.2.0 | Governance line records a semver version but states no bump policy; adding a new article without altering existing principles maps to MINOR under common constitution-versioning convention | S:70 R:90 A:75 D:80 |
| 5 | Confident | Em-dashes (—) substituted for the supplied "--" in the article text | The "--" reads as plain-text em-dash encoding; the constitution consistently uses — ; trivially reversible | S:60 R:90 A:80 D:75 |
| 6 | Confident | No memory updates (Affected Memory: none) | Constitution is a fab/project governance file; existing memory domains cover source behavior only | S:70 R:80 A:75 D:75 |
| 7 | Certain | Change type `docs`, shipped via normal flow (fab pipeline → PR); no conformance fixes | Explicitly stated in the task description | S:95 R:90 A:95 D:95 |

7 assumptions (4 certain, 3 confident, 0 tentative, 0 unresolved).
