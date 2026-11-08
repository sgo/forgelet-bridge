#!/usr/bin/env bash
# Verify the installed adapter exposes every command the project-removal host
# invokes, and that each command reaches its dispatcher.
set -euo pipefail

adapter="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/scripts/matrix-bridge.sh}"
[[ -x "$adapter" ]] || {
  echo "adapter conformance: $adapter is not executable" >&2
  exit 1
}

help="$("$adapter" --help 2>&1)"
for command in remove-project-plan remove-project; do
  if ! grep -q -- "$command <forge-root> <project-name>" <<< "$help"; then
    echo "adapter conformance: usage does not name $command" >&2
    exit 1
  fi
  refusal="$("$adapter" "$command" 2>&1)" || {
    echo "adapter conformance: $command did not reach its dispatcher" >&2
    exit 1
  }
  if [[ "$refusal" != '{"status":"refused","reason":"unsafe_root"}' ]]; then
    echo "adapter conformance: $command returned an unexpected no-argument result" >&2
    exit 1
  fi
done

echo "adapter conformance: remove-project-plan and remove-project dispatch"
