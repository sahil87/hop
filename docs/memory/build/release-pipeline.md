---
description: "tag-driven GitHub Actions release workflow; cross-compile matrix; homebrew-tap update via formula template; shll.ai help-reference pull model (2026-06-03 transport inversion); scripts/release.sh"
type: memory
---
# Release Pipeline

How `hop` cuts a release. Hand-rolled GitHub Actions workflow mirroring `~/code/sahil87/run-kit`'s shape, with a tag-driven version source (no `VERSION` file — `hop` is single-binary, so the git tag itself is the source of truth).

## Trigger

A release is triggered by pushing a tag matching `v*` to the origin remote. In practice this happens via:

```
just release [patch|minor|major]   # default: patch
```

which delegates to `scripts/release.sh`. That script computes the next tag, creates it locally, and pushes it. The push fires `.github/workflows/release.yml`, and CI takes over.

The workflow also accepts a `workflow_dispatch` trigger (inputs: `bump` = `patch`/`minor`/`major`, default `patch`) so a release can be cut from the GitHub Actions UI without a local checkout — on that path the **Create tag (manual dispatch)** step computes and pushes the tag inside CI (see Workflow steps below). A `workflow_dispatch` run must originate from `main` (the job's `if` guard). There is no branch-push trigger and no schedule.

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

Single job (`release`) on `ubuntu-latest`, `permissions: contents: write` (no other scopes). The job has eight named steps, but one (**Create tag (manual dispatch)**) is gated by `if: github.event_name == 'workflow_dispatch'` and only runs on the manual-dispatch path — the ordinary tag-push path executes seven of them:

1. **Checkout** with `fetch-depth: 0` — needed for the previous-tag-base computation. Checks out `main` on a `workflow_dispatch` run, otherwise `github.ref`.
2. **Create tag (manual dispatch)** *(`workflow_dispatch` only)* — gated by `if: github.event_name == 'workflow_dispatch'`. Configures the `github-actions[bot]` identity, runs `scripts/release.sh "${{ inputs.bump }}"` to compute + push the next tag, and exports the new tag via `$GITHUB_OUTPUT`. On the tag-push path this step is skipped entirely (the tag already exists).
3. **Setup Go** with `go-version-file: src/go.mod` — keeps the CI Go version in lockstep with `go.mod`.
4. **Extract version from tag** — sets two outputs; on a `workflow_dispatch` run the tag comes from the `Create tag` step's output, otherwise from `${GITHUB_REF#refs/tags/}`:
   - `tag` (with `v` prefix, e.g. `v0.0.1`) — used for ldflags injection.
   - `version` (without prefix, e.g. `0.0.1`) — used for `sed` substitution into the formula.
5. **Cross-compile** — loops over `darwin/arm64 darwin/amd64 linux/arm64 linux/amd64`, building with `CGO_ENABLED=0` and `-ldflags "-X main.version=${tag}"`. Each binary is tarred via `tar -czf "dist/${output}.tar.gz" -C "dist/${output}" hop` — archives contain only the `hop` binary (no LICENSE/README inside).
6. **Determine release notes base tag** — minor-aware logic: if the patch component is `0` (minor bump), `base_tag` is set to the earliest tag matching `v{major}.{minor-1}.*` (sorted by `version:refname`, head -1), so v0.2.0's notes span the entire 0.1.x series. For patch bumps and major bumps, `base_tag` is left unset (default behavior: compare against the immediate previous tag).
7. **Create GitHub Release** via `softprops/action-gh-release` with `files: dist/*.tar.gz`, `generate_release_notes: true`, and `previous_tag: ${{ steps.release-base.outputs.base_tag }}`.
8. **Update Homebrew tap** — see Formula template below. This is the **final** step of the job.

## Action SHAs

All third-party actions are pinned to commit SHAs with `# v<N>` comments:

| Action | SHA | Tag |
|---|---|---|
| `actions/checkout` | `34e114876b0b11c390a56381ad16ebd13914f8d5` | v4 |
| `actions/setup-go` | `40f1582b2485089dde7abd97c1529aa768e1baff` | v5 |
| `softprops/action-gh-release` | `153bb8e04406b158c6c84fc1615b65b24149a1fe` | v2 |

**Policy**: SHAs match `~/code/sahil87/run-kit/.github/workflows/release.yml` at apply time. Deviations need explicit justification — the lockstep keeps both repos updateable via a single-source diff if a third-party action ever needs bumping.

## Help reference (shll.ai)

shll.ai renders an expandable "Command reference" on each tool's landing page from a per-tool `help/<tool>.json` artifact. hop's slice of that artifact is produced by the hidden [`hop help-dump`](/cli/subcommands.md#hop-help-dump--json-help-tree-contract) command, which remains the **contract surface**.

**Transport inversion (2026-06-03)** — shll.ai's help collection used to be **push**-based: hop's release workflow ran a final `Dump help tree and PR to shll.ai` step (added by change `jr5f`) that dumped the help tree from the freshly built binary, injected `captured_at`, validated the envelope, and opened an auto-merged PR into `sahil87/shll.ai`. On 2026-06-03 shll.ai inverted the transport to **pull**: shll.ai's own scheduled job installs each tool and runs the published binary's `hop help-dump` itself to collect `help/hop.json`. With that puller confirmed live, change `g56l` tore down hop's push wiring — the `Dump help tree and PR to shll.ai` workflow step, the `help-dump` justfile recipe, and `scripts/help-dump.sh` were all removed. **hop no longer pushes**; it only exposes the `hop help-dump` command for shll.ai to pull.

> The `SHLLAI_TOKEN` repository secret (the push step's `contents:write` + `pull-requests:write` PAT on `sahil87/shll.ai`) became unused with the push step gone. Deleting it from `sahil87/hop` is a manual maintainer follow-up (GitHub UI) the teardown change could not perform — retire it once no other usage is confirmed.

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
- **No `depends_on` on any toolkit tool.** The formula declares no inter-tool Homebrew dependency, so `brew install sahil87/tap/hop` installs hop standalone and does not pull `wt`. hop's `wt`-delegating surfaces (`hop <name> open` shells out to `wt open`; `hop ls --trees` and the `<name>/<wt>` grammar shell out to `wt list --json`) treat `wt` as a **runtime-probed optional tool** — when it is absent, the `wtMissingHint` fail-fast carries the install command (`brew install sahil87/tap/wt`) so the user can act on it (see [`cli/subcommands`](/cli/subcommands.md) and [`architecture/wrapper-boundaries`](/architecture/wrapper-boundaries.md)). `Formula/wt.rb` lives independently in `sahil87/homebrew-tap` alongside hop, fab-kit, rk, tu, and idea.
- `on_macos` block with nested `on_arm` / `on_intel` blocks declaring `url` and `sha256` for the two darwin tar.gz files.
- `on_linux` block with the same shape for the two linux tar.gz files.
- URLs follow `https://github.com/sahil87/hop/releases/download/v#{version}/hop-{os}-{arch}.tar.gz` — note the `v` prefix is re-added in the URL, so `version "VERSION_PLACEHOLDER"` stores the bare form.
- `install` block: `bin.install "hop"`.
- `test` block: `assert_match version.to_s, shell_output("#{bin}/hop --version")`.

## Setup checklist

One-time setup per repo:

1. **Provision `HOMEBREW_TAP_TOKEN`** as a GitHub repository secret on `sahil87/hop`. It must be a fine-grained Personal Access Token with `Contents: write` permission scoped to `sahil87/homebrew-tap`. This step is manual (GitHub UI) and cannot be automated.
2. **Verify the tap repo** — `sahil87/homebrew-tap` must exist and the bot must have push access via the token. The `Formula/` directory already exists (it hosts `Formula/rk.rb` for run-kit).

## Release-day runbook

1. `just release [patch|minor|major]` (default `patch`) on a clean working tree, on a branch.
2. Watch the workflow at `https://github.com/sahil87/hop/actions`.
3. Verify the GitHub Release page shows four `hop-{os}-{arch}.tar.gz` assets (no separate `checksums.txt` is published).
4. Verify `sahil87/homebrew-tap` got a new commit adding/updating `Formula/hop.rb` authored by `github-actions[bot]`.
5. Smoke test in a clean shell: `brew install sahil87/tap/hop && hop --version` should print `hop version v<version>`.

If `HOMEBREW_TAP_TOKEN` is missing or invalid, the tap-update step fails on `git clone` with an auth error. The GitHub Release (created in the prior step) remains published — re-running typically means provisioning the secret and tagging again (e.g., `v0.0.2`).

## Out of scope

These are policy decisions, not deferrals:

- **Code signing / notarization** — binaries ship unsigned. macOS users see a Gatekeeper warning on first run for direct downloads; brew installs typically don't trip it as hard. An Apple Developer account ($99/yr) is not justified for personal-tooling CLIs.
- **Linux native packaging** (`.deb`, `.rpm`, custom apt/dnf repos) — Linux users install via brew-on-Linux or direct tar.gz download.
- **Prerelease tags** (`v0.0.1-rc.1`) — `release.sh` accepts only `patch|minor|major`. Adding RC support is ~30 LOC across the script and the workflow if/when iterative pipeline testing becomes valuable.
- **A `VERSION` file** — git tag is the single source of truth (single-binary project; run-kit's multi-binary `VERSION`-file rationale doesn't apply).
- **Goreleaser** — the minor-aware base-tag logic for release notes is awkward in goreleaser (requires disabling its changelog and using post-hoc `gh release edit`); cleaner here via `softprops/action-gh-release`'s `previous_tag` parameter. Switching back is a one-evening rewrite if the project grows multiple binaries or wants signing/Docker/Snap.

## Design Decisions

1. **No inter-tool Homebrew dependency — each toolkit tool installs standalone.**
   - *Decision*: The formula template declares no `depends_on` on another tool in the shll toolkit; `wt` is probed at runtime and its missing-tool hint carries the install command.
   - *Why*: A toolkit-wide decision to remove inter-tool brew dependencies so each tool installs and versions independently — users who never touch worktree features don't pay the `wt` install cost, and the tools can be released on separate cadences. hop already routes every `wt` call site through `internal/proc` with `ErrNotFound` handling, so runtime probing is the natural fit; the only gap a bare hint left was discoverability, closed by putting the install command in `wtMissingHint`.
   - *Rejected*: Keeping `depends_on "sahil87/tap/wt"` in the formula (couples the tools at the package-manager level, contradicting the toolkit standard, and forces the `wt` install on every hop user).
   - *Introduced by*: remove-wt-brew-dependency

## Cross-references

- `docs/specs/build-and-release.md` — pre-implementation design intent and behavioral scenarios.
- `docs/memory/build/local.md` — `just build` / `just install` for local development.
- `docs/memory/build/ci-pipeline.md` — the push/PR test gate (`ci.yml`); shares this workflow's SHA-pinned actions and `src/`-rooted Go setup, but fires on pushes/PRs (not `v*` tags) and only needs `contents: read`.
- `docs/memory/cli/subcommands.md` — the binary being released, including its `--version` surface and the hidden [`hop help-dump`](/cli/subcommands.md#hop-help-dump--json-help-tree-contract) producer that shll.ai's scheduled job pulls (see Help reference above).
