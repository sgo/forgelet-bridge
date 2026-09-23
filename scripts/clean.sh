#!/usr/bin/env bash
# Clean up the throwaway runs a build leaves behind: the scenario runs an
# acceptance pass starts and the mutant runs a mutation pass leaves. The newest
# of each kind stays - a run still in flight is using it - and the binary the
# bridge runs from sits beside them, so it is never touched.
#
#   scripts/clean.sh [build-root]      # defaults to <project>/build
#
# How many runs of each kind to keep comes from FORGELET_KEEP_SCENARIO_RUNS and
# FORGELET_KEEP_MUTANT_RUNS.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
build_root="${1:-$project_root/build}"
cleaner="$project_root/build/acceptance/bin/clean-runs"
if [ ! -x "$cleaner" ]; then
  mkdir -p "$(dirname "$cleaner")"
  (cd "$project_root" && go build -tags goolm -o "$cleaner" ./cmd/clean-runs)
fi
"$cleaner" --build "$build_root"
