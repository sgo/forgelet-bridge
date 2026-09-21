#!/usr/bin/env bash
# The acceptance runner adapter in the shape the Gherkin mutator wants: a
# command that stays hot and answers mutation jobs on stdin/stdout.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

go build -tags goolm -o build/acceptance/bin/acceptance-runner ./cmd/acceptance-runner
exec "$project_root/build/acceptance/bin/acceptance-runner" --worker
