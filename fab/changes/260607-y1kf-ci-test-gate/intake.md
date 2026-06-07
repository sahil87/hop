# Intake: CI Test Gate

**Change**: 260607-y1kf-ci-test-gate
**Created**: 2026-06-07
**Status**: Draft

## Origin

> Add a GitHub Actions CI workflow to `hop`, modeled closely on the `wt` repo's CI
> (`~/code/sahil87/wt/.github/workflows/ci.yml`), that makes passing tests a mandatory
> part of merging PRs. New file `.github/workflows/ci.yml`. Triggers: push to `main` and
> `pull_request` (all branches). A `test` job (gofmt -l, go vet, go test) and a `ci-gate`
> job that `needs: [test]` and provides the single stable required-status-check name to pin
> in GitHub branch protection. No `-race`/`-cover` initially (match wt's plain `go test ./...`).

One-shot draft (no prior `/fab-discuss` or exploratory conversation). The source material
was a fully specified description naming the `wt` CI as the model. Two repository facts were
verified at draft time rather than left open:

1. **Go module location** — confirmed `hop` keeps `go.mod` and `go.sum` under `src/`
   (`src/go.mod`, `src/go.sum`), NOT at the repo root. Verified by `find` and by inspecting
   the existing `.github/workflows/release.yml`, which already uses `go-version-file: src/go.mod`
   and `cache-dependency-path: src/go.sum`. The Go commands therefore run with
   `working-directory: src`, exactly as the `wt` model does.
2. **Action pin SHAs** — `hop`'s existing `release.yml` already pins `actions/checkout@v4`
   to `34e114876b0b11c390a56381ad16ebd13914f8d5` and `actions/setup-go@v5` to
   `40f1582b2485089dde7abd97c1529aa768e1baff`. These same SHAs are reused so the repo has a
   single internal source of truth for action pins (matches `wt`, which pins the same SHAs).

## Why

**Problem.** `hop` has a Go test suite (`just test` → `cd src && go test ./...`) and a
release pipeline, but nothing runs tests automatically on pull requests or pushes to `main`.
Tests pass or fail only when a human remembers to run them locally. There is no automated
guard preventing a PR that breaks the build, fails `go vet`, or introduces unformatted code
from being merged.

**Consequence if not fixed.** Regressions land on `main` undetected until the next local run
or — worse — until a release is cut (the release workflow does not run tests; it cross-compiles
and ships). The Constitution's **Test Integrity** constraint declares specs the source of truth
and tests the conformance check, but that contract is hollow if tests never actually run in CI.
A broken `main` also poisons every subsequent PR branch.

**Why this approach.** Mirror the `wt` repo's CI shape exactly. `wt` is the same author's
sibling Go CLI with the identical `src/`-rooted module layout and the same SHA-pinned action
conventions, so its `ci.yml` is a proven, drop-in-shaped template. This is the cheapest path
to a working test gate, and it keeps `hop`'s CI mentally aligned with `wt`'s for cross-repo
maintenance. The two-job split (`test` + `ci-gate`) gives branch protection a **single stable
required-check name** to pin: future splits or renames of the `test` job only touch `ci-gate`'s
`needs:` list, never the repo ruleset.

**Constitution alignment.**
- **Principle V (Thin Justfile, Fab-Kit Build Pattern)** — CI/release mirror the fab-kit/`wt`/run-kit
  structure. This change extends that pattern: a hand-shaped GitHub Actions workflow that mirrors
  the sibling repo, with SHA-pinned actions, consistent with the existing `release.yml`.
- **Test Integrity** — specs are the source of truth; tests verify conformance. This change makes
  that verification actually execute on every PR and push, enforcing the constraint instead of
  trusting it.

## What Changes

### New file: `.github/workflows/ci.yml`

A single new workflow file. No existing files are modified. Modeled section-for-section on
`~/code/sahil87/wt/.github/workflows/ci.yml`, adapted only where `hop` differs (which is nowhere
material — both use `src/`-rooted modules and the same action SHAs).

#### Triggers, permissions, concurrency

```yaml
name: CI

on:
  push:
    branches:
      - main
  pull_request:

permissions:
  contents: read

concurrency:
  # Cancel superseded runs on the same ref (PR branch or main).
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

- `push` restricted to `main`; `pull_request` left unfiltered (fires for PRs against any base branch).
- `permissions: contents: read` — least privilege; CI only reads the repo.
- Concurrency group keyed on `github.ref` with `cancel-in-progress: true` so a new push to a
  branch cancels its in-flight run.

#### `test` job

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4

      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          # Single source of truth for the Go version — same as release.yml.
          go-version-file: src/go.mod
          cache-dependency-path: src/go.sum

      - name: gofmt
        working-directory: src
        run: |
          # gofmt -l prints files that are NOT formatted. Any output ⇒ fail.
          unformatted="$(gofmt -l .)"
          if [ -n "$unformatted" ]; then
            echo "The following files are not gofmt-clean:" >&2
            echo "$unformatted" >&2
            echo "Run: (cd src && gofmt -w .)" >&2
            exit 1
          fi

      - name: go vet
        working-directory: src
        run: go vet ./...

      - name: go test
        working-directory: src
        run: go test ./...
```

- Actions pinned to the **same SHAs already used in `hop`'s `release.yml`** (and in `wt`'s CI),
  with `# v4` / `# v5` trailing comments.
- `setup-go` derives the Go version from `src/go.mod` (single source of truth, identical to
  `release.yml`) and caches modules keyed on `src/go.sum`.
- All Go steps run from `working-directory: src` because the module is rooted there.
- `gofmt -l .` lists unformatted files; non-empty output fails the step with a remediation hint.
- `go vet ./...` and `go test ./...` are plain — **no `-race` or `-cover` flags** initially, matching
  `wt`. These MAY be added later (see Open Questions / Impact).

#### `ci-gate` job

```yaml
  # Single stable status check to pin branch protection to. Pinning the
  # ruleset to this job (rather than `test` directly) means the required
  # check name stays constant even if `test` is later split or renamed —
  # only this job's `needs:` list has to track that, not the repo ruleset.
  ci-gate:
    needs: [test]
    runs-on: ubuntu-latest
    # Run even when a dependency failed, so the gate can fail explicitly
    # rather than being skipped (a skipped required check blocks merge in a
    # confusing way).
    if: always()
    steps:
      - name: Verify CI passed
        run: |
          if [ "${{ needs.test.result }}" != "success" ]; then
            echo "CI gate failed: test job result = ${{ needs.test.result }}" >&2
            exit 1
          fi
          echo "CI gate passed: all required jobs succeeded."
```

- `needs: [test]` + `if: always()` so the gate runs and fails **explicitly** even when `test`
  fails (a skipped required check blocks merges confusingly).
- Fails unless `needs.test.result == 'success'`. `ci-gate` is the single, stable required-status-check
  name to pin in GitHub branch protection.

### Out of scope (explicit)

- **Branch-protection enforcement is NOT in this change.** The workflow can only *provide* the
  `ci-gate` check. Marking it *required* lives in GitHub repo settings (a human via the UI, or a
  `gh api` call against `repos/sahil87/hop/branches/main/protection` / the rulesets API). This is an
  operational follow-up after merge — see Impact.
- **No `-race` / `-cover`.** Plain `go test ./...` to match `wt`. Optional future enhancement.
- **No lint job** (e.g., `golangci-lint`) — `gofmt -l` + `go vet` only, matching `wt`.
- **No release/build changes.** `release.yml` is untouched.

## Affected Memory

- `build/release-pipeline`: (modify) — this domain currently documents the tag-driven release
  workflow only. Add a short subsection (or split a sibling note) covering the new CI workflow:
  its triggers, the `test`/`ci-gate` two-job shape, the rationale for the gate job as the single
  pinned required check, and the manual branch-protection follow-up. The CI workflow is part of the
  same "GitHub Actions, mirror the sibling repo, SHA-pinned actions" story already captured there.
  (Hydrate decides final placement — a dedicated `build/ci-pipeline.md` is also reasonable.)

## Impact

- **New file**: `.github/workflows/ci.yml`. No source code (`src/`) changes; no `cmd/` or `internal/`
  changes; the build and release scripts are untouched.
- **Affected systems**: GitHub Actions only. First push of this workflow triggers a run on the PR
  itself, surfacing any pre-existing `gofmt`/`vet`/`test` failures immediately.
- **Dependencies**: `actions/checkout@v4` and `actions/setup-go@v5`, both pinned to SHAs already
  present in `release.yml`. No new third-party actions, no new Go dependencies.
- **Operational follow-up (post-merge, manual)**: configure `ci-gate` as a required status check on
  `main` via GitHub branch-protection / rulesets settings. The repo cannot self-enforce this; it must
  be done in repo settings (UI or `gh api`). Until then, the workflow runs and reports but does not
  block merges.
- **Risk / blast radius**: very low. Additive, single file, fully reversible (delete the file). The
  only behavioral surprise is that a `main` currently carrying unformatted code or a vet/test failure
  would make the first CI run red — which is the intended signal, not a defect.

## Open Questions

- Should `go test` adopt `-race` (and/or `-cover`) from the start, or stay plain to match `wt`?
  Decision recorded as Confident: match `wt` (plain) initially; revisit if flakiness or coverage
  tracking becomes a need. Not blocking.
- Final memory placement: extend `build/release-pipeline.md` vs. create `build/ci-pipeline.md`.
  Deferred to hydrate; not blocking.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Go module lives at `src/go.mod` / `src/go.sum`; all Go steps use `working-directory: src` and `go-version-file: src/go.mod`, `cache-dependency-path: src/go.sum` | Verified directly: only `go.mod`/`go.sum` in repo are under `src/`; existing `release.yml` already uses these exact paths. No ambiguity. | S:95 R:90 A:98 D:95 |
| 2 | Certain | Reuse the SHA pins from `release.yml` — `checkout@34e1148…` (v4), `setup-go@40f1582…` (v5) | The repo already pins these exact SHAs; `wt`'s CI pins the same. Single internal source of truth. Constitution/release-spec mandate SHA pinning. | S:90 R:90 A:95 D:95 |
| 3 | Certain | Workflow file path is `.github/workflows/ci.yml` | Explicitly specified; matches GitHub Actions convention and the `wt` model. | S:98 R:95 A:98 D:98 |
| 4 | Confident | Two-job shape: `test` + `ci-gate` (`needs: [test]`, `if: always()`, fail unless `needs.test.result == 'success'`) | Specified in source material and copied verbatim from `wt`. Gives branch protection a single stable check name. One obvious interpretation. | S:90 R:80 A:85 D:88 |
| 5 | Confident | Triggers: `push` to `main` + unfiltered `pull_request`; `permissions: contents: read`; concurrency `ci-${{ github.ref }}` with `cancel-in-progress: true` | Specified and matches `wt` verbatim. Standard least-privilege CI config; easily adjusted later. | S:90 R:85 A:85 D:85 |
| 6 | Confident | Test steps are `gofmt -l .` (fail on non-empty), `go vet ./...`, `go test ./...` — plain, no `-race`/`-cover` | Specified to match `wt`. Plain `go test` is the safe default; flags are an additive, reversible follow-up. | S:85 R:85 A:85 D:80 |
| 7 | Confident | Branch-protection enforcement is out of scope (manual repo-settings follow-up); workflow only provides the `ci-gate` check | Explicitly called out as out of scope. Repo files cannot configure branch protection — it lives in GitHub settings. Correct boundary. | S:90 R:75 A:90 D:85 |
| 8 | Confident | Affected memory: extend `build/release-pipeline` (or a new `build/ci-pipeline.md`) at hydrate | CI is part of the same GitHub-Actions/mirror-sibling-repo story already in that domain; placement is a low-stakes hydrate-time call. | S:80 R:80 A:80 D:70 |

8 assumptions (3 certain, 5 confident, 0 tentative, 0 unresolved).
