#!/usr/bin/env zsh
# Forge schedule: the machine's cadence for a forge. One pass runs the stall
# watch over every forge root it serves, and then a doorbell pass for each root,
# so a request that never reached its role is rung with nobody asking.
#
# The tools stay separate and so do their rules: the watch notices a role that
# holds work and is not moving, and the doorbell notices a message the operator
# sent that never arrived. This script is only the cadence - what the machine
# runs on a timer, and the evidence that it ran.
#
# Usage:
#   forge_schedule.sh run <forge-root>...
#
# Every pass runs even when another fails, so a broken one cannot take the rest
# down; a root that cannot be served is named rather than skipped in silence;
# and the heartbeat and the log live in the first root, which is where the
# agent that runs this was installed from.
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:$PATH"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

forge_roots() {
  local given=("$@")
  if (( ${#given[@]} == 0 )); then
    given=("$(cd "$SCRIPT_DIR/../.." && pwd)")
  fi
  local root
  for root in "${given[@]}"; do
    root="$(cd "$root" && pwd)"
    [[ -d "$root/projects" ]] || { echo "Not a forge root (no projects/): $root" >&2; exit 2; }
    echo "$root"
  done
}

# A root the schedule cannot serve is a wrong path or a wrong forge rather than
# a quiet one: it holds no project with a roles file to read.
serves_structure() {
  local root="$1" project
  for project in "$root"/projects/*; do
    [[ -f "$project/.swarmforge/roles.tsv" ]] && return 0
  done
  return 1
}

cmd_run() {
  local roots; roots=(${(f)"$(forge_roots "$@")"})
  local failed=0
  local root

  # The watch's pass over every root first: one pass, every project of every
  # forge it was installed for.
  "$SCRIPT_DIR/stall_watch.sh" run "${roots[@]}" || { echo "the watch's pass failed" >&2; failed=$((failed + 1)); }

  # Then a doorbell pass for each root: a lost request belongs to one forge, and
  # one root that cannot be served must not stop the others.
  for root in "${roots[@]}"; do
    if ! serves_structure "$root"; then
      echo "could not serve the forge root $root: it holds no project with a roles file" >&2
      failed=$((failed + 1))
      continue
    fi
    "$SCRIPT_DIR/doorbell.sh" "$root" || { echo "the doorbell's pass failed for $root" >&2; failed=$((failed + 1)); }
  done

  local hb="${roots[1]}/.swarmforge/forge-schedule.heartbeat"
  mkdir -p "$(dirname "$hb")"
  printf '%s roots=%s failed=%s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "${#roots[@]}" "$failed" > "$hb"
  echo "the forge schedule ran for ${#roots[@]} forges (${failed} failed)"
  (( failed == 0 )) || return 1
}

case "${1:-}" in
  run) shift; cmd_run "$@" ;;
  *) sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
esac
