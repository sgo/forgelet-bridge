# accept-superseded-automation-resolutions

# Accept superseded automation resolutions

## Dependency

Completes the contract repair begun by
`fix-project-scoped-resolved-approvals` in `forgelet-swarmforge`.

## Operator-observed problem

The shared projection now preserves the authoritative `superseded` ending and
scopes resolved approvals to their projects. The bridge validator still accepts
only `approved`, `sent_back`, `automatic_spec_approval` and `auto_approved`, so
it rejects otherwise valid projections containing `superseded`. The live bridge
therefore cannot serve all configured Families through the bounded path.

## Desired behaviour

- Accept `superseded` as a known bounded automation resolution.
- Preserve it through the bridge snapshot while continuing to reject missing,
  non-string and unknown values.
- Keep `superseded` distinct from the manual `approved` and `sent_back` Matrix
  marks; existing reply wording may identify it as superseded, but it must not
  be guessed into either manual action.
- Do not weaken validation or introduce fallback on malformed or failed
  projections.
- Keep this card in `projects/forgelet-bridge`; do not revisit the shared layer.

## E2E

Run the bridge against the composed current layer and confirm one tick reaches
all four configured Families, with no automation projection or resolved
approval errors. Prove a fixture carrying `superseded` preserves the value and
still rejects unknown endings.

## Design

`internal/dashboard` already owns the allowlist for bounded resolved-approval
endings. Add `relay.ResolutionSuperseded` to that allowlist without changing
the shape checks around the row. The snapshot converter already copies the
validated value verbatim, so preserving the ending requires no fallback or
guessed replacement.

The existing relay policy remains authoritative for presentation. `superseded`
uses the threaded reply "Superseded by a newer proposal" and does not select
either manual Matrix mark: only `approved` selects ➡ and only `sent_back`
selects ⬅. Missing, non-string, unknown and malformed rows continue to fail the
whole projection closed.

## Acceptance examples

The existing `one projection carries every authoritative ending` Scenario
Outline becomes the executable five-ending contract. `superseded` is a mutable
example that expects no bridge mark. A focused unit fixture proves the snapshot
keeps the exact value, the reply remains distinct from manual actions, and an
unknown ending is rejected without entering the compatibility fallback.

## End-to-end procedure

`qa/accept-superseded-automation-resolutions.md` and its script run the focused
acceptance feature, then execute one real `Bridge.Tick` through the production
dashboard adapters and the currently composed bridge configuration. Matrix is
represented by the test room boundary so the check does not contact or mutate
the live homeserver. The tick must reach each of the four configured Families
with no unhappy Family and no projection or resolved-approval error. The
procedure reads configuration locally and prints only Family names and counts,
never credentials.

## Documentation

- `README.md` needs no change: its operator-facing projection summary already
  says exact desk resolutions are preserved, and the new ending does not change
  setup, coordinates, quick start or project status.
- `docs/manual.md` will name all five bounded endings and state that
  `superseded` is preserved with its distinct threaded reply and no manual mark.
- `ARCHITECTURE.md` will record the validator-to-snapshot-to-relay separation
  for automatic endings.
- `docs/manual-contract.edn` needs no new section: the existing dashboard
  snapshot section owns the public API and executable example for this policy.

## Verification

- `code` passes all Go unit and fixture packages.
- `clean` and `clean-final` pass with the full dashboard validation functions at
  100% statement coverage and the highest CRAP score at the allowed ceiling of
  6.0; `dry4go` passes.
- `architect` passes `go vet` and the package-boundary checks.
- `harden` kills all 10 changed Gherkin example mutations with zero survivors
  and zero errors, including both the `superseded` resolution and its
  no-manual-mark expectation.
- `qa` passes property tests and all 242 executable acceptance scenarios.
  The focused automation feature passes all five resolution endings plus the
  shared-state, malformed-recovery and compatibility scenarios.
- `e2e` reaches `Forgelet`, `Saibill`, `Innersource` and `Ledgerhall` in one
  composed bridge tick, with zero owed items, no unhappy Family and no
  projection fallback. The fixture keeps `superseded` exactly and the unknown
  ending still fails closed.
- `docs` passes all 15 executable documentation examples across 3 documents and
  21 maintained sections.
- `README.md` is unchanged because installation, configuration, quick start and
  project status are unchanged. `docs/manual.md` now states the five bounded
  endings and the distinct superseded reply. `ARCHITECTURE.md` records the
  validator, snapshot and relay responsibilities. `docs/manual-contract.edn`
  already owns the dashboard snapshot APIs and needed no new section.

## Operator acceptance

- owner: operator | environment: deployed bridge and configured dashboards | checks: all four configured Families remain reachable after a family serves a bounded `superseded` approval alongside the other known endings, and Matrix shows no guessed manual approval or send-back mark for it | status: pending
