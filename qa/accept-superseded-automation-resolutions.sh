#!/usr/bin/env bash
# Verify superseded automation endings through executable acceptance and one
# composed-layer Bridge.Tick using the configured production dashboard adapters.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
forge_root="$(cd "$project_root/../.." && pwd)"
config="${MATRIX_BRIDGE_CONFIG:-$forge_root/.swarmforge/matrix-bridge.json}"
current_pack="$forge_root/projects/forgelet-swarmforge/swarmforge/scripts/pack_web.bb"
cd "$project_root"

work="$(mktemp -d "$project_root/tmp/current-layer.XXXXXX")"
pids=()
cleanup() {
  if (( ${#pids[@]} > 0 )); then
    kill "${pids[@]}" 2>/dev/null || true
    wait "${pids[@]}" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT

scripts/acceptance.sh features/consume-bounded-automation-state-in-bridge.feature

go test ./internal/dashboard \
  -run 'TestSnapshotPreservesEveryAuthoritativeResolution|TestSnapshotDoesNotFallbackForAnUnknownResolution' \
  -count=1
go test ./internal/relay \
  -run 'TestPlanApprovalsWithDeskReportsSupersededWithoutAManualMark|TestApprovalsReply' \
  -count=1

while IFS=$'\t' read -r index root; do
  bb qa/current-layer-dashboard.bb "$current_pack" "$root" \
    >"$work/$index.url" 2>"$work/$index.log" &
  pids+=("$!")
done < <(python3 - "$config" <<'PY'
import json
import sys

config = json.loads(open(sys.argv[1], encoding="utf-8").read())
for index, forge in enumerate(config["forges"]):
    print(f"{index}\t{forge['root']}")
PY
)

for index in "${!pids[@]}"; do
  for _ in {1..300}; do
    [[ -s "$work/$index.url" ]] && break
    kill -0 "${pids[$index]}" 2>/dev/null || {
      cat "$work/$index.log" >&2
      exit 1
    }
    sleep 0.1
  done
  [[ -s "$work/$index.url" ]] || {
    echo "current-layer dashboard $index did not start" >&2
    exit 1
  }
done

python3 - "$config" "$work" <<'PY'
import json
import pathlib
import sys

source, work = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
config = json.loads(source.read_text(encoding="utf-8"))
config["state_dir"] = str(work / "state")
for index, forge in enumerate(config["forges"]):
    forge["dashboard_url"] = (work / f"{index}.url").read_text(encoding="utf-8").strip()
(work / "config.json").write_text(json.dumps(config), encoding="utf-8")
PY

FORGELET_COMPOSED_BRIDGE_CONFIG="$work/config.json" \
  go test ./internal/bridge -run TestComposedCurrentLayerReachesEveryFamilyInOneTick -count=1 -v
