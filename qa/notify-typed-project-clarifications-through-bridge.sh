#!/usr/bin/env bash
# Verify typed clarification notification parity through fixtures and real
# project/Family dashboards.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

scripts/acceptance.sh features/notify-typed-project-clarifications-through-bridge.feature

mkdir -p target/e2e
go run -tags goolm ./qa/typed-clarification-e2e \
  --output target/e2e/typed-clarification.json

python3 - docs/evidence/notify-typed-project-clarifications-through-bridge.json \
  target/e2e/typed-clarification.json <<'PY'
import json
import pathlib
import sys

recorded = json.loads(pathlib.Path(sys.argv[1]).read_text())
measured = json.loads(pathlib.Path(sys.argv[2]).read_text())
for name in ("clarification_id", "project", "initial_lifecycle", "initial_attention",
             "final_lifecycle", "project_agent_woken", "attention_cleared"):
    if measured[name] != recorded[name]:
        raise SystemExit(f"typed clarification E2E: {name} differs from evidence")
if "[redacted-path]" not in measured["question_excerpt"]:
    raise SystemExit("typed clarification E2E: path was not redacted")
if "[redacted-token]" not in measured["question_excerpt"]:
    raise SystemExit("typed clarification E2E: token was not redacted")
print(
    "typed clarification E2E: {project}/{clarification_id} {initial_lifecycle}/{initial_attention}"
    " -> {final_lifecycle}, project answer queued, attention cleared".format(**measured)
)
PY
