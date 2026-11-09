#!/usr/bin/env bash
# Measure fresh Synapse, shared Synapse and the in-memory Matrix fixture.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

feature="${FORGELET_PROTOTYPE_FEATURE:-features/forge-display-names.feature}"
runs="${FORGELET_PROTOTYPE_RUNS:-3}"
out="$project_root/target/acceptance-matrix-prototype"
evidence_json="$project_root/docs/evidence/matrix-backend-prototype.json"
evidence_md="$project_root/docs/evidence/matrix-backend-prototype.md"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
work="$out/$stamp"
runs_file="$work/runs.tsv"
mutation_file="$work/mutations.tsv"
reuse_dir="${FORGELET_PROTOTYPE_REUSE_DIR:-}"

if [[ -n "$reuse_dir" ]]; then
  work="$reuse_dir"
  runs_file="$work/runs.tsv"
  mutation_file="$work/mutations.tsv.rebuilt"
  [[ -f "$runs_file" ]] || {
    echo "matrix prototype: reuse directory has no runs.tsv: $work" >&2
    exit 1
  }
  printf 'backend\tpass\telapsed_s\ttotal\tkilled\tsurvived\terrors\n' > "$mutation_file"
  for backend in fresh shared memory; do
    python3 - "$backend" "$work/$backend/mutation.log" "$mutation_file" <<'PY'
import pathlib
import re
import sys

backend, log_path, mutation_file = sys.argv[1:]
text = pathlib.Path(log_path).read_text(errors="replace")
matches = list(re.finditer(r"elapsed=(\d+)ms[^\n]*total=(\d+)[^\n]*killed=(\d+)[^\n]*survived=(\d+)[^\n]*errors=(\d+)", text))
if not matches:
    raise SystemExit(f"matrix prototype: cannot parse mutation evidence for {backend}")
match = matches[-1]
elapsed_ms, total, killed, survived, errors = match.groups()
with pathlib.Path(mutation_file).open("a") as output:
    output.write("\t".join([backend, "1", f"{int(elapsed_ms) / 1000:.3f}", total, killed, survived, errors]) + "\n")
PY
  done
else
  mkdir -p "$work" "$out" "$(dirname "$evidence_json")"
  printf 'backend\trun\tpass\texit\treal_s\tuser_s\tsys_s\tmax_rss_bytes\tlog_bytes\n' > "$runs_file"
  printf 'backend\tpass\telapsed_s\ttotal\tkilled\tsurvived\terrors\n' > "$mutation_file"

for backend in fresh shared memory; do
  backend_dir="$work/$backend"
  mkdir -p "$backend_dir/profiles"
  for ((run = 1; run <= runs; run++)); do
    log="$backend_dir/run-$run.log"
    exit_code=0
    FORGELET_MATRIX_BACKEND="$backend" \
    FORGELET_MATRIX_PROFILE_DIR="$backend_dir/profiles/run-$run" \
      /usr/bin/time -l scripts/acceptance.sh "$feature" >"$log" 2>&1 || exit_code=$?
    pass=0
    [[ "$exit_code" -eq 0 ]] && pass=1
    python3 - "$backend" "$run" "$pass" "$exit_code" "$log" "$runs_file" <<'PY'
import pathlib
import re
import sys

backend, run, passed, exit_code, log_path, runs_file = sys.argv[1:]
text = pathlib.Path(log_path).read_text(errors="replace")

def metric(pattern, default="0"):
    match = re.search(pattern, text, re.MULTILINE)
    return match.group(1) if match else default

real = metric(r"([0-9.]+)\s+real")
user = metric(r"([0-9.]+)\s+user")
sys_time = metric(r"([0-9.]+)\s+sys")
rss = metric(r"^\s*([0-9]+)\s+maximum resident set size$")
log_bytes = str(pathlib.Path(log_path).stat().st_size)
with pathlib.Path(runs_file).open("a") as output:
    output.write("\t".join([backend, run, passed, exit_code, real, user, sys_time, rss, log_bytes]) + "\n")
PY
    if [[ "$pass" != "1" ]]; then
      echo "matrix prototype: $backend run $run failed; see $log" >&2
      exit 1
    fi
  done

  mutation_log="$backend_dir/mutation.log"
  mutation_exit=0
  FORGELET_MUTATION_NO_CACHE=1 \
  FORGELET_MATRIX_BACKEND="$backend" \
    scripts/run-acceptance-mutation.sh "$feature" >"$mutation_log" 2>&1 || mutation_exit=$?
  mutation_pass=0
  [[ "$mutation_exit" -eq 0 ]] && mutation_pass=1
  python3 - "$backend" "$mutation_pass" "$mutation_log" "$mutation_file" <<'PY'
import pathlib
import re
import sys

backend, passed, log_path, mutation_file = sys.argv[1:]
text = pathlib.Path(log_path).read_text(errors="replace")
matches = list(re.finditer(r"elapsed=(\d+)ms[^\n]*total=(\d+)[^\n]*killed=(\d+)[^\n]*survived=(\d+)[^\n]*errors=(\d+)", text))
if not matches:
    elapsed = total = killed = survived = errors = "0"
else:
    match = matches[-1]
    elapsed_ms, total, killed, survived, errors = match.groups()
    elapsed = f"{int(elapsed_ms) / 1000:.3f}"
with pathlib.Path(mutation_file).open("a") as output:
    output.write("\t".join([backend, passed, elapsed, total, killed, survived, errors]) + "\n")
PY
    if [[ "$mutation_pass" != "1" ]]; then
      echo "matrix prototype: $backend mutation failed; see $mutation_log" >&2
      exit 1
    fi
  done
fi

python3 - "$work" "$runs_file" "$mutation_file" "$evidence_json" "$evidence_md" "$feature" <<'PY'
import json
import pathlib
import statistics
import sys

work, runs_file, mutation_file, json_path, md_path, feature = map(pathlib.Path, sys.argv[1:])

runs = []
with runs_file.open() as source:
    headers = source.readline().rstrip("\n").split("\t")
    for line in source:
        values = line.rstrip("\n").split("\t")
        row = dict(zip(headers, values))
        row["run"] = int(row["run"])
        row["pass"] = row["pass"] == "1"
        row["exit"] = int(row["exit"])
        for name in ("real_s", "user_s", "sys_s"):
            row[name] = float(row[name])
        for name in ("max_rss_bytes", "log_bytes"):
            row[name] = int(row[name])
        runs.append(row)

mutations = []
with mutation_file.open() as source:
    headers = source.readline().rstrip("\n").split("\t")
    for line in source:
        values = line.rstrip("\n").split("\t")
        row = dict(zip(headers, values))
        row["pass"] = row["pass"] == "1"
        row["elapsed_s"] = float(row["elapsed_s"])
        for name in ("total", "killed", "survived", "errors"):
            row[name] = int(row[name])
        mutations.append(row)

profiles = []
for path in work.glob("*/profiles/*/*.json"):
    profile = json.loads(path.read_text())
    profile["evidence_path"] = str(path.relative_to(work.parent.parent))
    profiles.append(profile)

backends = {}
for backend in ("fresh", "shared", "memory"):
    backend_runs = [row for row in runs if row["backend"] == backend]
    backend_profiles = [row for row in profiles if row["backend"] == backend]
    mutation = next(row for row in mutations if row["backend"] == backend)
    phases = {}
    for key in ("total_ms", "setup_ms", "bridge_start_ms", "scenario_ms", "teardown_ms"):
        values = [row[key] for row in backend_profiles]
        phases[key] = {
            "median": statistics.median(values) if values else 0,
            "total": sum(values),
            "samples": len(values),
        }
    backends[backend] = {
        "runs": backend_runs,
        "pass": all(row["pass"] for row in backend_runs),
        "median_wall_s": statistics.median([row["real_s"] for row in backend_runs]),
        "median_max_rss_bytes": int(statistics.median([row["max_rss_bytes"] for row in backend_runs])),
        "phases": phases,
        "mutation": mutation,
    }

baseline = backends["fresh"]["median_wall_s"]
memory_speedup = baseline / backends["memory"]["median_wall_s"] if backends["memory"]["median_wall_s"] else 0
shared_speedup = baseline / backends["shared"]["median_wall_s"] if backends["shared"]["median_wall_s"] else 0
mutation_results = {
    (row["total"], row["killed"], row["survived"], row["errors"])
    for row in mutations
}
equivalent = (
    all(item["pass"] for item in backends.values())
    and len(mutation_results) == 1
    and next(iter(mutation_results))[2] == 0
    and next(iter(mutation_results))[3] == 0
)

recommendation = "reject the fake until the mismatch is fixed"
if equivalent:
    recommendation = (
        "adopt the in-memory backend as an opt-in fast lane for routine process-heavy "
        "Gherkin scenarios, keep a smaller fresh-Synapse compatibility lane for Olm/device "
        "and protocol drift, and retain shared Synapse only for scenarios that need a real "
        "server without per-scenario process startup"
    )

evidence = {
    "schema_version": 1,
    "recorded_at": work.name,
    "feature": str(feature),
    "runs_per_backend": len(backends["fresh"]["runs"]),
    "backends": backends,
    "speedup_vs_fresh": {
        "shared": shared_speedup,
        "memory": memory_speedup,
    },
    "equivalent": equivalent,
    "recommendation": recommendation,
}
json_path.parent.mkdir(parents=True, exist_ok=True)
json_path.write_text(json.dumps(evidence, indent=2) + "\n")

lines = [
    "# Matrix backend prototype evidence",
    "",
    f"Selected feature: `{feature}`",
    f"Runs per backend: {evidence['runs_per_backend']}",
    f"Recorded: `{evidence['recorded_at']}`",
    "",
    "| Backend | Ordinary | Median wall | Median max RSS | Setup median | Bridge median | Scenario median | Teardown median | Mutation wall | Mutations |",
    "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
]
for backend in ("fresh", "shared", "memory"):
    item = backends[backend]
    phases = item["phases"]
    mutation = item["mutation"]
    lines.append(
        f"| {backend} | {'pass' if item['pass'] else 'fail'} | "
        f"{item['median_wall_s']:.3f}s | {item['median_max_rss_bytes'] / (1024 * 1024):.1f} MiB | "
        f"{phases['setup_ms']['median']:.0f} ms | {phases['bridge_start_ms']['median']:.0f} ms | "
        f"{phases['scenario_ms']['median']:.0f} ms | {phases['teardown_ms']['median']:.0f} ms | "
        f"{mutation['elapsed_s']:.1f}s | "
        f"{mutation['killed']}/{mutation['total']} killed, {mutation['survived']} survived, {mutation['errors']} errors |"
    )
lines += [
    "",
    f"Shared/fresh wall-time ratio: {shared_speedup:.2f}x. In-memory/fresh wall-time ratio: {memory_speedup:.2f}x.",
    f"Backend and mutation equivalence: {'pass' if equivalent else 'fail'}.",
    "",
    f"Recommendation: {recommendation}.",
    "",
    "## Follow-up plan",
    "",
    "1. Adopt the memory backend as an opt-in lane for routine routing, state and threading scenarios after each feature's Matrix endpoints are covered.",
    "2. Keep a smaller fresh-Synapse suite for Olm, device identity, encrypted compatibility, restart and protocol-drift coverage.",
    "3. Use shared Synapse only for scenarios that require real-server semantics but cannot justify per-scenario process startup; keep per-scenario rooms and unique bridge devices.",
    "4. Migrate one scenario group at a time in a separate measured card, with ordinary and mutation equivalence checked before the fast lane becomes the routine default.",
    "",
]
md_path.write_text("\n".join(lines))
print(json.dumps(evidence, indent=2))
PY

echo "matrix prototype: evidence written to $evidence_json and $evidence_md"
