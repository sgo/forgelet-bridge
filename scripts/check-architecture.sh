#!/usr/bin/env bash
# Check Go vet and the bridge's package-boundary invariants.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"
export PATH="$project_root/.swarmforge/bin:$project_root/swarmforge/scripts:$PATH"

go vet -tags goolm ./...

violations="$(
  go list -tags goolm -f '{{.ImportPath}}{{"\t"}}{{join .Imports " "}}' ./internal/... |
    while IFS=$'\t' read -r package imports; do
      short="${package#github.com/unclebob/forgelet-bridge/}"
      if [[ "$imports" == *github.com/unclebob/forgelet-bridge/internal/bridge* ||
            "$imports" == *github.com/unclebob/forgelet-bridge/internal/matrix* ]]; then
        if [[ "$short" != "internal/matrix" ]]; then
          printf '%s may not import bridge or matrix: %s\n' "$short" "$imports"
        fi
      elif [[ "$imports" == *maunium.net/go/mautrix* ]]; then
        if [[ "$short" != "internal/matrix" ]]; then
          printf '%s may not import the Matrix client: %s\n' "$short" "$imports"
        fi
      fi
    done
)"

if [[ -n "$violations" ]]; then
  printf '%s\n' "$violations" >&2
  exit 1
fi

echo "architecture: go vet passed"
echo "architecture: only internal/matrix imports the Matrix client and bridge boundary"
