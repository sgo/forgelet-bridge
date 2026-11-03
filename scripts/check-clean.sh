#!/usr/bin/env bash
# Check CRAP and duplicate-code quality with the Go constitution tools.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"
export PATH="$project_root/.swarmforge/bin:$project_root/swarmforge/scripts:$PATH"

mkdir -p tmp target/coverage
report="tmp/crap4go-report.txt"
: > "$report"

sources=()
if [[ -n "${FORGELET_CRAP_FILES:-}" ]]; then
  read -r -a requested <<< "$FORGELET_CRAP_FILES"
  sources=("${requested[@]}")
else
  base="${FORGELET_BASE_REF:-origin/master}"
  while IFS= read -r file; do
    [[ "$file" == *_test.go ]] && continue
    sources+=("$file")
  done < <({
    git diff --name-only "$base" -- '*.go'
    git ls-files --others --exclude-standard -- '*.go'
  } | grep -E '^(cmd|internal)/' | sort -u)
fi

if [[ ${#sources[@]} -eq 0 ]]; then
  echo "clean: no changed Go production source files to measure" >&2
  exit 1
fi

printf 'clean: measuring %d changed Go source file(s)\n' "${#sources[@]}"
for file in "${sources[@]}"; do
  package="./$(dirname "$file")"
  crap4go \
    --max-workers 4 \
    --test-command "go test -tags goolm -count=1 $package" \
    "$file" 2>&1 | tee -a "$report"
done

python3 - "$report" <<'PY'
import re
import sys

highest = 0.0
over = []
in_report = False
for line in open(sys.argv[1], encoding="utf-8"):
    if line.startswith("CRAP Report"):
        in_report = True
        continue
    if not in_report:
        continue
    match = re.search(r"\s+(\d+(?:\.\d+)?)\s*$", line)
    if not match:
        continue
    score = float(match.group(1))
    highest = max(highest, score)
    if score > 6:
        over.append(line.rstrip())

if highest == 0.0:
    raise SystemExit("clean: crap4go produced no scored functions")
if over:
    print("clean: CRAP above the ceiling of 6:", file=sys.stderr)
    print("\n".join(over), file=sys.stderr)
    raise SystemExit(1)
print(f"clean: highest CRAP {highest:.1f} (ceiling 6)")
PY

dry4go --threshold 0.82 --min-lines 4 --min-nodes 20 --format text cmd internal acceptance
echo "clean: dry4go passed"
