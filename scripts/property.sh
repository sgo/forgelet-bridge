#!/usr/bin/env bash
# Property tests: the invariants the bridge leans on (restart bookkeeping,
# round trips, allowlisting). They are behind the `property` build tag so they
# stay out of the unit coverage, CRAP, and mutation runs.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

go test -tags "goolm property" -count=1 -run 'Property' ./internal/... ./acceptance/... "$@"
