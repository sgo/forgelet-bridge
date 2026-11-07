#!/usr/bin/env bash
# Gherkin acceptance mutation: mutate example values in each approved feature
# and confirm the acceptance runner detects every change.
#
# The mutator rewrites the feature with its manifest and stamp, so each
# feature is copied into tmp/ first. The approved feature files stay untouched;
# the gate log records the mutation totals instead.
set -euo pipefail
cd "$(dirname "$0")/.."
project_root="$PWD"

if ! command -v gherkin-mutator >/dev/null 2>&1; then
  echo "mutation: gherkin-mutator is not on PATH; run swarm_tool.sh ensure gherkin-mutator" >&2
  exit 1
fi
if ! command -v gherkin-parser >/dev/null 2>&1; then
  echo "mutation: gherkin-parser is not on PATH; run swarm_tool.sh ensure gherkin-parser" >&2
  exit 1
fi

features=("$@")
if [[ ${#features[@]} -eq 0 ]]; then
  while IFS= read -r feature; do
    features+=("$feature")
  done < <(find features -type f -name '*.feature' 2>/dev/null | sort)
fi

work=tmp/acceptance-mutation
mkdir -p "$work"
find "$work" -mindepth 1 -delete
mkdir -p "$work/features" target/acceptance-mutation/cache build/acceptance/bin

report=target/acceptance-mutation/mutation.log
: > "$report"
failed=0
ran=0

go build -tags goolm -o build/acceptance/bin/acceptance-entrypoint-generator \
  ./cmd/acceptance-entrypoint-generator

for feature in "${features[@]}"; do
  [[ -f "$feature" ]] || continue
  waived=""
  if [[ -f acceptance/waived-features.txt ]]; then
    while IFS=$'\t' read -r path reason; do
      [[ -z "$path" || "$path" == \#* ]] && continue
      if [[ "$path" == "$feature" ]]; then
        waived="${reason:-listed in acceptance/waived-features.txt}"
      fi
    done < acceptance/waived-features.txt
  fi
  if [[ -n "$waived" ]]; then
    echo "waived $feature: $waived" | tee -a "$report"
    continue
  fi

  stem="$(printf '%s' "$feature" \
    | sed -e 's#^features/##' -e 's#/#-#g' -e 's#\.feature$##')"
  copy="$work/features/$stem.feature"
  ir="$work/$stem.json"
  generated="$work/$stem/generated"
  mkdir -p "$generated"
  cp "$feature" "$copy"
  gherkin-parser "$copy" "$ir"
  ./build/acceptance/bin/acceptance-entrypoint-generator \
    "$ir" "$generated" \
    --feature "$copy" \
    --project-root "$project_root"
  feature_hash="$(shasum -a 256 "$feature" | awk '{print $1}')"
  implementation_hash="$(shasum -a 256 "$generated"/*_acceptance_test.go | awk '{print $1}' | paste -sd, -)"
  cache="target/acceptance-mutation/cache/$stem.passed"
  if [[ -f "$cache" ]] && [[ "$(cat "$cache")" == "$feature_hash"$'\t'"$implementation_hash" ]]; then
    echo "mutation: $feature is unchanged since its successful mutation run" | tee -a "$report"
    continue
  fi
  echo "mutating $feature" | tee -a "$report"
  if ! gherkin-mutator \
      --feature "$copy" \
      --work-dir "$work/$stem/work" \
      --generated-dir "$generated" \
      --status-interval 15s \
      --level hard \
      --runner-worker "$project_root/scripts/acceptance-runner-worker.sh" \
      2>&1 | tee -a "$report"; then
    failed=1
    rm -f "$cache"
  else
    printf '%s\t%s\n' "$feature_hash" "$implementation_hash" > "$cache"
  fi
  ran=1
done

if [[ "$ran" -eq 0 ]]; then
  echo "mutation: no executable features; every feature is waived" | tee -a "$report"
fi

if [[ "$failed" -ne 0 ]]; then
  echo "mutation: Gherkin mutation reported a survivor or error; see $report" >&2
  exit 1
fi
echo "mutation: Gherkin mutation passed; see $report"
