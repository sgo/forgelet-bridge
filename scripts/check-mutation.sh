#!/usr/bin/env bash
# Run differential function mutation over changed Go functions, then mutate the
# changed Gherkin feature through the project's one acceptance pipeline.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"
export PATH="$project_root/.swarmforge/bin:$project_root/swarmforge/scripts:$PATH"

selection=()
if [[ -n "${FORGELET_MUTATION_FILES:-}" ]]; then
  read -r -a requested <<< "$FORGELET_MUTATION_FILES"
  selection=("${requested[@]}")
else
  base="${FORGELET_BASE_REF:-origin/master}"
  while IFS= read -r file; do
    [[ "$file" == *_test.go ]] && continue
    selection+=("$file")
  done < <({
    git diff --name-only "$base" -- '*.go'
    git ls-files --others --exclude-standard -- '*.go'
  } | grep -E '^(cmd|internal)/' | sort -u)
fi

if [[ ${#selection[@]} -eq 0 ]]; then
  echo "mutation: no changed, covered Go functions to mutate" >&2
  exit 1
fi

printf 'mutation: selected %d changed Go source file(s)\n' "${#selection[@]}"
base="${FORGELET_BASE_REF:-origin/master}"
for file in "${selection[@]}"; do
  package="./$(dirname "$file")"
  if git ls-files --error-unmatch "$file" >/dev/null 2>&1; then
    changed_lines="$(
      git diff --unified=0 "$base" -- "$file" |
        python3 -c '
import re
import sys

lines = []
for line in sys.stdin:
    match = re.match(r"@@ -\S+ \+(\d+)(?:,(\d+))? @@", line)
    if not match:
        continue
    start = int(match.group(1))
    count = int(match.group(2) or "1")
    lines.extend(str(number) for number in range(start, start + count))
print(",".join(dict.fromkeys(lines)))
'
    )"
  else
    changed_lines="$(seq -s, 1 "$(wc -l < "$file")")"
  fi
  if [[ -z "$changed_lines" ]]; then
    echo "mutation: $file has no changed Go lines" >&2
    exit 1
  fi
  scan="$(mutate4go "$file" --scan 2>&1)"
  if grep -q '^Changed mutation sites: 0$' <<< "$scan" &&
     grep -q '^Manifest exists: true$' <<< "$scan"; then
    echo "mutation: $file is unchanged since its successful manifest"
    continue
  fi
  output="$(mutate4go "$file" \
    --lines "$changed_lines" \
    --test-command "go test -tags goolm -count=1 $package" 2>&1)"
  printf '%s\n' "$output"
  if ! grep -q '^Survived: 0$' <<< "$output"; then
    echo "mutation: $file has a surviving Go mutant" >&2
    exit 1
  fi
  if ! grep -q '^Uncovered: 0$' <<< "$output"; then
    echo "mutation: $file has an uncovered Go mutation site" >&2
    exit 1
  fi
  if ! grep -Eq '^Selected mutation sites: [1-9][0-9]*$' <<< "$output"; then
    echo "mutation: $file selected no mutation sites" >&2
    exit 1
  fi
done

features=()
if [[ -n "${FORGELET_GHERKIN_FEATURES:-}" ]]; then
  read -r -a features <<< "$FORGELET_GHERKIN_FEATURES"
else
  while IFS= read -r feature; do
    features+=("$feature")
  done < <({
    git diff --name-only "$base" -- 'features/*.feature'
    git ls-files --others --exclude-standard -- 'features/*.feature'
  } | sort -u)
fi

if [[ ${#features[@]} -eq 0 ]]; then
  echo "mutation: no changed Gherkin feature to mutate" >&2
  exit 1
fi
scripts/run-acceptance-mutation.sh "${features[@]}"
