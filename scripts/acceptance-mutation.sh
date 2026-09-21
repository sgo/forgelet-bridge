#!/usr/bin/env bash
# Acceptance mutation: mutate Gherkin example values in one feature's JSON IR
# and check that the generated acceptance tests notice.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

feature="${1:-features/chat-channel-relay.feature}"
shift || true

mutator="$(command -v gherkin-mutator || true)"
if [ -z "$mutator" ]; then
  mutator=".swarmforge/bin/gherkin-mutator"
fi
if [ ! -x "$mutator" ]; then
  echo "acceptance-mutation: gherkin-mutator is not installed; run: swarm_tool.sh require gherkin-mutator" >&2
  exit 1
fi

parser="$(command -v gherkin-parser || true)"
if [ -z "$parser" ]; then
  parser=".swarmforge/bin/gherkin-parser"
fi
if [ ! -x "$parser" ]; then
  echo "acceptance-mutation: gherkin-parser is not installed; run: swarm_tool.sh require gherkin-parser" >&2
  exit 1
fi

# The mutator reuses the generated entry points for every mutated IR, so this
# run generates them first, exactly as the normal acceptance run does.
stem="$(basename "$feature" .feature)"
mkdir -p build/acceptance-mutation/ir build/acceptance-mutation/generated
"$parser" "$feature" "build/acceptance-mutation/ir/$stem.json"
go build -tags goolm -o build/acceptance/bin/acceptance-entrypoint-generator ./cmd/acceptance-entrypoint-generator
./build/acceptance/bin/acceptance-entrypoint-generator \
  "build/acceptance-mutation/ir/$stem.json" \
  build/acceptance-mutation/generated \
  --feature "$feature" \
  --project-root "$project_root"

"$mutator" \
  --feature "$feature" \
  --work-dir build/acceptance-mutation \
  --generated-dir build/acceptance-mutation/generated \
  --workers 4 \
  --level hard \
  --status-interval 10s \
  --runner-worker "$project_root/scripts/acceptance-runner-worker.sh" \
  "$@"
