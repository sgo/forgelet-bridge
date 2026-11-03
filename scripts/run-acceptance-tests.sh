#!/usr/bin/env bash
# Compatibility entry point for the project's single, existing Go Gherkin
# acceptance pipeline. scripts/acceptance.sh is the implementation.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "$project_root/scripts/acceptance.sh" "$@"
