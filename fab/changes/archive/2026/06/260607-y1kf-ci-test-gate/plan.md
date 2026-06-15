# Plan: CI Test Gate

**Change**: 260607-y1kf-ci-test-gate
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### CI: Workflow File & Triggers

#### R1: CI workflow exists at the conventional path
A GitHub Actions workflow file SHALL exist at `.github/workflows/ci.yml` named `CI`. It is a new
file; no existing workflow (`release.yml`) is modified.

- **GIVEN** the repo has no CI workflow
- **WHEN** this change is applied
- **THEN** `.github/workflows/ci.yml` exists, is valid YAML, and declares `name: CI`

#### R2: Triggers, permissions, and concurrency
The workflow MUST trigger on `push` to `main` and on `pull_request` (unfiltered, so it fires for PRs
against any base branch). It MUST declare least-privilege `permissions: contents: read`. It MUST
declare a concurrency group `ci-${{ github.ref }}` with `cancel-in-progress: true`.

- **GIVEN** a push to `main` or a pull request is opened/updated
- **WHEN** GitHub Actions evaluates the workflow triggers
- **THEN** the `CI` workflow runs
- **AND** only `contents: read` permission is granted
- **AND** a superseded in-flight run on the same ref is cancelled

### CI: Test Job

#### R3: `test` job runs gofmt, vet, and tests from `src/`
The workflow MUST define a `test` job on `ubuntu-latest` that checks out the repo, sets up Go from
`src/go.mod` (caching on `src/go.sum`), then runs three checks from `working-directory: src`:
`gofmt -l .` (failing on any non-empty output, with a remediation hint), `go vet ./...`, and
`go test ./...` (plain — no `-race` / `-cover`). Actions MUST be SHA-pinned to the same SHAs used by
`release.yml`: `actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4` and
`actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5`.

- **GIVEN** a CI run is triggered
- **WHEN** the `test` job executes
- **THEN** Go is provisioned from `src/go.mod`
- **AND** unformatted files fail the `gofmt` step with a non-empty `gofmt -l .` list
- **AND** `go vet ./...` and `go test ./...` run from `src/`
- **AND** any vet or test failure fails the job

### CI: Gate Job

#### R4: `ci-gate` provides a single stable required-check name
The workflow MUST define a `ci-gate` job that `needs: [test]`, runs with `if: always()` on
`ubuntu-latest`, and fails unless `needs.test.result == 'success'` (echoing the failing result on
failure). This is the single, stable status-check name intended to be pinned in GitHub branch
protection so that future renames/splits of `test` touch only `ci-gate`'s `needs:` list.

- **GIVEN** the `test` job completes (success or failure)
- **WHEN** the `ci-gate` job runs (it always runs, via `if: always()`)
- **THEN** `ci-gate` succeeds only when `needs.test.result == 'success'`
- **AND** `ci-gate` fails explicitly (non-skipped) when `test` failed

### Non-Goals

- Branch-protection enforcement — marking `ci-gate` required lives in GitHub repo settings; it is a
  manual post-merge follow-up, not a repo file.
- `-race` / `-cover` flags on `go test` — plain `go test ./...` to match `wt`; an additive future option.
- A lint job (e.g., `golangci-lint`) — `gofmt -l` + `go vet` only, matching `wt`.
- Any change to `release.yml` or the build/release scripts.
- Any *behavioral* `src/` code change. (One whitespace-only `gofmt -w` fix to
  `src/internal/scan/scan_test.go` IS included — see Design Decision 4 — so the new gate is green
  on first run; no logic is touched.)

### Design Decisions

1. **Mirror `wt`'s `ci.yml` verbatim**: `~/code/sahil87/wt/.github/workflows/ci.yml` is byte-identical
   to the intake's proposed YAML — same `src/`-rooted module layout, same action SHAs. — *Why*: cheapest
   proven template; keeps `hop` and `wt` CI mentally aligned. — *Rejected*: hand-rolling a bespoke
   workflow (more risk, no benefit).
2. **Two-job split (`test` + `ci-gate`)**: branch protection pins the stable `ci-gate` name. — *Why*:
   future `test` renames/splits only touch `ci-gate`'s `needs:`, never the repo ruleset. — *Rejected*:
   pinning `test` directly (brittle to renames).
3. **Reuse `release.yml` action SHAs**: single internal source of truth for pins. — *Why*: Constitution
   Principle V + release-spec mandate SHA pinning; matches `wt`. — *Rejected*: floating tags (`@v4`).
4. **Include the `gofmt -w` fix for `src/internal/scan/scan_test.go` in this PR**: `main` currently
   carries a whitespace/column-alignment nit in two map literals, which the new `gofmt` step would
   flag — making the gate red on its very first run. — *Why*: a CI gate that is red on arrival is
   confusing and undermines the change's purpose; the fix is whitespace-only (no logic, no behavior),
   so it does not violate Test Integrity (which forbids changing *implementation* to suit test
   infra — this is neither). User-confirmed during apply. — *Rejected*: (a) ship CI-only and let the
   gate go red as an "honest signal" (confusing first impression); (b) land the format fix as a
   separate prior PR (more ceremony for a two-line whitespace diff).

## Tasks

### Phase 1: Implementation

- [x] T001 Create `.github/workflows/ci.yml` with `name: CI`, the `on:` block (`push` → `main`, unfiltered `pull_request`), `permissions: contents: read`, and the `concurrency` block (`group: ci-${{ github.ref }}`, `cancel-in-progress: true`) <!-- R1 R2 -->
- [x] T002 Add the `test` job to `.github/workflows/ci.yml`: `ubuntu-latest`; checkout + setup-go SHA-pinned to the `release.yml` SHAs (`go-version-file: src/go.mod`, `cache-dependency-path: src/go.sum`); `gofmt` / `go vet` / `go test` steps all `working-directory: src`, plain `go test ./...` <!-- R3 -->
- [x] T003 Add the `ci-gate` job to `.github/workflows/ci.yml`: `needs: [test]`, `if: always()`, `ubuntu-latest`, single step failing unless `needs.test.result == 'success'` <!-- R4 -->

### Phase 2: Verification

- [x] T004 Validate `.github/workflows/ci.yml` parses as valid YAML and structurally contains all required keys (name, on, permissions, concurrency, both jobs, all steps) <!-- R1 R2 R3 R4 -->
- [x] T005 Confirm the action SHAs in `ci.yml` exactly match those in `.github/workflows/release.yml` <!-- R3 -->
- [x] T006 Apply `gofmt -w` to `src/internal/scan/scan_test.go` (whitespace-only map-literal realignment) so `gofmt -l .` is clean and the new gate is green on first run; re-verify `gofmt -l . && go vet ./... && go test ./...` all pass from `src/` <!-- R3 -->

## Execution Order

- T001 → T002 → T003 build up the single file in order (one file, written whole).
- T004 and T005 verify after the file is complete.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `.github/workflows/ci.yml` exists, is valid YAML, and declares `name: CI`
- [x] A-002 R2: Workflow triggers on `push` to `main` and unfiltered `pull_request`; declares `permissions: contents: read`; declares concurrency `group: ci-${{ github.ref }}` with `cancel-in-progress: true`
- [x] A-003 R3: `test` job on `ubuntu-latest` checks out, sets up Go from `src/go.mod` (cache `src/go.sum`), and runs `gofmt -l .`, `go vet ./...`, `go test ./...` all from `working-directory: src`
- [x] A-004 R4: `ci-gate` job has `needs: [test]`, `if: always()`, and fails unless `needs.test.result == 'success'`

### Behavioral Correctness

- [x] A-005 R3: The `gofmt` step fails on non-empty `gofmt -l .` output and prints a remediation hint; `go test` is plain (no `-race` / `-cover`)
- [x] A-006 R4: `ci-gate` runs even when `test` fails (not skipped) and fails explicitly with the failing result echoed

### Scenario Coverage

- [x] A-007 R3: Action SHAs in `ci.yml` exactly match `release.yml` (`checkout@34e1148…` v4, `setup-go@40f1582…` v5)

### Code Quality

- [x] A-008 Pattern consistency: `ci.yml` mirrors `release.yml` / `wt`'s `ci.yml` conventions (SHA-pinned actions with `# v#` comments, `src/`-rooted Go steps)
- [x] A-009 No unnecessary duplication: Action pin SHAs reuse `release.yml`'s values; Go version derived from `src/go.mod` rather than hardcoded

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- `main` carried one pre-existing `gofmt` nit (`src/internal/scan/scan_test.go`, whitespace only).
  Per the user's decision at apply, this PR includes the `gofmt -w` fix (T006) so the gate is green
  on first run. This is a whitespace-only change with no logic impact, so it does not conflict with
  Test Integrity (which forbids changing *implementation* to suit test infra — formatting is neither).
  `go vet` and `go test` were already clean.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Go module at `src/go.mod` / `src/go.sum`; all Go steps use `working-directory: src`, `go-version-file: src/go.mod`, `cache-dependency-path: src/go.sum` | Verified: only `go.mod`/`go.sum` in repo are under `src/`; `release.yml` already uses these exact paths. | S:95 R:90 A:98 D:95 |
| 2 | Certain | Reuse SHA pins from `release.yml` — `checkout@34e1148…` (v4), `setup-go@40f1582…` (v5) | Confirmed by reading `release.yml`; `wt`'s CI pins the same. Single internal source of truth. | S:90 R:90 A:95 D:95 |
| 3 | Certain | Workflow file path is `.github/workflows/ci.yml`, name `CI` | Explicitly specified; GitHub Actions convention; matches `wt`. | S:98 R:95 A:98 D:98 |
| 4 | Confident | Two-job shape: `test` + `ci-gate` (`needs: [test]`, `if: always()`, fail unless `needs.test.result == 'success'`) | Specified and byte-identical to `wt`'s `ci.yml` (verified). Single stable required-check name. | S:90 R:80 A:85 D:88 |
| 5 | Confident | Triggers: `push`→`main` + unfiltered `pull_request`; `permissions: contents: read`; concurrency `ci-${{ github.ref }}` with `cancel-in-progress: true` | Specified and matches `wt` verbatim. Standard least-privilege CI config. | S:90 R:85 A:85 D:85 |
| 6 | Confident | Test steps `gofmt -l .` (fail on non-empty), `go vet ./...`, `go test ./...` — plain, no `-race`/`-cover` | Specified to match `wt`. Plain `go test` is the safe default; flags are additive follow-up. | S:85 R:85 A:85 D:80 |
| 7 | Confident | Branch-protection enforcement out of scope (manual repo-settings follow-up); workflow only provides `ci-gate` | Explicitly out of scope. Repo files cannot configure branch protection. | S:90 R:75 A:90 D:85 |
| 8 | Confident | Affected memory: extend `build/release-pipeline` (or new `build/ci-pipeline.md`) at hydrate | CI is part of the same GitHub-Actions/mirror-sibling story; placement is a low-stakes hydrate-time call. | S:80 R:80 A:80 D:70 |

8 assumptions (3 certain, 5 confident, 0 tentative, 0 unresolved).
