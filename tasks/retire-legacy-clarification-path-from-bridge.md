# retire-legacy-clarification-path-from-bridge

# Retire the legacy clarification path from the bridge

## Dependency

Follows verified `notify-typed-project-clarifications-through-bridge`. The
live rollout gate passed on 2026-11-17 for every configured Family:
`Forgelet`, `Saibill`, `Innersource` and `Ledgerhall` each served a valid
`automation-v1` Family projection with the typed clarification section,
lifecycle, attention and bounded excerpt contract present. The typed
notification, Matrix reply, project wake and app-marker-clearing acceptance
from the dependency is complete.

## Operator-observed problem

Typed project clarifications now have Matrix notification and reply parity, but
the bridge still derives legacy clarification notices and answers from the
legacy store. Keeping both authorities prolongs the old clarification system
and leaves a path that should be retired now that every configured Family is
migrated.

## Desired behaviour

- Use only typed project clarification rows in the bounded automation snapshot
  as the bridge's clarification notification authority.
- Remove bridge-owned legacy clarification notification derivation and legacy
  clarification answer routing.
- Preserve the existing Matrix Clarifications rooms, notification identities,
  deduplication state, event history and project/Family isolation.
- Replies continue through the owning project dashboard's normal project-chat
  path and wake only that project.
- A legacy-only, mixed, malformed or unsupported projection fails closed with a
  visible bounded status. The bridge does not revive the legacy store or
  manufacture notices.
- Remove the mixed legacy/typed bridge state and the rollout compatibility
  branches once every supported dashboard emits `automation-v1`.
- Existing typed notification records remain readable across restart. Obsolete
  legacy records for the authoritative room are reconciled away without
  deleting Matrix events.
- Preserve approvals, resolutions, card activity, chat and every other bridge
  behavior.
- Do not remove the shared-layer legacy store or its endpoints in this card.

## Testing

- Typed open/unseen rows notify once; open/seen, answered and withdrawn rows
  send no new notice and reconcile bridge state correctly.
- Repeated ticks and bridge restart do not duplicate a notice.
- Matrix replies answer through normal project chat, wake only the owning
  project and clear app attention.
- Same clarification ids across projects and Families remain isolated.
- Legacy-only, mixed, malformed and unsupported projections fail closed with a
  bounded status and no notice.
- Existing Matrix room history and stored typed notification marks remain
  readable; obsolete legacy bridge marks do not produce work.
- Approvals, resolutions, activity and non-clarification bridge behavior remain
  unchanged.

## Boundary

Work in `projects/forgelet-bridge`. Do not remove the shared-layer
clarification store/endpoints, delete Matrix rooms or history, infer questions
from prose, expose full project chat, or deploy before every configured Family
passes the typed live-contract preflight.

## Design

`internal/dashboard.AutomationState` keeps the legacy section only long enough
to validate that it is empty of pending work. A pending legacy row, including a
mixed legacy/typed projection, returns a bounded `LegacyClarificationError`.
The typed section remains required and non-truncated, and every row must retain
its project, clarification id, lifecycle, attention and bounded question
excerpt. Unsupported and malformed projections return their bounded errors
without the full-state compatibility fallback.

`automationSnapshot` derives only `DashboardSnapshot.TypedClarifications`; it
no longer copies legacy rows or records a projection fallback. The Family
projection is the sole typed notification authority for its open projects, so
a project-dashboard snapshot cannot create a second notice.

`internal/relay.PlanClarifications` receives typed rows only. Open/unseen rows
post once, open/seen rows remain answerable, and a Matrix answer is marked once
on the original notice. An authoritative answered, withdrawn or absent row
reconciles the project-qualified bridge record. Reconciliation also removes
obsolete legacy clarification records in that root and room; Matrix messages and
state history are untouched. Persisted `ClarificationState` fields remain
backward readable.

The bridge's clarification store surface becomes the typed answerer only.
`internal/dashboard.Clarifications.Answer` and the legacy answer endpoint remain
available to the shared layer, but the bridge no longer calls them. Typed
answers use `AnswerTyped`, which resolves exactly one marked owning project and
posts to that project's `/api/chat` with `clarification_id`.

The bridge binary gains a read-only typed-clarification preflight command. It
loads the configured bridge configuration, reads each Family's bounded
automation projection, requires a named Family identity and the same validation
used by the running bridge, and prints only bounded Family names and reasons.
`scripts/deploy-check.sh` runs it before stopping or starting the live adapter.

## Acceptance examples

Projection-shape behavior is a Scenario Outline over typed, legacy-only, mixed,
malformed and unsupported examples, each with mutable notice/status
expectations. Lifecycle/attention behavior remains a Scenario Outline over
open-unseen, open-seen, answered and withdrawn. Restart deduplication, Matrix
reply routing, project/Family isolation, state reconciliation and preservation
of approvals/activity are executable boolean scenarios; their fixed prose is
not behavioral input. The preflight has focused unit coverage plus the live
deployment procedure described below.

`phone-clarifications.feature` is explicitly exempt from acceptance mutation:
its surviving scenarios are boolean protocol checks whose question and answer
text is fixed prose, not an example table. The mutable value-driven behavior is
covered by the two Scenario Outlines in the card feature and the retained
typed-notification outlines. This exemption is why a zero-mutation report for
that focused boolean feature is expected.

## E2E procedure

`qa/retire-legacy-clarification-path-from-bridge.md` and its script run the
focused Gherkin through the real bridge, then run the typed live-contract
preflight against every configured Family and one real composed `Bridge.Tick`
with the production dashboard adapters. The Matrix side is the test room
boundary so the check does not mutate the operator's live rooms. The procedure
prints only Family names, counts and bounded results; it never prints dashboard
addresses, credentials or question bodies.

The operator acceptance remains the live end-to-end proof: create one typed
clarification, receive one Matrix notice, reply, observe only that project
wake and its app marker clear, restart the bridge and confirm no duplicate,
then confirm a legacy-only or malformed Family projection creates no notice.

## Documentation

- `README.md`: replace the mixed authority and rollout-fallback description
  with the typed-only clarification path and preflight.
- `docs/manual.md`: document typed-only notification, answer routing,
  fail-closed projection shapes, state reconciliation and preserved history.
- `ARCHITECTURE.md`: record the typed authority, typed answerer boundary,
  absence of compatibility fallback and deployment preflight.
- `docs/manual-contract.edn`: update the automated-state and clarification
  sections so their executable examples exercise the typed-only APIs.

## Verification

Pending implementation. Record exact Go, CRAP/DRY, architecture, mutation,
property/acceptance, E2E, documentation and deploy evidence here before
completion.

## Operator acceptance

- owner: operator | environment: deployed bridge, every configured Family, Matrix Clarifications rooms and the Forgelet app | checks: create one typed clarification, receive one notice, reply in Matrix, observe only the owning project wake and its app marker clear, restart without a duplicate, and confirm a legacy-only or malformed projection creates no notice | status: pending
