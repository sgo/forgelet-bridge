#!/usr/bin/env bash
# Verify one dashboard snapshot per tick and record the bounded synthetic gain.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

scripts/acceptance.sh features/fetch-one-dashboard-snapshot-per-bridge-tick.feature

mkdir -p target/e2e
go run -tags goolm ./qa/state-snapshot-measure \
  --ticks 32 \
  --payload-bytes 1048576 \
  --output target/e2e/state-snapshot-measure.json

python3 - docs/evidence/fetch-one-dashboard-snapshot-per-bridge-tick.json \
  target/e2e/state-snapshot-measure.json <<'PY'
import json
import pathlib
import sys

recorded = json.loads(pathlib.Path(sys.argv[1]).read_text())
measured = json.loads(pathlib.Path(sys.argv[2]).read_text())
for name in ("ticks", "payload_bytes"):
    if measured[name] != recorded[name]:
        raise SystemExit(f"snapshot measurement: {name} differs from evidence")
if measured["before"]["requests"] != 128 or measured["after"]["requests"] != 64:
    raise SystemExit("snapshot measurement: request counts differ from 128/64")
if measured["after"]["bytes"] != measured["before"]["bytes"] // 2:
    raise SystemExit("snapshot measurement: bytes did not fall by half")
if measured["after"]["requests"] >= measured["before"]["requests"]:
    raise SystemExit("snapshot measurement: request count did not fall")
print(
    "snapshot measurement: requests {before} -> {after}, bytes {before_bytes} -> {after_bytes}, "
    "CPU {before_cpu:.3f}s -> {after_cpu:.3f}s".format(
        before=measured["before"]["requests"],
        after=measured["after"]["requests"],
        before_bytes=measured["before"]["bytes"],
        after_bytes=measured["after"]["bytes"],
        before_cpu=measured["before"]["cpu_seconds"],
        after_cpu=measured["after"]["cpu_seconds"],
    )
)
PY
