---
description: "push/PR GitHub Actions test gate (`ci.yml`); `test` job (gofmt/vet/test from `src/`) + `ci-gate` single required-check job; byte-identical to `wt`; branch-protection is a manual repo-settings follow-up"
type: memory
---
# CI Pipeline

How `hop` gates merges on passing tests. A GitHub Actions workflow at `.github/workflows/ci.yml` (name `CI`) that runs `gofmt`, `go vet`, and `go test` on every push to `main` and every pull request. Distinct from the [release-pipeline](/build/release-pipeline.md): that fires on `v*` tags and ships binaries; this fires on pushes/PRs and only reads the repo to verify it builds clean.

The workflow is byte-identical to the sibling `wt` repo's `ci.yml` (verified at apply time via `diff`), the same way [release-pipeline](/build/release-pipeline.md) mirrors run-kit's release workflow. `wt` is the same author's sibling Go CLI with the identical `src/`-rooted module layout and the same SHA-pinned action conventions, so its `ci.yml` is a proven drop-in template. Keeping the two byte-identical lets a single diff update both repos if the workflow ever needs to change.

## Triggers, permissions, concurrency

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
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

- `push` is restricted to `main`; `pull_request` is **unfiltered**, so it fires for PRs against any base branch.
- `permissions: contents: read` — least privilege. CI only reads the repo; it never writes (contrast with `release.yml`, which needs `contents: write`).
- Concurrency group is keyed on `github.ref` with `cancel-in-progress: true`, so a new push to a branch (or `main`) cancels its own in-flight run.

## `test` job

Single job on `ubuntu-latest`. Checks out the repo, provisions Go, then runs three checks — all from `working-directory: src` because the Go module is rooted there (`src/go.mod`, `src/go.sum`), not at the repo root.

1. **Checkout** — `actions/checkout` (no `fetch-depth`; a shallow clone suffices, unlike `release.yml` which needs `fetch-depth: 0` for tag-base computation).
2. **Setup Go** — `actions/setup-go` with `go-version-file: src/go.mod` (single source of truth for the Go version, identical to `release.yml`) and `cache-dependency-path: src/go.sum` (module cache key).
3. **gofmt** — runs `gofmt -l .`; any non-empty output (a list of unformatted files) fails the step with a remediation hint (`Run: (cd src && gofmt -w .)`). `gofmt -l` only lists, never rewrites.
4. **go vet** — `go vet ./...`.
5. **go test** — `go test ./...`, **plain**: no `-race`, no `-cover`. This matches `wt`. Race/coverage flags are an additive, reversible follow-up if flakiness or coverage tracking ever becomes a need; not adopted initially.

## `ci-gate` job

```yaml
ci-gate:
  needs: [test]
  runs-on: ubuntu-latest
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

`ci-gate` is the **single stable required-status-check name** to pin in GitHub branch protection. It `needs: [test]`, runs with `if: always()` (so it runs even when `test` fails), and fails unless `needs.test.result == 'success'`.

- **Why `if: always()`** — a required check that gets *skipped* (which a downstream job does by default when its dependency fails) blocks merges in a confusing way. Running unconditionally lets the gate fail *explicitly*, echoing the failing `test` result.
- **Why a separate gate job rather than pinning `test` directly** — the required-check name pinned in the repo ruleset stays constant even if `test` is later split (e.g., into `lint` + `test`) or renamed. Future restructuring touches only this job's `needs:` list, never the GitHub branch-protection ruleset. Pinning `test` directly would be brittle to renames.

## Action SHAs

Both actions are SHA-pinned with `# v<N>` trailing comments, reusing **the exact SHAs already pinned in `release.yml`** — a single internal source of truth for action pins within the repo:

| Action | SHA | Tag |
|---|---|---|
| `actions/checkout` | `34e114876b0b11c390a56381ad16ebd13914f8d5` | v4 |
| `actions/setup-go` | `40f1582b2485089dde7abd97c1529aa768e1baff` | v5 |

`wt`'s `ci.yml` pins the same SHAs. See [release-pipeline § Action SHAs](/build/release-pipeline.md#action-shas) for the release workflow's full pin table and the lockstep policy.

## Branch-protection enforcement (manual, post-merge)

**The workflow only *provides* the `ci-gate` check — it does not *enforce* it.** Marking `ci-gate` as a required status check on `main` lives in GitHub repo settings, not in any repo file. Until that is configured, the workflow runs and reports status on PRs but **does not block merges**.

This must be done by a maintainer, either via the GitHub UI (Settings → Branches → branch protection / rulesets) or `gh api`:

```sh
gh api repos/sahil87/hop/branches/main/protection ...
# or the rulesets API
```

Repo files cannot self-configure branch protection, so this is an operational follow-up that the change introducing the workflow could not perform.

## Out of scope

Policy decisions, not deferrals (all matching `wt`):

- **No `-race` / `-cover`** — plain `go test ./...`. Additive future option if needed.
- **No lint job** (e.g., `golangci-lint`) — `gofmt -l` + `go vet` are the only static checks.
- **No release/build changes** — `release.yml` and the build scripts are untouched; CI is a separate, additive workflow.

## Cross-references

- `docs/memory/build/release-pipeline.md` — the tag-driven release workflow; shares the same SHA-pinned action conventions and `src/`-rooted Go setup.
- `docs/memory/build/local.md` — `just test` (`cd src && go test ./...`), the local equivalent of the CI `test` job.
- `docs/specs/build-and-release.md` — pre-implementation design intent for the build/release surface.
