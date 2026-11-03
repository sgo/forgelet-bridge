#!/usr/bin/env bash
# Run one project gate with the real Go pipeline and retain its console log.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"
export PATH="$project_root/.swarmforge/bin:$project_root/swarmforge/scripts:$PATH"

stage="${1:-}"
if [[ -z "$stage" ]]; then
  echo "usage: scripts/stage-gate.sh <stage>" >&2
  exit 2
fi

mkdir -p build/stages
log="build/stages/$stage.log"
: > "$log"

run() {
  printf '$' | tee -a "$log"
  printf ' %q' "$@" | tee -a "$log"
  printf '\n' | tee -a "$log"
  "$@" 2>&1 | tee -a "$log"
}

case "$stage" in
  code)
    run scripts/test.sh
    ;;
  clean)
    run scripts/check-clean.sh
    ;;
  clean-final)
    run scripts/check-clean.sh
    ;;
  architect)
    run scripts/check-architecture.sh
    ;;
  harden)
    run scripts/check-mutation.sh
    ;;
  qa)
    run scripts/property.sh
    run scripts/acceptance.sh
    ;;
  e2e)
    run scripts/run-qa-e2e.sh
    ;;
  docs)
    run scripts/check-documentation.sh
    ;;
  deploy)
    export MATRIX_BRIDGE_FORGE_ROOT="${MATRIX_BRIDGE_FORGE_ROOT:-$(cd "$project_root/../.." && pwd)}"
    run scripts/build.sh
    run scripts/matrix-bridge.sh status
    run scripts/deploy-check.sh
    ;;
  *)
    echo "stage-gate: unknown stage: $stage" >&2
    exit 2
    ;;
esac
