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
  while IFS= read -r file; do
    scan="$(mutate4go "$file" --scan)"
    printf '%s\n' "$scan"
    changed="$(printf '%s\n' "$scan" | sed -n 's/^Changed mutation sites: //p')"
    manifest="$(printf '%s\n' "$scan" | sed -n 's/^Manifest exists: //p')"
    if [[ "$changed" != "0" || "$manifest" != "true" ]]; then
      selection+=("$file")
    fi
  done < <(find cmd internal acceptance -type f -name '*.go' ! -name '*_test.go' | sort)
fi

if [[ ${#selection[@]} -eq 0 ]]; then
  echo "mutation: no changed, covered Go functions to mutate" >&2
  exit 1
fi

printf 'mutation: selected %d changed Go source file(s)\n' "${#selection[@]}"
for file in "${selection[@]}"; do
  package="./$(dirname "$file")"
  output="$(mutate4go "$file" \
    --since-last-run \
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

scripts/run-acceptance-mutation.sh \
  features/forgelet-bridge-single-agent-conversion.feature
