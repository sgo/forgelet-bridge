# consume-bounded-automation-state-in-bridge

# Consume bounded automation state in the bridge

## Dependency

Depends on verified proposal `proposal-20261115T072142048531Z`
(`serve-bounded-dashboard-state-projections`). The bridge must consume the
delivered `/api/automation-state` contract, not invent a projection of its own.

## Operator-observed problem

The bridge now reads one dashboard state per configured dashboard per tick, but
that state is still the full Family payload of roughly 0.18-1 MB. The shared
dashboard now offers the bounded `automation-v1` projection, which is much
smaller and contains the approvals, resolved approvals, legacy and typed
clarifications, boards/cards and activity facts the bridge needs.

## Desired behaviour

- Replace bridge `/api/state` reads with `/api/automation-state` wherever that
  projection contains every field the consumer needs.
- Continue to make at most one read per configured dashboard per tick and
  share it among all consumers.
- Validate the projection header/shape and fail that dashboard's tick safely if
  malformed or unavailable.
- Negotiate a safe rollout: an older dashboard may return 404/unsupported;
  report that state explicitly and do not silently fabricate data. A temporary
  compatibility fallback is allowed only if it remains bounded and is removed
  after the controlled dashboard rollout.
- Preserve Matrix approvals, clarifications, facts, board/activity notices,
  ordering, retries and Family isolation.
- Do not copy arbitrary transcript, paths, commands or credentials from the
  projection.

## Testing and measurement

- Fake dashboards count endpoint calls and prove one projection read per tick
  with all consumers sharing it.
- Cover approvals, resolved approvals, legacy and typed clarifications, cards,
  malformed projection, unsupported endpoint, recovery and multiple Families.
- Prove the existing Matrix event behavior is unchanged.
- Measure full-state versus automation bytes and bridge CPU on the fixed
  synthetic workload.
- Run all project gates, deploy and verify all configured Families remain
  reached with zero owed work.

## Boundary

Work in `projects/forgelet-bridge`. Do not change the dashboard projection
contract, operate active dashboards, weaken stale/error handling or remove the
legacy clarification path in this card.

## Design

`internal/dashboard` will read `GET /api/automation-state` for every
tick-scoped snapshot. It will require the `X-SwarmForge-Projection:
automation-v1` header, the matching projection discriminator, the required
sections, exact manual and automatic resolved-approval values, and bounded
sections before deriving bridge values.

The derived snapshot will carry pending approvals, resolved approvals, legacy
clarifications, typed clarification rows for the following notification-parity
card, and project boards. Only the safe fields named above are retained.
Question text comes only from `question_excerpt`. Matrix activity will use the
project/card fields from the same snapshot instead of reading the board files
again.

The existing independent stores remain the fallback for adapters and tests that
do not provide snapshots. An unsupported or malformed automation endpoint is
reported as an unavailable dashboard: this card does not silently fall back to
`/api/state`, retry in the same tick, or invent empty data.

The bridge will preserve the exact resolved ending. Only manual `approved` and
`sent_back` select a manual ➡/⬅ reaction; `automatic_spec_approval` and
`auto_approved` remain automatic and get no manual mark.

## Acceptance examples

All four authoritative approval endings are Scenario Outline examples.
Approval, resolved approval, legacy clarification, typed clarification and card
activity sharing one projection is a plain boolean scenario. Malformed
projection failure with recovery and unsupported-endpoint refusal are plain
boolean scenarios with no mutable example cells.

## Measurement

The fixed synthetic workload will send the same full-state and automation
content over a controlled number of bridge ticks. It will record endpoint
requests, response bytes and bridge CPU for the full-state path and the bounded
automation path. Correctness and event delivery remain gate requirements.

## End-to-end procedure

`qa/consume-bounded-automation-state-in-bridge.md` and its script will run the
card's Gherkin through the real bridge and the fixed synthetic measurement. The
procedure uses fixture roots only and requires no operator-supplied secrets.

## Documentation

- `README.md`: describe the automation projection as the bridge's state source.
- `docs/manual.md`: document validation, exact resolution policy, bounded
  sections, typed-row handoff and unsupported-endpoint behavior.
- `ARCHITECTURE.md`: record the projection, validation and shared tick boundary.
- `docs/manual-contract.edn`: own executable examples for the projection API
  and measurement.

## Verification

Pending implementation and gates.

## Operator acceptance

- owner: operator | environment: deployed bridge and Matrix clients | checks: approvals, clarifications, card activity and facts remain current while each configured dashboard receives one `/api/automation-state` read per tick and measured response bytes and bridge CPU fall | status: pending
