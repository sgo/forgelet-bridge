#!/usr/bin/env bash
# Unit tests. Acceptance tests live behind scripts/acceptance.sh.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

go test -tags goolm -count=1 ./internal/... ./acceptance/... ./cmd/...
