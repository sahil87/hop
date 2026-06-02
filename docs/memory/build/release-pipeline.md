# Release Pipeline

How `hop` cuts a release. Hand-rolled GitHub Actions workflow mirroring `~/code/sahil87/run-kit`'s shape, with a tag-driven version source (no `VERSION` file — `hop` is single-binary, so the git tag itself is the source of truth).

## Trigger

A release is triggered by pushing a tag matching `v*` to the origin remote. In practice this happens via:

```
just release [patch|minor|major]   # default: patch
```

which delegates to `scripts/release.sh`. That script computes the next tag, creates it locally, and pushes it. The push fires `.github/workflows/release.yml`, and CI takes over.

There are no other release triggers — no `workflow_dispatch`, no branch-push, no schedule.

## `scripts/release.sh`

Computes the next semver tag from the current latest tag and pushes it. It does **not** modify any tracked files (no `VERSION` file write, no commit step) — this is a deliberate divergence from run-kit, which uses a `VERSION` file because it's a multi-binary monorepo.

Behavior:

- Accepts exactly one of `patch | minor | major` (or `-h`/`--help`). Multiple bump types or unknown values exit 1 with a usage message. Bare invocation prints usage and exits 0.
- Pre-flight: rejects dirty working tree (`git status --porcelain` non-empty) and detached HEAD. Both exit 1.
- Computes current tag via `git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"`. The `v0.0.0` fallback handles the first-release case (no tags yet) — `release.sh patch` produces `v0.0.1`, `minor` produces `v0.1.0`, `major` produces `v1.0.0`.
- Bump arithmetic:

  ```sh
  case "$bump_type" in
    patch) patch=$((patch + 1)) ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    major) major=$((major + 1)); minor=0; patch=0 ;;
  esac
  ```

- Creates the tag with `git tag "$new_tag"` and pushes with `git push origin "$new_tag"`.
- No `--force` flag, no main-branch check — releases can happen from any branch (mirrors run-kit; intentional flexibility for hotfix flows).

## Workflow steps (`.github/workflows/release.yml`)

Single job (`release`) on `ubuntu-latest`, `permissions: contents: write` (no other scopes), eight steps:

1. **Checkout** with `fetch-depth: 0` — needed for the previous-tag-base computation.
2. **Setup Go** with `go-version-file: src/go.mod` — keeps the CI Go version in lockstep with `go.mod`.
3. **Extract version from tag** — sets two outputs from `${GITHUB_REF#refs/tags/}`:
   - `tag` (with `v` prefix, e.g. `v0.0.1`) — used for ldflags injection.
   - `version` (without prefix, e.g. `0.0.1`) — used for `sed` substitution into the formula.
4. **Cross-compile** — loops over `darwin/arm64 darwin/amd64 linux/arm64 linux/amd64`, building with `CGO_ENABLED=0` and `-ldflags "-X main.version=${tag}"`. Each binary is tarred via `tar -czf "dist/${output}.tar.gz" -C "dist/${output}" hop` — archives contain only the `hop` binary (no LICENSE/README inside).
5. **Determine release notes base tag** — minor-aware logic: if the patch component is `0` (minor bump), `base_tag` is set to the earliest tag matching `v{major}.{minor-1}.*` (sorted by `version:refname`, head -1), so v0.2.0's notes span the entire 0.1.x series. For patch bumps and major bumps, `base_tag` is left unset (default behavior: compare against the immediate previous tag).
6. **Create GitHub Release** via `softprops/action-gh-release` with `files: dist/*.tar.gz`, `generate_release_notes: true`, and `previous_tag: ${{ steps.release-base.outputs.base_tag }}`.
7. **Update Homebrew tap** — see Formula template below.
8. **Dump help tree and PR to shll.ai** — see [help reference publish step](#help-reference-publish-step-shllai) below. Runs **last** and is `continue-on-error: true` so a downstream-publish hiccup can never fail the release after the binaries + formula are out.

## Action SHAs

All third-party actions are pinned to commit SHAs with `# v<N>` comments:

| Action | SHA | Tag |
|---|---|---|
| `actions/checkout` | `34e114876b0b11c390a56381ad16ebd13914f8d5` | v4 |
| `actions/setup-go` | `40f1582b2485089dde7abd97c1529aa768e1baff` | v5 |
| `softprops/action-gh-release` | `153bb8e04406b158c6c84fc1615b65b24149a1fe` | v2 |

**Policy**: SHAs match `~/code/sahil87/run-kit/.github/workflows/release.yml` at apply time. Deviations need explicit justification — the lockstep keeps both repos updateable via a single-source diff if a third-party action ever needs bumping.

## Help reference publish step (shll.ai)

Added by change `jr5f`. Runs **last** in the job (after Create Release + Update Homebrew), on every `v*` tag, and is marked `continue-on-error: true`. It publishes hop's CLI help tree to the shll.ai landing site, which renders an expandable "Command reference" on each tool's page from a per-tool `help/<tool>.json` artifact. This is hop's slice of a 7-tool rollout that all publish the same JSON shape; the shll.ai site-side consumer (Astro loader + reference UI) is tracked separately in the `sahil87/shll.ai` repo. The step mirrors the canonical rollout pattern in `sahil87/idea`'s `release.yml`.

> **Ordering + best-effort are load-bearing.** The step originally ran *before* the release/homebrew steps and ended with `gh pr merge --auto --squash`. On the `v0.1.10` tag that merge call failed with `Auto merge is not allowed for this repository` (shll.ai has `allow_auto_merge: false`), `set -e` aborted the step, and because it ran first the **GitHub Release and Homebrew tap never published** — the release was blocked by a downstream-docs nicety. Fix: move it last + `continue-on-error: true` + drop the merge call (merging is owned by shll.ai, see below).

The step (`set -euo pipefail`, `env: GH_TOKEN: ${{ secrets.SHLLAI_TOKEN }}`) does:

1. **Dumps** the help tree from the freshly built, version-stamped linux-amd64 binary into `help/hop.json` (after `mkdir -p help`): `./dist/hop-linux-amd64/hop help-dump > /tmp/hop.raw.json`. The producer leaves `captured_at` empty (see [cli/subcommands § help-dump contract](../cli/subcommands.md#hop-help-dump--json-help-tree-contract)). Running the just-built binary guarantees the captured `version` matches the released tag.
2. **Injects `captured_at`** as a date-floored UTC value: `captured_at=$(date -u +%Y-%m-%dT00:00:00Z)` then `jq --arg t "$captured_at" '.captured_at=$t'` → `help/hop.json`. Date-floored (`00:00:00Z`) keeps the dump deterministic per day.
3. **Validates** with `jq -e '.tool=="hop" and .schema_version==1 and (.root|type=="object") and (.captured_at|test("Z$"))'`. The authoritative schema gate lives in shll.ai's `validate-help.mjs`.
4. **No-op guard**: clones shll.ai, then `diff`s the new `help/hop.json` against the existing one with the `captured_at` line stripped (`strip_captured_at() { grep -v '"captured_at"' "$1"; }`). If only `captured_at` differs, it `exit 0`s without opening a PR — so re-running a release doesn't spam content-identical PRs downstream.
5. **Opens a PR** into `sahil87/shll.ai` (never a direct push): branch `help-dump/hop-${version}` (bare version, no `v` prefix), stages **only** `help/hop.json` (content guard), commits **authored as `sahil87` / `sahil@noon.design`** (to satisfy shll.ai's `TRUSTED_AUTHOR` actor guard), pushes, then `gh pr create`. **No `gh pr merge`** — merging is owned by shll.ai's `help-automerge.yml`, which merges the PR (as `github-actions`) once its actor/content/schema guards pass.

**Why PR, not push-to-main**: when all 7 tools publish around the same time, concurrent direct pushes to `shll.ai` `main` would race. A per-tool branch + shll.ai-side automerge serializes the merges. The tag-scoped branch name (`help-dump/hop-${version}`) plus the no-op guard provide idempotency for re-running the same tag.

**Secret**: uses the repo secret `SHLLAI_TOKEN` (distinct from `HOMEBREW_TAP_TOKEN`), exposed to the step as `GH_TOKEN` (so `gh` and the clone URL both pick it up) and which must grant `contents:write` + `pull-requests:write` on `sahil87/shll.ai`. Set as a GitHub repository secret on `sahil87/hop`. **Security**: the producer is pure Go (no subprocess); the CI step shells out to `git`/`gh`/`jq` only with the trusted tag-derived `version`, never untrusted user input (Constitution Principle I).

**Local equivalent**: `just help-dump` → `scripts/help-dump.sh` builds hop and pretty-prints the dump via `jq` for inspection (no publish). See [build/local](local.md).

## Formula template (`.github/formula-template.rb`)

A syntactically valid Homebrew Formula Ruby file with five placeholders that the workflow's tap-update step replaces via `sed`:

| Placeholder | Replacement |
|---|---|
| `VERSION_PLACEHOLDER` | bare version (no `v` prefix), e.g. `0.0.1` |
| `SHA_DARWIN_ARM64` | `sha256sum dist/hop-darwin-arm64.tar.gz` |
| `SHA_DARWIN_AMD64` | `sha256sum dist/hop-darwin-amd64.tar.gz` |
| `SHA_LINUX_ARM64`  | `sha256sum dist/hop-linux-arm64.tar.gz` |
| `SHA_LINUX_AMD64`  | `sha256sum dist/hop-linux-amd64.tar.gz` |

The substituted file is written to `Formula/hop.rb` in a clone of `sahil87/homebrew-tap`. The clone uses `https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/sahil87/homebrew-tap.git`. The commit is authored as `github-actions[bot]` with message `hop v<version>` and pushed directly to the tap's default branch (no PR).

The published formula's structure:

- `class Hop < Formula` opener.
- `desc`, `homepage`, `version`, `license "MIT"` (informational — brew does not enforce).
- `depends_on "sahil87/tap/wt"` — runtime dependency on the `wt` worktree CLI. `hop <name> open` shells out to `wt open` to delegate app detection, menu selection, and the "Open here" cd round-trip (see [`architecture/wrapper-boundaries`](../architecture/wrapper-boundaries.md#wt-env-contract-cmdhopopengo) for the env contract). Declared in the template so `brew install sahil87/tap/hop` pulls wt automatically. `Formula/wt.rb` already lives in `sahil87/homebrew-tap` alongside hop, fab-kit, rk, tu, and idea — no separate tap-side work was needed.
- `on_macos` block with nested `on_arm` / `on_intel` blocks declaring `url` and `sha256` for the two darwin tar.gz files.
- `on_linux` block with the same shape for the two linux tar.gz files.
- URLs follow `https://github.com/sahil87/hop/releases/download/v#{version}/hop-{os}-{arch}.tar.gz` — note the `v` prefix is re-added in the URL, so `version "VERSION_PLACEHOLDER"` stores the bare form.
- `install` block: `bin.install "hop"`.
- `test` block: `assert_match version.to_s, shell_output("#{bin}/hop --version")`.

## Setup checklist

One-time setup per repo:

1. **Provision `HOMEBREW_TAP_TOKEN`** as a GitHub repository secret on `sahil87/hop`. It must be a fine-grained Personal Access Token with `Contents: write` permission scoped to `sahil87/homebrew-tap`. This step is manual (GitHub UI) and cannot be automated.
2. **Provision `SHLLAI_TOKEN`** as a GitHub repository secret on `sahil87/hop`. It must grant `Contents: write` + `Pull requests: write` scoped to `sahil87/shll.ai` (the help-reference publish step opens a PR). Manual (GitHub UI). If missing/invalid, the publish step fails on `git clone` with an auth error — but since it is `continue-on-error: true` and runs last, the release + homebrew steps still publish; only the downstream command-reference PR is skipped.
3. **Verify the tap repo** — `sahil87/homebrew-tap` must exist and the bot must have push access via the token. The `Formula/` directory already exists (it hosts `Formula/rk.rb` for run-kit).

## Release-day runbook

1. `just release [patch|minor|major]` (default `patch`) on a clean working tree, on a branch.
2. Watch the workflow at `https://github.com/sahil87/hop/actions`.
3. Verify the GitHub Release page shows four `hop-{os}-{arch}.tar.gz` assets (no separate `checksums.txt` is published).
4. Verify `sahil87/homebrew-tap` got a new commit adding/updating `Formula/hop.rb` authored by `github-actions[bot]`.
5. Smoke test in a clean shell: `brew install sahil87/tap/hop && hop --version` should print `hop version v<version>`.
6. Verify a `help-dump/hop-<version>` PR opened on `sahil87/shll.ai` and was merged by `github-actions` (via shll.ai's `help-automerge.yml`). The publish step is `continue-on-error: true`, so if it shows red, the release itself is still complete — re-check the PR/automerge guards on the shll.ai side.

If `HOMEBREW_TAP_TOKEN` is missing or invalid, the tap-update step fails on `git clone` with an auth error. The GitHub Release (created in the prior step) remains published — re-running typically means provisioning the secret and tagging again (e.g., `v0.0.2`).

## Out of scope

These are policy decisions, not deferrals:

- **Code signing / notarization** — binaries ship unsigned. macOS users see a Gatekeeper warning on first run for direct downloads; brew installs typically don't trip it as hard. An Apple Developer account ($99/yr) is not justified for personal-tooling CLIs.
- **Linux native packaging** (`.deb`, `.rpm`, custom apt/dnf repos) — Linux users install via brew-on-Linux or direct tar.gz download.
- **Prerelease tags** (`v0.0.1-rc.1`) — `release.sh` accepts only `patch|minor|major`. Adding RC support is ~30 LOC across the script and the workflow if/when iterative pipeline testing becomes valuable.
- **A `VERSION` file** — git tag is the single source of truth (single-binary project; run-kit's multi-binary `VERSION`-file rationale doesn't apply).
- **Goreleaser** — the minor-aware base-tag logic for release notes is awkward in goreleaser (requires disabling its changelog and using post-hoc `gh release edit`); cleaner here via `softprops/action-gh-release`'s `previous_tag` parameter. Switching back is a one-evening rewrite if the project grows multiple binaries or wants signing/Docker/Snap.

## Cross-references

- `docs/specs/build-and-release.md` — pre-implementation design intent and behavioral scenarios.
- `docs/memory/build/local.md` — `just build` / `just install` for local development.
- `docs/memory/cli/subcommands.md` — the binary being released, including its `--version` surface and the hidden [`hop help-dump`](../cli/subcommands.md#hop-help-dump--json-help-tree-contract) producer the publish step invokes.
