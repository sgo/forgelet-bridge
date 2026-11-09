# adopt-memory-matrix-for-waiting-facts

# Adopt the in-memory Matrix backend for waiting-facts scenarios

## Operator-observed problem

The bridge prototype proved that the in-memory Matrix backend preserves
behavior and runs the representative feature 1.68x faster than fresh Synapse,
but the production acceptance suite still defaults every feature to fresh
Synapse. The measured speedup is therefore not realized by ordinary gates.

The first migration group is `waiting-facts-as-state.feature`: seven scenarios
that took 32.9s under fresh Synapse and exercise board, approval and
clarification state plus a bridge restart.

## Backend policy contract

`acceptance/matrix-backends.json` is the test-only policy. It maps exact
relative paths under `features/` to `fresh`, `shared` or `memory`. The initial
policy maps only `features/waiting-facts-as-state.feature` to `memory`; an
unlisted feature resolves to `fresh`.

`FORGELET_MATRIX_BACKEND` remains the focused-run override. A valid explicit
value wins over the policy. Without it, every generated acceptance entry point
resolves and passes its feature's backend into that feature's scenario worlds,
so selection is per feature and does not depend on test execution order.

The resolver validates the whole policy before using it. Missing, malformed or
trailing JSON, a missing `features` map, unknown fields, unknown backend names,
and absolute, traversing or nested feature paths fail the generated test
visibly. They never degrade to a fallback backend. Requests without a policy
entry, in a valid policy, select fresh Synapse.

The bridge, dashboard, adapters and Olm clients remain real in every backend.
Production bridge configuration and behavior do not read this policy. The
existing fresh Synapse encryption, device-identity, restart and protocol-drift
features remain unlisted and therefore required fresh coverage. The existing
`fresh`, `shared` and `memory` implementations are all retained.

## Testing and evidence

The ordinary acceptance feature proves exact mapping, fresh fallback, focused
override, the actual checked-in compatibility selection, and visible rejection
of bad policy. Unit tests cover malformed JSON, trailing input, unknown fields
and backends, unsafe request paths and missing policy files.

`scripts/run-matrix-policy-evidence.sh` runs the selected waiting-facts feature
on memory and forced fresh Synapse and compares ordinary outcomes. It then runs
the same feature's Gherkin mutations on both backends and compares total,
killed, survived and error counts. It also runs an unlisted feature through the
generated selector to prove it still uses fresh, and proves malformed and unsafe
policy fixtures fail.

The evidence records:

- before and after wall time for the selected feature;
- full QA wall time before and after;
- full harden wall time before and after;
- ordinary and mutation equivalence between memory and fresh;
- the unchanged fresh-Synapse compatibility lane.

The checked-in summary is `docs/evidence/waiting-facts-memory-policy.{json,md}`.
The ordinary full gates are the authority; the evidence script exists to
reproduce migration-specific comparison and timing.

## Boundaries

Changes are limited to acceptance fixtures, generated-test selection, test
scripts and their documentation. No other feature group migrates. Production
bridge behavior and deployment state do not change.

Because this card changes acceptance code only, its deploy evidence builds and
checks the command, adapter conformance and current live status without
stopping or restarting the live bridge. A restart is not part of this card.

## Acceptance

`features/waiting-facts-as-state.feature` runs on memory in routine acceptance
and when Gherkin mutation selects changed features. Every unlisted feature
selects fresh unless a focused-run override asks otherwise. Memory and fresh
produce the same ordinary and mutation classifications for the selected
feature. Full-gate timing is recorded and the real compatibility lane remains
required.

The policy example is a Scenario Outline because its mapping and request values
must be mutation-sensitive. Scenarios that only state a boolean safety outcome
are plain scenarios; the unsafe and malformed cases are value-driven and receive
independent unit coverage.

## E2E procedure

The card registers `qa/e2e.edn` and ships
`qa/adopt-memory-matrix-for-waiting-facts.{md,sh}`. The procedure runs the
selected waiting-facts feature on memory and fresh, checks the policy safety
contract, and validates the checked-in equivalence and timing summary. It needs
no operator-supplied secret.

## Documentation

- `README.md` records the checked-in per-feature policy and focused override.
- `docs/manual.md` documents exact mapping, fresh fallback, policy validation
  and the required fresh compatibility lane.
- `ARCHITECTURE.md` records generated-test backend selection and why production
  transport remains independent.
- `docs/manual-contract.edn` links the fixture policy API and evidence command
  to executable examples.

## Delivery record

- `acceptance/matrix-backends.json` selects memory only for
  `features/waiting-facts-as-state.feature`.
- Generated entry points resolve the feature backend and pass it explicitly to
  the acceptance registry; ordinary and mutation runs use the same selection.
- The global focused-run override remains available and wins over policy.
- Fresh Synapse remains the default for every unlisted feature, including the
  encryption, device-continuity and compatibility features.
- The Matrix policy evidence script and checked-in summary compare ordinary and
  mutation outcomes and retain before/after selected, QA and harden timings.
- The acceptance-only deploy check builds and validates the live command and
  adapter without restarting the bridge.
