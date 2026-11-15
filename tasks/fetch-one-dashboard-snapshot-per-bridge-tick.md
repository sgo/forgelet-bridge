# fetch-one-dashboard-snapshot-per-bridge-tick

# Fetch one dashboard snapshot per bridge tick

Task ID: `20261114T220557883681Z-fetch-one-dashboard-snapshot-per-bridge-tick`

## Observed problem

The dashboards and bridge consume excessive CPU and transfer large amounts of
state even while little changes. On one bridge tick the forge dashboard is read
through `/api/state` once for pending approvals, again for desk resolutions,
and again for clarifications. Each project dashboard is read for its
clarifications as well. Measured on 2026-11-14, forge state responses were
0.18-1.0 MB and took about 0.12-0.30 seconds to construct. The Forgelet root
dashboard emitted roughly 109 GB over three days and Saibill roughly 11.5 GB
over two hours.

## Desired behaviour

- Read one authoritative dashboard-state response per configured dashboard per
  bridge tick.
- Derive pending approvals, desk-resolved approvals, clarifications and other
  consumers already present in that response from the same response.
- Reuse that response inside the tick when both forge-level and project-level
  work address the same dashboard. Never reuse it across ticks or between
  different dashboard identities.
- If a snapshot read fails, fail every consumer that needs it in that tick.
  Do not use a previous snapshot, synthesize empty state, or retry the read
  during the same tick. Chat, board activity and other independent forge work
  continue.
- Preserve Matrix approvals, desk resolutions, clarifications, waiting facts,
  room ordering, backoff and project isolation.
- Keep the `/api/state` payload contract unchanged.

## Design

`internal/dashboard` will decode the complete dashboard payload into one
snapshot value and derive the relay approval, resolution and clarification
values from it. A tick-scoped bridge cache will key snapshots by both forge
root and resolved dashboard address, so one address may serve both forge-level
and project-level consumers but no entry survives into another tick.

The bridge will read a snapshot once before its approval and clarification
consumers and pass the derived values down. Their action methods continue to
use the dashboard endpoints. The existing independent store interfaces remain
as the fallback for tests and adapters that do not provide snapshots.

## Acceptance examples

- An empty snapshot posts no waiting items.
- One snapshot containing a pending approval, a resolved previous approval and
  a pending clarification posts the new approval, carries the desk resolution
  to the old message, and posts the clarification, with one state request.
- A failed snapshot posts nothing for that tick, is reported as unreachable,
  and makes no same-tick retry. The next healthy tick recovers without stale
  state.
- Two forge roots and two project dashboards each receive one request in the
  same tick, and each room carries only its own dashboard's item.

The failure/recovery scenario is boolean and remains a plain scenario. The
state-content, resolution and isolation scenarios are Scenario Outlines with
value-driven examples.

## Measurement

A fixed synthetic run will use one 1 MiB state response across the former
three forge-dashboard reads and one project-dashboard read. It will record
request count, response bytes and process CPU for the pre-change call pattern
and the snapshot call pattern. Correctness and event delivery remain gate
requirements; the metric is not improved by suppressing work.

## End-to-end procedure

`qa/fetch-one-dashboard-snapshot-per-bridge-tick.md` will describe running the
fake-dashboard acceptance scenario and the synthetic measurement through the
real bridge binary. Its script will be registered in `qa/e2e.edn` with no
operator-supplied secrets.

## Documentation

- `README.md`: state that each configured dashboard is read once per tick.
- `docs/manual.md`: describe tick-scoped snapshot semantics, failures and the
  one-read guarantee.
- `ARCHITECTURE.md`: record the snapshot boundary and tick/identity scope.
- `docs/manual-contract.edn`: add an executable snapshot API example.

## Boundary

Work only in `projects/forgelet-bridge`. Do not change `/api/state`, edit an
active forge, or operate the live bridge until the reviewed tree is committed
and pushed. Deployment follows the required deploy gate.

## Verification

- `code`, `clean`, `architect`, `harden` and `clean-final` pass on the reviewed
  tree. Hardening reports zero Go survivors and zero Gherkin survivors.
- The full `qa` gate passes 212 scenario runs, including the snapshot, phone,
  restart, removal, ordering and isolation suites.
- The required `e2e` gate passes all four snapshot executions through the
  executable Gherkin pipeline and the fixed synthetic measurement.
- The fixed measurement uses 32 ticks and two 1 MiB dashboards. It records
  128 requests and 134,256,768 bytes before versus 64 requests and 67,128,384
  bytes after: 50% fewer requests and bytes. Repeated runs measured bridge CPU
  at 3.281s before and 1.813s after (44.7% less); the latest e2e run measured
  2.881s before and 1.493s after (48.2% less).
- `docs` passes its executable examples. `README.md` gained the per-tick
  dashboard guarantee. `docs/manual.md` documents snapshot sharing, failure,
  no-retry and tick-scope semantics plus the measurement.
  `ARCHITECTURE.md` records the forge-root/dashboard-identity boundary.
  `docs/manual-contract.edn` owns executable examples for both snapshot APIs.
- The required `deploy` gate passes after the reviewed tree was committed and
  pushed. The restarted live bridge reports Forgelet, Saibill, Innersource and
  Ledgerhall reached, no unavailable forges and zero owed items. A 30-second
  sample advanced from tick 5 to tick 11 using 0.83 seconds of bridge CPU
  (2.8% of one core).
- Operator acceptance remains pending.

## Operator acceptance

- owner: operator | environment: deployed bridge and Matrix clients | checks: approvals, clarifications and activity remain current while each configured dashboard receives at most one `/api/state` read per tick; deployment evidence reports requests, bytes and bridge CPU | status: pending
