#!/usr/bin/env bash
# Compare the selected feature on fresh Synapse and the policy-selected memory
# backend, verify the fresh compatibility lane, and record migration timings.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

feature="features/waiting-facts-as-state.feature"
unlisted_feature="features/operator-allowlist.feature"
policy_feature="features/adopt-memory-matrix-for-waiting-facts.feature"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
work="target/acceptance-matrix-policy/$stamp"
if [[ "${FORGELET_POLICY_EVIDENCE_WRITE:-1}" == "1" ]]; then
  evidence_json="docs/evidence/waiting-facts-memory-policy.json"
  evidence_md="docs/evidence/waiting-facts-memory-policy.md"
else
  evidence_json="$work/evidence.json"
  evidence_md="$work/evidence.md"
fi
mkdir -p "$work" "$(dirname "$evidence_json")"

run_acceptance() {
  local backend="$1" log="$2"
  local exit_code=0
  if [[ "$backend" == "memory" ]]; then
    /usr/bin/time -p env -u FORGELET_MATRIX_BACKEND \
      scripts/acceptance.sh "$feature" >"$log" 2>&1 || exit_code=$?
  else
    /usr/bin/time -p env FORGELET_MATRIX_BACKEND="$backend" \
      scripts/acceptance.sh "$feature" >"$log" 2>&1 || exit_code=$?
  fi
  if [[ "$exit_code" -ne 0 ]]; then
    echo "matrix policy: $backend acceptance failed; see $log" >&2
    exit "$exit_code"
  fi
}

run_mutation() {
  local backend="$1" log="$2"
  local exit_code=0
  if [[ "$backend" == "memory" ]]; then
    FORGELET_MUTATION_NO_CACHE=1 \
      env -u FORGELET_MATRIX_BACKEND \
      scripts/run-acceptance-mutation.sh "$feature" >"$log" 2>&1 || exit_code=$?
  else
    FORGELET_MUTATION_NO_CACHE=1 \
      env FORGELET_MATRIX_BACKEND="$backend" \
      scripts/run-acceptance-mutation.sh "$feature" >"$log" 2>&1 || exit_code=$?
  fi
  if [[ "$exit_code" -ne 0 ]]; then
    echo "matrix policy: $backend mutation failed; see $log" >&2
    exit "$exit_code"
  fi
}

run_acceptance fresh "$work/fresh-acceptance.log"
run_acceptance memory "$work/memory-acceptance.log"
run_mutation fresh "$work/fresh-mutation.log"
run_mutation memory "$work/memory-mutation.log"

profile_dir="$project_root/$work/unlisted-profiles"
env -u FORGELET_MATRIX_BACKEND \
  FORGELET_MATRIX_PROFILE_DIR="$profile_dir" \
  scripts/acceptance.sh "$unlisted_feature" >"$work/unlisted-acceptance.log" 2>&1

env -u FORGELET_MATRIX_BACKEND \
  scripts/acceptance.sh "$policy_feature" >"$work/policy-acceptance.log" 2>&1

python3 - \
  "$work" "$evidence_json" "$evidence_md" "$feature" \
  "${FORGELET_POLICY_BEFORE_SELECTED_S:-40.100}" \
  "${FORGELET_POLICY_AFTER_QA_S:-}" \
  "${FORGELET_POLICY_AFTER_HARDEN_S:-}" <<'PY'
import json
import pathlib
import re
import sys

work, evidence_json, evidence_md, feature = map(pathlib.Path, sys.argv[1:5])
before_selected = float(sys.argv[5])
after_qa_arg = sys.argv[6]
after_harden_arg = sys.argv[7]

def wall_seconds(path):
    matches = re.findall(r"^real ([0-9.]+)$", path.read_text(errors="replace"), re.MULTILINE)
    if not matches:
        raise SystemExit(f"matrix policy: no wall time in {path}")
    return float(matches[-1])

def mutation(path):
    matches = re.findall(
        r"^total=(\d+) killed=(\d+) survived=(\d+) errors=(\d+)",
        path.read_text(errors="replace"),
        re.MULTILINE,
    )
    if not matches:
        raise SystemExit(f"matrix policy: no mutation totals in {path}")
    total, killed, survived, errors = map(int, matches[-1])
    return {
        "total": total,
        "killed": killed,
        "survived": survived,
        "errors": errors,
    }

profiles = []
for path in sorted((work / "unlisted-profiles").glob("*.json")):
    profiles.append(json.loads(path.read_text()))
if not profiles:
    raise SystemExit("matrix policy: unlisted feature wrote no backend profiles")
unlisted_backends = sorted({profile["backend"] for profile in profiles})

fresh = {
    "pass": True,
    "wall_s": wall_seconds(work / "fresh-acceptance.log"),
    "mutation": mutation(work / "fresh-mutation.log"),
}
memory = {
    "pass": True,
    "wall_s": wall_seconds(work / "memory-acceptance.log"),
    "mutation": mutation(work / "memory-mutation.log"),
}

existing = {}
if evidence_json.is_file():
    existing = json.loads(evidence_json.read_text())
existing_after = existing.get("after", {})
after_qa = float(after_qa_arg) if after_qa_arg else float(existing_after.get("full_qa_s") or 0)
after_harden = (
    float(after_harden_arg)
    if after_harden_arg
    else float(existing_after.get("full_harden_s") or 0)
)

ordinary_equivalent = fresh["pass"] and memory["pass"]
mutation_equivalent = fresh["mutation"] == memory["mutation"]
policy_safe = unlisted_backends == ["fresh"]
equivalent = ordinary_equivalent and mutation_equivalent and policy_safe
if not equivalent:
    raise SystemExit(
        "matrix policy: backend evidence differs: "
        f"fresh={fresh} memory={memory} unlisted={unlisted_backends}"
    )

result = {
    "schema_version": 1,
    "recorded_at": work.name,
    "feature": str(feature),
    "policy": "acceptance/matrix-backends.json",
    "backends": {"fresh": fresh, "memory": memory},
    "ordinary_equivalent": ordinary_equivalent,
    "mutation_equivalent": mutation_equivalent,
    "unlisted_backend": unlisted_backends[0],
    "required_fresh_lane": True,
    "equivalent": equivalent,
    "before": {
        "selected_s": before_selected,
        "operator_reported_selected_s": 32.9,
        "full_qa_s": 856.908,
        "full_harden_s": 567.0,
    },
    "after": {
        "selected_fresh_s": fresh["wall_s"],
        "selected_memory_s": memory["wall_s"],
        "full_qa_s": after_qa,
        "full_harden_s": after_harden,
    },
    "harden_scope": "changed-feature differential gate",
    "selected_speedup": before_selected / memory["wall_s"],
}
evidence_json.write_text(json.dumps(result, indent=2) + "\n")

def timing(value):
    return f"{value:.1f}s" if value else "pending"

lines = [
    "# Waiting-facts Matrix backend policy evidence",
    "",
    f"Selected feature: `{feature}`",
    f"Recorded: `{result['recorded_at']}`",
    "",
    "| Backend | Ordinary | Wall | Mutation total | Killed | Survived | Errors |",
    "|---|---:|---:|---:|---:|---:|---:|",
]
for name in ("fresh", "memory"):
    item = result["backends"][name]
    mutation_result = item["mutation"]
    lines.append(
        f"| {name} | pass | {item['wall_s']:.2f}s | {mutation_result['total']} | "
        f"{mutation_result['killed']} | {mutation_result['survived']} | "
        f"{mutation_result['errors']} |"
    )
lines += [
    "",
    f"Fresh compatibility selection for the unlisted feature: `{result['unlisted_backend']}`.",
    f"Ordinary equivalence: {'pass' if ordinary_equivalent else 'fail'}.",
    f"Mutation classification equivalence: {'pass' if mutation_equivalent else 'fail'}.",
    "",
    "## Timing",
    "",
    "| Measure | Before | After |",
    "|---|---:|---:|",
    f"| Selected waiting-facts feature | {timing(before_selected)} fresh | {timing(memory['wall_s'])} memory |",
    f"| Full QA | {timing(result['before']['full_qa_s'])} | {timing(after_qa)} |",
    f"| Full harden | {timing(result['before']['full_harden_s'])} | {timing(after_harden)} |",
    "",
    f"The selected-feature before/after ratio is {result['selected_speedup']:.2f}x.",
    "The after harden time is this card's changed-feature differential gate.",
    "The before figure is the broader migration reference recorded by the card,",
    "so only the selected-feature comparison is like-for-like.",
    "",
    "The operator's original selected-feature baseline was 32.9s; the current-tree",
    "fresh measurement is retained separately so both comparisons are visible.",
    "",
    "## Recommendation",
    "",
    "Keep memory for the waiting-facts feature, keep every unlisted feature on",
    "fresh by default, and retain the required fresh-Synapse device, encryption",
    "and restart compatibility lane. The policy validator refuses an attempt to",
    "move that required lane to a non-fresh backend.",
    "",
]
evidence_md.write_text("\n".join(lines))
print(json.dumps(result, indent=2))
PY

echo "matrix policy: evidence written to $evidence_json and $evidence_md"
