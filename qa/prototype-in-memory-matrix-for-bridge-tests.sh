#!/usr/bin/env bash
# Re-run the Matrix backend prototype and verify its evidence summary.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

scripts/run-matrix-prototype.sh
python3 - "$project_root/docs/evidence/matrix-backend-prototype.json" <<'PY'
import json
import pathlib
import sys

evidence = json.loads(pathlib.Path(sys.argv[1]).read_text())
if not evidence.get("equivalent"):
    raise SystemExit("matrix prototype: backend outcomes are not equivalent")
backends = evidence["backends"]
for backend in ("fresh", "shared", "memory"):
    item = backends[backend]
    if not item["pass"]:
        raise SystemExit(f"matrix prototype: {backend} failed")
    mutation = item["mutation"]
    if mutation["survived"] or mutation["errors"]:
        raise SystemExit(f"matrix prototype: {backend} mutation mismatch")
if len({(b["mutation"]["total"], b["mutation"]["killed"], b["mutation"]["survived"], b["mutation"]["errors"]) for b in backends.values()}) != 1:
    raise SystemExit("matrix prototype: mutation classifications differ")
print("matrix prototype: all backends and mutations equivalent")
PY
