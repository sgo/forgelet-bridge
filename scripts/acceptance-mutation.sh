#!/usr/bin/env bash
# Run differential Gherkin mutation through the project's single acceptance
# pipeline. With no arguments every feature is mutated.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "$project_root/scripts/run-acceptance-mutation.sh" "$@"
