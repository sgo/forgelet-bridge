#!/usr/bin/env bash
# Verify the per-feature Matrix backend policy through routine acceptance paths.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

scripts/run-matrix-policy-evidence.sh
python3 - docs/evidence/waiting-facts-memory-policy.json <<'PY'
import json
import pathlib
import sys

evidence = json.loads(pathlib.Path(sys.argv[1]).read_text())
if not evidence.get("equivalent"):
    raise SystemExit("matrix policy: ordinary or mutation outcomes differ")
if evidence.get("unlisted_backend") != "fresh":
    raise SystemExit("matrix policy: an unlisted feature did not select fresh")
if not evidence.get("required_fresh_lane"):
    raise SystemExit("matrix policy: the required fresh compatibility lane is absent")
if not evidence.get("after", {}).get("full_qa_s"):
    raise SystemExit("matrix policy: full QA after timing is missing")
if not evidence.get("after", {}).get("full_harden_s"):
    raise SystemExit("matrix policy: full harden after timing is missing")
print("matrix policy: memory and fresh outcomes and timing evidence match")
PY
