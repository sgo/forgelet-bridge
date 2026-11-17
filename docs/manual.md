# Operator manual

This manual describes the current public behaviour of the Go bridge. It is
maintained alongside the executable Gherkin features and the package APIs.

## Installation and quick start

The project module is `github.com/unclebob/forgelet-bridge`. Build the commands
and run the bridge from the checkout:

```sh
./scripts/build.sh
./build/acceptance/bin/forgelet-bridge --config forgelet-bridge.json
```

The build output remains under `build/acceptance/bin`. A configuration can be
checked without starting the Matrix connection by adding `--validate`.

## Bridge operation

The `cmd/forgelet-bridge` process reads one JSON configuration, connects to the
configured Matrix homeserver, and serves each configured forge. It creates or
reuses the forge space and the chat, approvals, activity and clarifications
rooms. The status file under `state_dir` reports the device, current tick,
reached forges, unavailable forges, and the work still owed to each forge.

The bridge keeps relay facts in Matrix room state so a restart does not replay
delivered messages. A temporarily unavailable forge is reported under
`unhappy_forges` while the other forges continue.

A dashboard request may carry the optional safe `source` field. The exact value
`app.completed-card-report` identifies a completed-card report created by the
Forgelet app rather than a message the operator sent in Matrix. The bridge
posts that report under the bridge's own Matrix account with this exact
presentation:

```text
From the Forgelet app:

<the unchanged report body>
```

The posted event is stored as the request's thread root, so the Lieutenant's
answer is a reply under the explicit app-relayed report and survives restart
without replay. The bridge does not create another dashboard request to change
presentation. A missing source or any other source value keeps the existing
request body and pairing behavior. `internal/relay.Request.Source` carries the
metadata and `internal/relay.AppCompletedCardReportSource` names the one value
that triggers the app-relay presentation.

## Dashboard state snapshots

Each bridge tick reads `/api/automation-state` once from every dashboard it
addresses. `internal/dashboard.SnapshotSource` requires the
`X-SwarmForge-Projection: automation-v1` header and the matching projection
discriminator, validates the bounded shape and decodes one
`internal/dashboard.AutomationState` into one
`internal/relay.DashboardSnapshot`. `internal/dashboard.AutomationProjection`
names the accepted discriminator. `internal/bridge.SnapshotStore` shares that
value among approvals, resolved approvals, typed clarifications and project
board activity in the tick. Typed clarification rows are validated and carried
in `DashboardSnapshot.TypedClarifications`; `internal/relay.Clarification`
retains their project, stable clarification id, `open|answered|withdrawn`
lifecycle and `unseen|seen` attention. A pending legacy row, mixed
legacy/typed projection, malformed lifecycle, attention value, identity, blank
excerpt or truncated typed section fails the projection closed because a
partial sample cannot safely reconcile previously posted notices.

The tick cache is keyed by both the forge root and the resolved dashboard
address. Forge-level and project-level work that address the same dashboard
therefore share one response. The cache is discarded when the tick ends, so a
later tick never sees an earlier response. Two different forge roots never
share a dashboard result.

If the request fails, has the wrong projection header, or lacks a required
bounded field, every consumer that needs that snapshot fails for the tick. The
bridge does not retry in the same tick, substitute an earlier response or
manufacture empty data. Chat relay continues because it does not depend on the
projection. Unsupported endpoints and older projections no longer use an
`/api/state` compatibility read. They fail closed with a bounded `last_error`,
no stale notification work and no revival of the legacy clarification store.
Unknown resolutions and every malformed shape fail the same way. The shared
dashboard approval, send-back, clarification-answer and chat endpoints remain
available outside the bridge.

Resolved approvals retain the exact bounded value. Manual `approved` selects
the ➡ mark, manual `sent_back` selects ⬅, and the automatic endings
`automatic_spec_approval`, `auto_approved` and `superseded` retain their exact
value without a manual mark. A superseded ending is reported as "Superseded by
a newer proposal"; it is never guessed into either manual action. Clarification
questions come only from the bounded `question_excerpt`; blank excerpts fail
the projection closed.

An open typed clarification posts in the Family Clarifications room only while
its attention is `unseen`. The notification record has a typed,
project-qualified identity, so the same clarification id in another project or
Family remains distinct. The Family projection is the typed authority for its
open projects; a nested project-dashboard snapshot creates no second notice.
Open seen questions remain answerable without another post. A later snapshot
that reports the row as answered or withdrawn, or no longer reports it, removes
the typed notification record without posting stale work. Reconciliation also
removes obsolete legacy bridge records for that room while leaving every Matrix
message and state event in place. Repeated ticks and bridge restarts therefore
cannot duplicate an unseen notice, and a failed projection changes no
notification record.

`internal/dashboard.Clarifications.AnswerTyped` sends a Matrix answer to the
owning project's normal `POST /api/chat` endpoint as
`{"text":"...","clarification_id":"..."}`. It resolves exactly one marked,
open project beneath the configured Family; an unknown, duplicate, mismatched
or unmarked project fails the action closed. The bridge no longer calls the
legacy `/api/clarifications/<id>/answer` endpoint; that shared endpoint remains
available outside the bridge.

Deployment runs `forgelet-bridge --typed-clarification-preflight` before the
adapter stops or starts the service. It reads every configured Family's live
`automation-v1` typed section and requires a named Family identity. A failure
refuses the deployment before a mixed-authority bridge can start.

The fixed comparison in
`docs/evidence/consume-bounded-automation-state-in-bridge.json` records the
earlier measurement that motivated the bounded projection. Bytes fell from
67,114,048 to 1,424,096, a 97.9% reduction; measured process CPU fell from
1.513 seconds to 0.187 seconds, an 87.6% reduction. These synthetic figures
isolate transfer and decoding. The deployment preflight and operator acceptance
check the live bridge and Matrix delivery.
The typed clarification E2E record in
`docs/evidence/notify-typed-project-clarifications-through-bridge.json`
creates an unseen question through the real project dashboard, observes its
bounded redacted projection, answers through the Family adapter, and verifies
the project answer request and cleared attention marker.

## Configuration

The `internal/config.Config` type owns the bridge configuration. Its JSON
fields are the public `homeserver_url`, `user_id`, `password` or `access_token`,
optional `device_id`, `operator`, `forges`, and optional `state_dir`.

Each `forges` entry names a forge root and the name shown to the operator. The
root is also the key for that forge's persisted rooms and projects. Credentials
and configuration values must stay in runtime state and must never be committed.

The project adapter is `scripts/matrix-bridge.sh`. It derives the forge root
from its installed location and uses the forge's
`.swarmforge/matrix-bridge.json` unless `MATRIX_BRIDGE_CONFIG` overrides it.

## Forge removal

`swarmforge/scripts/matrix-bridge.sh remove-forge <forge-root>` is the adapter
entry point for the bridge's `remove-forge` operation. The
`internal/removal.Service` validates one configured root and the saved room
ids before stopping the bridge or changing Matrix. It clears relay facts,
removes the operator, and leaves and forgets each recorded room, persisting
per-room progress so a retry can continue.

The public result is one sanitized JSON object. Its status is `removed`,
`nothing_to_remove`, `retryable`, or `refused`. The adapter leaves the bridge
configuration and the forge files unchanged, and it keeps the bridge stopped
after a removal so the host can remove the configuration entry.

## Project removal

The project-removal host calls two public adapter operations. Planning is:

```sh
swarmforge/scripts/matrix-bridge.sh remove-project-plan <forge-root>
<project-name>
```

The adapter validates arity, the exact absolute configured root, the root's
`.swarmforge` directory, the safe project name and its
`projects/<name>/.swarmforge/single-agent` marker before invoking
`cmd/forgelet-bridge --remove-project-plan`. Unsafe or extra arguments return
`unsafe_root`; other unconfigured or non-Forgelet roots return `unknown_root`;
unsafe and unmarked projects return `unsafe_project` and `unknown_project`.
These refusals invoke neither the bridge binary nor Matrix.

For a valid argument, a `ready` plan is printed unchanged. The adapter also
accepts the bridge's `removed`, `nothing_to_remove`, `retryable` and `refused`
objects after reducing them to `status` and an optional lowercase stable
`reason`. Malformed, empty, multi-object or unknown-status output becomes
`{"status":"retryable","reason":"invalid_plan"}`; a nonzero binary exit becomes
`{"status":"retryable","reason":"state_unavailable"}`. Planning never stops or
restarts the bridge, connects to Matrix, or changes bridge state, configuration,
project files, logs or restart markers.

`swarmforge/scripts/matrix-bridge.sh remove-project <forge-root>
<project-name>` retires one stopped, managed single-agent project without
removing its Family space, its forge-level rooms, any sibling project, or the
project's server-side Matrix history.

`internal/removal.Service.RemoveProject` owns the state transition. The
`ProjectCatalog` boundary resolves the configured Forgelet root and managed
project marker before `ProjectCleaner` is allowed to change Matrix.

The root must be the exact absolute root configured for a Forgelet forge, and
the project name must match the managed
`projects/<name>/.swarmforge/single-agent` marker. Relative or traversing roots,
unknown or non-Forgelet roots, unsafe or unknown project names, and incomplete
or colliding saved project-room state are refused before Matrix changes. A
caller cannot supply a Matrix room id; the room id comes only from bridge state.

For a ready project, the bridge clears that project's `com.forgelet.*` facts,
removes the operator, and leaves and forgets the bridge membership. It then
removes only the project's bridge record and project-scoped relay bookkeeping.
Each stage is saved as completed so a retry resumes at the first unfinished
stage. No administrator purge, room deletion, or event deletion is issued.

The result is one sanitized JSON object with `status` set to `removed`,
`nothing_to_remove`, `retryable`, or `refused`. Stable reasons include
`unsafe_root`, `unknown_root`, `unsafe_project`, `unknown_project`,
`incomplete_project_state`, `relay_cleanup_failed`,
`operator_removal_failed`, `bridge_membership_failed`, `state_unavailable`,
`matrix_unavailable`, and `bridge_restart_failed`.

The combined `remove-project` operation performs its own planning before
stopping a running bridge and restarts a bridge it stopped after every apply
attempt, including retryable Matrix failures. It records a restart debt when
that restart fails, and the next project-removal invocation retries it. A
second successful removal while the project directory is still present returns
`nothing_to_remove` and touches no Matrix room. The bridge does not move the
directory, change the forge's open-project list, or edit bridge configuration.

`scripts/check-adapter-conformance.sh` verifies both host-facing commands in an
installed adapter's usage and dispatcher. The deploy gate runs it before
restarting the live bridge.

## Project pipeline

`scripts/stage-gate.sh` is the single entry point for project gates. It selects
the project's native scripts:

- `code` runs `scripts/test.sh`.
- `clean` and `clean-final` run `scripts/check-clean.sh`, which invokes
  `crap4go` with the Go test command and `dry4go`.
- `architect` runs `scripts/check-architecture.sh`, which combines `go vet`
  with the package-boundary checks.
- `harden` runs `scripts/check-mutation.sh`, differential `mutate4go` over
  changed Go functions followed by Gherkin mutation through the one acceptance
  pipeline.
- `qa` runs `scripts/property.sh` and `scripts/acceptance.sh`.
- `docs` runs `scripts/check-documentation.sh`.
- `deploy` runs `scripts/build.sh`, checks that the live process accounts for
  every configured forge, exposes the removal operations, and dispatches both
  host project-removal commands. Ordinary cards then restart
  `scripts/matrix-bridge.sh`; a card whose approved task note has
  `## Acceptance-only deploy` verifies and leaves the current live process
  running.

Every case writes `build/stages/<stage>.log`. No Maven reactor or second Java
acceptance harness participates in this project.

## Acceptance and verification

`scripts/acceptance.sh` parses every feature with `gherkin-parser`, generates
entry points with the Go generator, starts the fixture homeserver, and runs the
real bridge through the handlers in `acceptance/steps`. The
`acceptance/runtime.Registry` requires every Gherkin step to match exactly one
handler. Mutation uses the same runtime and acceptance entry points.

The property suite is separate from unit coverage:
`scripts/property.sh` runs tests tagged `property`. Unit tests use
`scripts/test.sh` and the `goolm` build tag. Documentation checks execute the
examples declared in `docs/manual-contract.edn`.

## Matrix fixture backends

`acceptance/matrix-backends.json` is the test-only per-feature policy.
`acceptance/fixtures.MatrixBackendForFeature` resolves an exact
`features/*.feature` path against it before a generated entry point starts any
scenario world. A listed feature uses its selected backend; an unlisted feature
uses `fresh`. The initial policy maps only
`features/waiting-facts-as-state.feature` to `memory`.

`FORGELET_MATRIX_BACKEND=fresh|shared|memory` remains the focused-run override
and wins over the policy. `fresh` preserves one pinned real Synapse process per
scenario. `shared` uses one real Synapse for the generated acceptance test
process; each scenario still gets its own rooms and a unique bridge device.
`memory` starts the in-process Matrix Client API fixture from
`acceptance/fixtures.StartMemoryHomeserver`.

Policy validation happens before backend use in
`acceptance/fixtures.ResolveMatrixBackend`. Missing, malformed or trailing JSON,
a missing `features` map, unknown fields or backend names, and absolute,
traversing or nested feature paths fail the generated test.
`acceptance/fixtures.RequiredFreshFeatures` names the compatibility lane, and
the resolver rejects an attempt to map those required fresh-Synapse features to
another backend. A focused override is still deliberate and explicit.

All three implement the `acceptance/fixtures.Homeserver` boundary. The real
bridge process, forge dashboard, fixture clients and Olm encryption remain in
use when `memory` is selected; the fake relays opaque encrypted content and
implements registration/login, sync, rooms, state, messages, threads,
memberships, device keys, one-time keys and to-device traffic.

`scripts/run-matrix-prototype.sh` measures
`features/forge-display-names.feature` three times on every backend and runs
the same eight Gherkin mutations on each. It writes machine-readable evidence
to `target/acceptance-matrix-prototype/` and maintains
`docs/evidence/matrix-backend-prototype.{json,md}`.

`scripts/run-matrix-policy-evidence.sh` runs the waiting-facts feature through
the policy-selected memory backend and forced fresh Synapse, compares ordinary
and mutation outcomes, and checks that an unlisted feature is still fresh. It
maintains `docs/evidence/waiting-facts-memory-policy.{json,md}` with the
selected-feature, full QA and full harden before/after timings.

## Extension points

New bridge commands belong under `cmd/`, policy and durable questions belong in
`internal/`, and Matrix transport behaviour belongs only in `internal/matrix`.
New acceptance vocabulary belongs in `acceptance/steps` and must call the same
public path a user exercises. Mutation manifests are tool-owned and are
committed with the code they describe.
