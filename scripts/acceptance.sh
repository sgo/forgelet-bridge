#!/usr/bin/env bash
# Normal acceptance run: parse the Gherkin, check the IR for duplicated step
# text, generate the entry points, and run the generated tests against a
# homeserver this script starts itself.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

features=()
test_args=()
for arg in "$@"; do
  if [[ "$arg" == -* ]]; then
    test_args+=("$arg")
  else
    features+=("$arg")
  fi
done
if [ ${#features[@]} -eq 0 ]; then
  while IFS= read -r feature; do
    features+=("$feature")
  done < <(find features -name '*.feature' | sort)
fi

parser="$(command -v gherkin-parser || true)"
if [ -z "$parser" ]; then
  parser=".swarmforge/bin/gherkin-parser"
fi
if [ ! -x "$parser" ]; then
  echo "acceptance: gherkin-parser is not installed; run: swarm_tool.sh require gherkin-parser" >&2
  exit 1
fi

mkdir -p build/acceptance/ir build/acceptance/dry build/acceptance/generated build/acceptance/run
rm -f build/acceptance/generated/*_acceptance_test.go
rm -rf build/acceptance/generated/metadata

for feature in "${features[@]}"; do
  stem="$(basename "$feature" .feature)"
  ir="build/acceptance/ir/$stem.json"
  "$parser" "$feature" "$ir"
  if command -v gherkin-ir-dry-checker >/dev/null 2>&1; then
    gherkin-ir-dry-checker "$ir" "build/acceptance/dry/$stem.json" >/dev/null
  fi
done

go build -tags goolm -o build/acceptance/bin/acceptance-entrypoint-generator ./cmd/acceptance-entrypoint-generator
go build -tags goolm -o build/acceptance/bin/forge-dashboard-stub ./cmd/forge-dashboard-stub
go build -tags goolm -o build/acceptance/bin/forgelet-bridge ./cmd/forgelet-bridge

for feature in "${features[@]}"; do
  stem="$(basename "$feature" .feature)"
  ./build/acceptance/bin/acceptance-entrypoint-generator \
    "build/acceptance/ir/$stem.json" \
    build/acceptance/generated \
    --feature "$feature" \
    --project-root "$project_root"
done

export FORGELET_PROJECT_ROOT="$project_root"
export FORGELET_BRIDGE_BIN="$project_root/build/acceptance/bin/forgelet-bridge"
go test -tags goolm -count=1 -v ./build/acceptance/generated "${test_args[@]+"${test_args[@]}"}"
