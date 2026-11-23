#!/usr/bin/env bash
# Reproduce the failing lieutenant composition from this committed bridge tree.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

if [[ -n "${SWARMFORGE_SKIP_BRIDGE:-}" ]]; then
  echo "e2e: SWARMFORGE_SKIP_BRIDGE must not be set" >&2
  exit 1
fi

commit="$(git rev-parse HEAD)"
tree="$(git rev-parse 'HEAD^{tree}')"
mkdir -p target/e2e
scratch_root="$project_root/target/e2e"
scratch="$(mktemp -d "$scratch_root/restore-doorbell.XXXXXX")"
bridge_source="$(mktemp -d "$scratch_root/restore-doorbell-source.XXXXXX")"
log="$project_root/target/e2e/restore-doorbell-composition.log"

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    echo "e2e: retained failed composition scratch at $scratch" >&2
    echo "e2e: retained the committed bridge snapshot at $bridge_source" >&2
    exit "$status"
  fi
  case "$scratch" in
    "$scratch_root"/restore-doorbell.*)
      rm -rf -- "$scratch"
      ;;
    *)
      echo "e2e: refusing to remove unexpected scratch path $scratch" >&2
      exit 1
      ;;
  esac
  case "$bridge_source" in
    "$scratch_root"/restore-doorbell-source.*)
      rm -rf -- "$bridge_source"
      ;;
    *)
      echo "e2e: refusing to remove unexpected bridge source path $bridge_source" >&2
      exit 1
      ;;
  esac
}
trap cleanup EXIT

git archive --format=tar HEAD | tar -x -C "$bridge_source"
(cd "$bridge_source" && scripts/build.sh)

(
  cd "$scratch"
  SWARMFORGE_LIEUTENANT_AGENT=claude \
  SWARMFORGE_BRIDGE_DIR="$bridge_source" \
  SWARMFORGE_NO_SELF_UPDATE=1 \
    get-swarm-forge lieutenant
) 2>&1 | tee "$log"

if [[ ! -f "$scratch/swarmforge/scripts/doorbell.bb" ]]; then
  echo "e2e: the composition did not install the canonical doorbell" >&2
  exit 1
fi
if ! grep -Fq "the answer is the operator's to give" "$scratch/swarmforge/scripts/doorbell.bb"; then
  echo "e2e: the composed doorbell lost the clarification clause" >&2
  exit 1
fi
if ! grep -Fq "self-check doorbell" "$log"; then
  echo "e2e: the composition emitted no doorbell self-check" >&2
  exit 1
fi
if ! grep -Fq "it rang a clarification and found the answer clause" "$log"; then
  echo "e2e: the doorbell self-check did not find the answer clause" >&2
  exit 1
fi
if ! grep -Fq "SwarmForge forge installed from" "$log"; then
  echo "e2e: the composition never reached its success message" >&2
  exit 1
fi

echo "e2e: bridge commit $commit, tree $tree"
echo "e2e: get-swarm-forge lieutenant completed through bridge kit installation"

