# Plan: Bind Constitution to sahil87 Toolkit Standards

**Change**: 260717-zono-bind-constitution-toolkit-standards
**Intake**: `intake.md`

## Requirements

### Constitution: Toolkit Standards Binding

#### R1: Toolkit Standards article
The constitution MUST carry a `### Toolkit Standards` article under the existing `## Additional Constraints` section, appended after the Cross-Platform Behavior article, using the exact article text supplied in the intake (with em-dashes, no standard names/counts/per-standard URLs beyond `shll standards`, the sahil87/shll `docs/site/standards/` tree, and https://shll.ai).

- **GIVEN** `fab/project/constitution.md` with `## Additional Constraints` holding Test Integrity and Cross-Platform Behavior
- **WHEN** the change is applied
- **THEN** a `### Toolkit Standards` article appears under `## Additional Constraints`, after Cross-Platform Behavior
- **AND** the article's only external references are `shll standards`, the sahil87/shll `docs/site/standards/` tree, and https://shll.ai — no standard names, counts, or per-standard URLs

#### R2: Governance line MINOR bump
The governance line MUST be bumped to record the amendment: version 1.1.0 → 1.2.0 (MINOR — a new article added, no existing principle changed or removed), Last Amended → 2026-07-18, Ratified unchanged (2026-05-03).

- **GIVEN** the governance line `**Version**: 1.1.0 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-05-03`
- **WHEN** the change is applied
- **THEN** the line reads `**Version**: 1.2.0 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-07-18`

### Non-Goals

- Conformance fixes — auditing or fixing hop's actual CLI surface, help output, README, or docs/site against the standards is out of scope; this change only creates the obligation.
- Any change to files other than `fab/project/constitution.md` (fab/changes/ artifacts aside).

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add the `### Toolkit Standards` article verbatim (per intake, with em-dashes) under `## Additional Constraints` after the Cross-Platform Behavior article in `fab/project/constitution.md` <!-- R1 -->
- [x] T002 Bump the governance line in `fab/project/constitution.md` to `**Version**: 1.2.0 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-07-18` <!-- R2 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: A `### Toolkit Standards` article exists under `## Additional Constraints` (after Cross-Platform Behavior) with the exact intake text, and its only external references are `shll standards`, the sahil87/shll `docs/site/standards/` tree, and https://shll.ai
- [x] A-002 R2: The governance line reads `**Version**: 1.2.0 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-07-18`

### Behavioral Correctness

- [x] A-003 R1: The article contains no standard names, counts, or per-standard URLs beyond the three permitted references

### Code Quality

- [x] A-004 Pattern consistency: The new article follows the file's existing `###` article structure and em-dash punctuation style
- [x] A-005 No unnecessary duplication: No standard enumeration is copied into the constitution — `shll standards` remains the single source of enumeration

## Notes

- Docs-only, single-file change (`fab/project/constitution.md`). No source code or tests to run — verify by re-reading the edited file.
- Check items as you review: `- [x]`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Article appended after Cross-Platform Behavior (last existing article under `## Additional Constraints`) | Intake specifies this exact placement; matches file's `###` article ordering | S:95 R:90 A:95 D:95 |
| 2 | Certain | Article text and em-dashes used verbatim from intake | Intake supplies exact text and resolves the em-dash substitution as a graded decision | S:95 R:90 A:95 D:95 |
| 3 | Confident | Version bump is MINOR (1.1.0 → 1.2.0) | New article added, no existing principle changed/removed; standard constitution-versioning convention | S:70 R:90 A:75 D:80 |

3 assumptions (2 certain, 1 confident, 0 tentative).
