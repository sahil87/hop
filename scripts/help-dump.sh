#!/usr/bin/env bash
# Build hop and pretty-print its CLI help tree as JSON (the help/hop.json
# contract published to shll.ai). Local ergonomics — CI generates the
# canonical artifact (see .github/workflows/release.yml). captured_at is left
# empty by the producer; CI injects it.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

"${script_dir}/build.sh" >&2

raw="$("${repo_root}/bin/hop" help-dump)"
if command -v jq >/dev/null 2>&1; then
  printf '%s\n' "$raw" | jq .
else
  printf '%s\n' "$raw"
fi
