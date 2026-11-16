#!/usr/bin/env bash
# Verify the bounded automation cutover and record its synthetic gain.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

scripts/acceptance.sh features/consume-bounded-automation-state-in-bridge.feature

mkdir -p target/e2e
go run -tags goolm ./qa/automation-state-measure \
  --ticks 32 \
  --full-payload-bytes 1048576 \
  --output target/e2e/automation-state-measure.json

python3 - docs/evidence/consume-bounded-automation-state-in-bridge.json \
  target/e2e/automation-state-measure.json <<'PY'
import json
import pathlib
import sys

recorded = json.loads(pathlib.Path(sys.argv[1]).read_text())
measured = json.loads(pathlib.Path(sys.argv[2]).read_text())
if measured["ticks"] != recorded["ticks"]:
    raise SystemExit("automation measurement: tick count differs from evidence")
if measured["before"]["requests"] != measured["after"]["requests"]:
    raise SystemExit("automation measurement: request counts differ")
if measured["after"]["bytes"] >= measured["before"]["bytes"]:
    raise SystemExit("automation measurement: bytes did not fall")
if measured["after"]["cpu_seconds"] >= measured["before"]["cpu_seconds"]:
    raise SystemExit("automation measurement: CPU did not fall")
print(
    "automation measurement: requests {requests}, bytes {before_bytes} -> {after_bytes}, "
    "CPU {before_cpu:.3f}s -> {after_cpu:.3f}s".format(
        requests=measured["after"]["requests"],
        before_bytes=measured["before"]["bytes"],
        after_bytes=measured["after"]["bytes"],
        before_cpu=measured["before"]["cpu_seconds"],
        after_cpu=measured["after"]["cpu_seconds"],
    )
)
PY
