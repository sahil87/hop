---
description: "justfile + scripts/build.sh + scripts/install.sh + scripts/sync-skill.sh; local development workflow, including the skill-bundle sync + drift-guard test loop"
type: memory
---
# Local Build

How `hop` is built and installed locally. The cross-platform release pipeline (GitHub Actions, homebrew-tap) lives in [release-pipeline](/build/release-pipeline.md).

## Justfile

`justfile` at the repo root — one-line recipes only (Constitution Principle V):

```just
default:
    @just --list

build:
    ./scripts/build.sh

local-install:
    ./scripts/install.sh

test:
    cd src && go test ./...

sync-skill:
    ./scripts/sync-skill.sh

release bump="patch":
    ./scripts/release.sh {{bump}}
```

The `release` recipe delegates to `scripts/release.sh` and is documented in [release-pipeline](/build/release-pipeline.md). The `sync-skill` recipe delegates to `scripts/sync-skill.sh` and is documented in [Skill bundle sync + drift guard](#skill-bundle-sync--drift-guard-change-armh) below.

## `scripts/build.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

VERSION="$(git describe --tags --always 2>/dev/null || echo dev)"
mkdir -p bin
cd src
go build -ldflags "-X main.version=${VERSION}" -o ../bin/hop ./cmd/hop
echo "built: bin/hop (version: ${VERSION})"
```

- Output: `./bin/hop` at the repo root. `bin/` is gitignored (`.gitignore` includes `bin/`).
- `VERSION` injected via `-ldflags "-X main.version=${VERSION}"` into the package-level `var version = "dev"` in `src/cmd/hop/main.go`. `hop --version` and `hop -v` print this string (cobra auto-wires both when `rootCmd.Version` is set).
- Possible `VERSION` values:
  - Pre-tag: short SHA from `git describe --always` (e.g., `9b6b2a4`).
  - Tagged: `v0.1.0`.
  - Post-tag commit: `v0.1.0-2-gabc123`.
  - No git history: `dev`.

## `scripts/install.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

./scripts/build.sh

DEST="${HOME}/.local/bin/hop"
mkdir -p "$(dirname "$DEST")"
cp -f ./bin/hop "$DEST"
echo "installed: $DEST"
```

- Always builds first (no skip-if-exists).
- Copies to `~/.local/bin/hop`. The user is responsible for `~/.local/bin` being on `$PATH`.
- Idempotent — re-running overwrites.

## `hop --version` chain

`scripts/build.sh` → `-ldflags "-X main.version=…"` → `src/cmd/hop/main.go::var version` → `rootCmd.Version = version` (set in `main()`) → cobra wires `--version` and `-v` automatically. The cobra-default `version` subcommand also works; no effort spent suppressing it.

## Skill bundle sync + drift guard (change `armh`)

The `hop skill` command embeds an agent skill bundle whose **canonical** source is `docs/site/skill.md` (also rendered at `https://shll.ai/hop/skill`). The Go module root is `src/`, so `//go:embed` cannot reach `docs/site/` above it — a **committed copy** at `src/cmd/hop/skill.md` bridges the gap. Three build/test pieces keep that copy honest, mirroring the pattern `shll standards` established (Constitution V: thin justfile, logic in `scripts/`; Constitution IV: reuse the sibling tool's proven mechanism):

`scripts/sync-skill.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
cp -f docs/site/skill.md src/cmd/hop/skill.md
echo "synced src/cmd/hop/skill.md from docs/site/skill.md"
```

- Copies the canonical file into the package dir; `cd "$(dirname "$0")/.."` makes it run from the repo root regardless of caller CWD. Idempotent — re-running overwrites.
- Invoked by `just sync-skill` (the one-line recipe above) and by `go generate` via the `//go:generate ../../../scripts/sync-skill.sh` directive in `src/cmd/hop/skill.go`. Run it after editing the bundle, then commit the refreshed copy.

**Drift guard + contract tests** (`src/cmd/hop/skill_test.go`), the "drift-guard test failing the build on divergence" the skill standard mandates — all run on every `cd src && go test ./...` and in the CI PR workflow (see [ci-pipeline](/build/ci-pipeline.md)):

- `TestSkillEmbedMatchesCanonical` — the **drift guard**: `skillBundle` (the embedded bytes) MUST equal `../../../docs/site/skill.md`; on divergence it fails naming the fix (`just sync-skill` / `scripts/sync-skill.sh`). A deliberate edit to the canonical file without re-syncing fails the build here.
- `TestSkillInvocationContract` — `hop skill` returns nil from RunE (exit 0), writes the embedded bundle byte-identically to stdout, and leaves stderr empty.
- `TestSkillRejectsArgs` — `hop skill extra` is a `cobra.NoArgs` usage error.
- `TestSkillVisible` — `Use == "skill"` and the command is NOT `Hidden`.
- `TestSkillBudget` — the **budget guard**: the canonical bundle is ≤150 lines (the standard's hard budget), read directly from `../../../docs/site/skill.md` so an over-long edit fails even before the sync step runs.

The committed copy means a clean `cd src && go build ./...` compiles **without** running the sync script — the script/`go generate` is only needed to refresh the copy after a bundle edit; the drift guard catches a forgotten refresh at test time. Command-level details (invocation contract, embed rationale, single-file `[]byte` vs shll's `embed.FS`) live in [cli/subcommands § hop skill](/cli/subcommands.md#hop-skill--agent-skill-bundle).

## Cross-platform builds

Verified at apply time by:

```
cd src && GOOS=darwin GOARCH=arm64 go build ./...
cd src && GOOS=linux GOARCH=amd64 go build ./...
```

Both succeed because `internal/platform/` uses build tags (`//go:build darwin`, `//go:build linux`). Runtime tests run on the host platform only.

## Cross-references

The cross-platform release pipeline (tag-driven workflow, formula template, `release.sh`, homebrew-tap update) is documented in [release-pipeline](/build/release-pipeline.md). Pre-implementation design intent lives in `docs/specs/build-and-release.md`.
