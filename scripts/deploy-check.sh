#!/usr/bin/env bash
# Restart the project's real forge adapter and verify the live bridge.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
forge_root="${MATRIX_BRIDGE_FORGE_ROOT:-$(cd "$project_root/../.." && pwd)}"
adapter="$project_root/scripts/matrix-bridge.sh"
binary="$project_root/build/acceptance/bin/forgelet-bridge"
config="${MATRIX_BRIDGE_CONFIG:-$forge_root/.swarmforge/matrix-bridge.json}"

[[ -x "$binary" ]] || {
  echo "deploy: missing $binary; run scripts/build.sh first" >&2
  exit 1
}
[[ -r "$config" ]] || {
  echo "deploy: bridge configuration is not readable: $config" >&2
  exit 1
}

help="$("$binary" --help 2>&1)"
if ! grep -q -- '-remove-forge' <<< "$help"; then
  echo "deploy: the built bridge does not expose remove-forge" >&2
  exit 1
fi
if ! grep -q -- '-remove-project' <<< "$help"; then
  echo "deploy: the built bridge does not expose remove-project" >&2
  exit 1
fi
"$project_root/scripts/check-adapter-conformance.sh" "$adapter"

export MATRIX_BRIDGE_FORGE_ROOT="$forge_root"
export MATRIX_BRIDGE_BINARY="$binary"
export MATRIX_BRIDGE_CONFIG="$config"

verify_live_status() {
  MATRIX_BRIDGE_TEST_CONFIG="$config" python3 - <<'PY'
import json
import os
from pathlib import Path

config = json.loads(Path(os.environ["MATRIX_BRIDGE_TEST_CONFIG"]).read_text())
status_path = Path(config["state_dir"]) / "status.json"
status = json.loads(status_path.read_text())
configured = [forge.get("name") or Path(forge["root"]).name
              for forge in config.get("forges", [])]
reached = status.get("reached_forges", [])
unhappy = status.get("unhappy_forges", [])
seen = set(reached) | set(unhappy)
missing = [name for name in configured if name not in seen]
if not reached:
    raise SystemExit("deploy: the bridge reached no configured forge")
if missing:
    raise SystemExit("deploy: bridge status does not account for: " +
                     ", ".join(missing))
print("deploy: reached forges: " + (", ".join(reached) or "none"))
print("deploy: unavailable forges: " + (", ".join(unhappy) or "none"))
PY
}

acceptance_only=false
if grep -Fxq '## Acceptance-only deploy' "tasks/$(
  awk -F '\t' '$2 != "done" {print $1}' "$project_root/.swarmforge/board/tasks.tsv"
).md" 2>/dev/null; then
  acceptance_only=true
fi

if [[ "$acceptance_only" == "true" ]]; then
  status="$("$adapter" status)"
  printf '%s\n' "$status"
  grep -q '^running ' <<< "$status"
  verify_live_status
  echo "deploy: acceptance-only; live bridge left running without a restart"
  echo "deploy: adapter conformance and configured-forge status verified"
  exit 0
fi

"$adapter" stop
"$adapter" start
status="$("$adapter" status)"
printf '%s\n' "$status"
grep -q '^running ' <<< "$status"

verify_live_status

echo "deploy: adapter running; host project-removal commands dispatch; bridge exposes removal operations"
