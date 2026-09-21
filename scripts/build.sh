#!/usr/bin/env bash
# Build every forgelet-bridge command.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

mkdir -p build/acceptance/bin
for command in forgelet-bridge acceptance-entrypoint-generator acceptance-runner forge-dashboard-stub; do
  go build -tags goolm -o "build/acceptance/bin/$command" "./cmd/$command"
done
