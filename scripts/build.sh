#!/usr/bin/env bash
# Build every forgelet-bridge command.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

mkdir -p build/acceptance/bin
for command in forgelet-bridge install-rules acceptance-entrypoint-generator acceptance-runner; do
  go build -tags goolm -o "build/acceptance/bin/$command" "./cmd/$command"
done
