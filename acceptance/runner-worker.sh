#!/usr/bin/env bash
# Compatibility path for the old pack mutation adapter. The single acceptance
# pipeline and its persistent runner are implemented in Go under scripts/.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "$project_root/scripts/acceptance-runner-worker.sh" "$@"
