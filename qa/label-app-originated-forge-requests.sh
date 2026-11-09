#!/usr/bin/env bash
# Exercise app-relayed completed-card reports through the encrypted Matrix path.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

exec scripts/acceptance.sh features/label-app-originated-forge-requests.feature
