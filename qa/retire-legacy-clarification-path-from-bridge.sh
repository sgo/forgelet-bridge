#!/usr/bin/env bash
# Verify typed-only clarification authority, the live Family preflight and one
# composed production-adapter tick without changing the live Matrix rooms.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
forge_root="${MATRIX_BRIDGE_FORGE_ROOT:-$(cd "$project_root/../.." && pwd)}"
config="${MATRIX_BRIDGE_CONFIG:-$forge_root/.swarmforge/matrix-bridge.json}"
cd "$project_root"

scripts/acceptance.sh features/retire-legacy-clarification-path-from-bridge.feature

mkdir -p target/e2e
go run -tags goolm ./qa/typed-clarification-e2e \
  --output target/e2e/retire-legacy-typed-clarification.json
python3 - target/e2e/retire-legacy-typed-clarification.json <<'PY'
import json
import pathlib
import sys

measured = json.loads(pathlib.Path(sys.argv[1]).read_text())
if measured["initial_lifecycle"] != "open" or measured["initial_attention"] != "unseen":
    raise SystemExit("typed clarification E2E: the question was not open/unseen")
if measured["final_lifecycle"] != "answered":
    raise SystemExit("typed clarification E2E: the question did not finish answered")
if not measured["project_agent_woken"] or not measured["attention_cleared"]:
    raise SystemExit("typed clarification E2E: project wake or attention clearing failed")
if "[redacted-path]" not in measured["question_excerpt"] or "[redacted-token]" not in measured["question_excerpt"]:
    raise SystemExit("typed clarification E2E: the bounded excerpt was not redacted")
print(
    "typed clarification E2E: {project}/{clarification_id} {initial_lifecycle}/{initial_attention}"
    " -> {final_lifecycle}, project answer queued, attention cleared".format(**measured)
)
PY

go build -tags goolm -o build/acceptance/bin/forgelet-bridge ./cmd/forgelet-bridge
build/acceptance/bin/forgelet-bridge --config "$config" --typed-clarification-preflight

FORGELET_COMPOSED_BRIDGE_CONFIG="$config" \
  go test ./internal/bridge -run TestComposedCurrentLayerReachesEveryFamilyInOneTick -count=1 -v
