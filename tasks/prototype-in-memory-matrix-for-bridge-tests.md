# prototype-in-memory-matrix-for-bridge-tests

# Prototype an in-memory Matrix homeserver for bridge acceptance tests

## Operator-observed problem

The bridge's unit and property suites are fast, but its end-to-end gates are
slow enough to dominate card completion time:

- unit tests: about 12s;
- property tests: about 8s;
- full acceptance: 835s across 202 scenario executions, about 4.1s each;
- Gherkin mutation: 567s for 24 mutants, about 23.6s each.

Every acceptance scenario creates fresh forge roots and a fresh Python Synapse
homeserver. The bridge under test is a real process, so this is primarily
external-test-infrastructure overhead rather than slow bridge logic.

## Desired outcome

Produce an evidence-backed prototype of a faster acceptance harness without
weakening coverage:

- instrument one representative process-heavy feature so fixture setup, bridge
  startup, scenario execution and teardown are timed separately;
- extract a homeserver fixture boundary shared by the real Synapse server and an
  in-process Matrix fake;
- implement enough of the Matrix Client API for that representative feature:
  registration/login, sync, rooms, messages, threads and any state or reaction
  behavior the feature uses;
- run that feature against (a) the current fresh Synapse server, (b) one shared
  real Synapse server with isolated users and rooms, and (c) the in-memory
  fake;
- record wall time, resource use, stability across repeated runs, ordinary test
  outcome and Gherkin mutation outcome for all three;
- recommend whether the fake, the shared server, both for different scenario
  classes, or neither should be adopted.

The bridge process, forge dashboard and adapters remain real where those
boundaries are part of the test. Do not replace the system under test.

## Coverage that must remain real

Olm encryption, device keys, encrypted event handling and compatibility with
the Matrix client are genuine coverage. Keep them on real Synapse. A smaller
real-Synapse lane must continue to exercise a representative end-to-end path
even if the fast fake is adopted for other scenarios.

An in-memory fake must not merely accept everything. For the prototype feature,
the same scenario and mutations must have equivalent pass/fail results against
the fake and real Synapse. Any mismatch is a reason to reject or narrow the
fake, not to waive the assertion.

## Testing and evidence

- Record baseline timings for the selected feature.
- Prove each backend reaches the same scenario outcomes.
- Prove all mutations for the selected feature have the same killed/survived
  classification.
- Repeat the feature enough times to expose obvious ordering or timing flakes.
- Preserve all existing gates and encryption coverage.
- Keep the fast backend test-only and opt-in; production bridge behavior must
  not change.

## Out of scope

- Migrating the entire acceptance suite in this card.
- Replacing the bridge's Matrix client or encryption implementation.
- Parallelizing scenarios, filtering mutation jobs or changing the bridge tick
  interval; those remain separate experiments after the fixture measurements.
- Removing the real Synapse compatibility suite.

## Deliverable

One selected feature running through the three backends, timing and equivalence
evidence, and a concrete recommended follow-up plan. The prototype may remain
as an opt-in fixture if it passes; wholesale adoption needs a separate measured
card.

## Selected feature and backend contract

`features/forge-display-names.feature` is the representative selection. Its
four process executions use two forge roots and two dashboards, provision two
forges through one bridge, check room/member state, relay encrypted dashboard
requests, and return encrypted threaded answers. Its two value-driven outlines
provide eight Gherkin mutation cells.

The acceptance fixture selects a backend with
`FORGELET_MATRIX_BACKEND=fresh|shared|memory`:

- `fresh`, also the default, preserves today's one pinned Synapse process per
  scenario;
- `shared` starts one real pinned Synapse per acceptance test process and gives
  every scenario fresh users, rooms, tokens and devices;
- `memory` starts the in-process test-only Matrix server.

The real bridge, forge dashboards and adapters remain processes in every lane.
Production code does not read the backend setting and no production bridge
behaviour changes.

The homeserver fixture boundary exposes the same URL, registration and close
contract to Synapse and the in-memory server. The in-memory implementation must
reject unknown tokens, duplicate room joins, unknown rooms and missing devices
instead of accepting every request. It relays opaque Olm ciphertext and must
support the key upload/query/claim and to-device paths needed for real
`mautrix-go` clients to encrypt and decrypt.

## Evidence contract

`scripts/run-matrix-prototype.sh` runs the selected feature repeatedly through
all three backends and records:

- pass/fail for every execution;
- per-world total, fixture setup, bridge startup, scenario and teardown times;
- whole-run real/user/system time, maximum resident size and captured output
  size;
- the Gherkin mutation total, killed, survived and error counts for each
  backend.

The evidence lands under `target/acceptance-matrix-prototype/` and a checked-in
summary lands under `docs/evidence/`. The summary must recommend adoption,
narrowing or rejection from the measured outcomes. A fake survivor, backend
outcome mismatch, infrastructure error or unstable repeated run blocks the
card.

## Acceptance

`features/in-memory-matrix-acceptance.feature` is the executable contract for
the opt-in backend. Its value-driven scenario proves encrypted dashboard
requests and threaded answers through the in-memory server while still running
the real bridge and dashboard.

The evidence script additionally runs `forge-display-names.feature` under fresh,
shared and memory backends. All three must return the same ordinary and
mutation classification. The existing fresh Synapse acceptance and encryption
coverage remain required.

## E2E procedure

The card registers `qa/e2e.edn` and ships
`qa/prototype-in-memory-matrix-for-bridge-tests.{md,sh}`. The procedure runs the
full three-backend evidence experiment and verifies that the generated summary
matches the evidence, includes resource and stability data, and recommends a
follow-up rather than changing the production default.

## Documentation

- `README.md` records that the fast backend is test-only and opt-in.
- `docs/manual.md` documents the backend selector, isolation limits and the
  real-Synapse compatibility lane.
- `ARCHITECTURE.md` records the homeserver fixture boundary and keeps production
  transport unchanged.
- `docs/manual-contract.edn` links the fixture selector to an executable
  example.

## Delivery record

- `acceptance/fixtures.Homeserver` is the shared URL/registration/backend
  boundary for fresh Synapse, shared Synapse and memory.
- `acceptance/fixtures.MemoryHomeserver` relays opaque encrypted events and
  implements registration/login, sync, rooms, memberships, state, messages,
  threads, device keys, one-time keys, to-device traffic and account data used
  by the selected feature.
- The default `fresh` backend is unchanged. `shared` uses one server per test
  process with unique bridge devices; `memory` is in-process and test-only.
- Per-world timing records total, fixture setup, bridge startup, scenario and
  teardown separately.
- `features/in-memory-matrix-acceptance.feature` runs the opt-in memory lane
  through the real bridge and encrypted fixture clients.
- `scripts/run-matrix-prototype.sh`, `qa/e2e.edn` and
  `docs/evidence/matrix-backend-prototype.{json,md}` reproduce and publish the
  three-backend measurement.
- `README.md` records the opt-in selector. `docs/manual.md` owns isolation,
  resource and compatibility details. `ARCHITECTURE.md` records the fixture
  boundary. `docs/manual-contract.edn` adds an executable homeserver API
  example.
