# build — memory

| File | Topic |
|---|---|
| [local](local.md) | justfile + scripts/build.sh + scripts/install.sh; local development workflow |
| [release-pipeline](release-pipeline.md) | tag-driven GitHub Actions release workflow; cross-compile matrix; homebrew-tap update via formula template; shll.ai help-reference pull model (2026-06-03 transport inversion); scripts/release.sh |
| [ci-pipeline](ci-pipeline.md) | push/PR GitHub Actions test gate (`ci.yml`); `test` job (gofmt/vet/test from `src/`) + `ci-gate` single required-check job; byte-identical to `wt`; branch-protection is a manual repo-settings follow-up |
