# Notify typed project clarifications through the bridge

## Dependency

Depends on verified proposal `proposal-20261115T072138806252Z`
(`show-project-clarification-attention-in-app`). App and host attention
projection must be complete before notification parity can be proved.

## Operator-observed problem

Typed clarifications now live in project chat and the app shows project-button
attention. The Matrix bridge still reads and relays the legacy clarification
store. Until typed clarifications produce equivalent operator notification and
reply behavior, the legacy path cannot be retired and some operators may miss a
project question when they are not watching the app.

## Desired behaviour

- Read the bounded typed clarification entries from
  `/api/automation-state`, including project, clarification id, lifecycle,
  attention and bounded question text.
- Notify the operator through the existing Clarifications room when a typed
  clarification is unseen. Do not send the same clarification again after
  restart or repeated ticks.
- A seen-but-unresolved clarification is not a new notification. Answered or
  withdrawn clarifications generate no pending notice and clear the bridge's
  notification record.
- Operator replies through the notification route must use the normal
  project-chat delivery path so the accepted reply marks the clarification
  answered and wakes the project agent.
- Preserve bounded redaction: no paths, commands, credentials or unrelated
  transcript content.
- Keep the legacy clarification notification/answer path operational until this
  card proves parity. Do not remove it here.
- Reconcile notification identity so duplicate clarification ids, project
  collisions and stale rows fail closed.

## Testing

- One unseen typed clarification produces exactly one Matrix notification.
- Repeated ticks and bridge restart produce no duplicate.
- Seen-but-unresolved, answered and withdrawn states send no new notification
  and update/clear bridge state correctly.
- An operator reply reaches the intended project chat, marks the clarification
  answered and wakes only the owning project.
- Cover multiple Families/projects, same clarification ids across projects,
  malformed entries, projection failure, recovery and bounded redaction.
- Retain all existing legacy clarification tests.

## Boundary

Work in `projects/forgelet-bridge`. Do not delete the legacy clarification
store or Matrix path, maintain a second clarification authority, infer
clarifications from prose, or expose full project chat.

## Design

The existing tick-scoped `automation-v1` snapshot remains the only authority.
Typed clarification rows retain their bounded project, clarification id,
lifecycle and `unseen|seen` attention. The legacy pending clarification rows
remain in their current lane.

Typed notification identity is type-distinct as well as project-qualified
within a forge root, while the same id in a different project or Family remains
distinct. Duplicate ids for the same project, mixed legacy/typed keys, invalid
lifecycle, invalid attention and truncated typed sections fail the dashboard
tick closed.

The existing Clarifications room and durable clarification notification state
carry typed notices. A typed row posts only while it is `open` and `unseen`.
`open` plus `seen` remains answerable without another notice. A missing,
answered or withdrawn authoritative row clears the typed notification record;
an unchanged projection failure leaves no new work and cannot create a stale
notice on recovery. The Family projection is the typed notification authority
for its open projects, so consuming the same projected row again from a project
dashboard cannot create a second notice.

Typed answers use the owning project dashboard's normal `POST /api/chat` path
with `clarification_id`, rather than the legacy clarification answer endpoint.
Project discovery must resolve exactly one owner inside the configured Family;
otherwise the answer fails closed. The legacy answer endpoint remains in use
for legacy rows.

Only the projection's bounded `question_excerpt` is copied into Matrix. The
bridge does not read project chat bodies, paths, commands, credentials or
unrelated conversation.

## Acceptance examples

Lifecycle and attention behavior is a Scenario Outline over open-unseen,
open-seen, answered and withdrawn examples. Malformed lifecycle, attention and
identity failures are another outline. Restart deduplication, projection
failure and recovery, operator reply delivery, state clearing, same-id project
isolation, multiple Families and legacy coexistence are executable boolean
scenarios; their fixed prompt/title text is not behavioral input.

## End-to-end procedure

`qa/notify-typed-project-clarifications-through-bridge.md` and its script will
run the card Gherkin through the real bridge. The procedure also creates one
typed question through the project's normal `clarify-chat` command, receives it
through the bridge, answers it through normal project chat, and verifies the
project agent wake and cleared app attention. No operator secret is required.

## Documentation

- `README.md`: describe typed clarification notification source and routing.
- `docs/manual.md`: document lifecycle, attention, identity, restart and answer behavior.
- `ARCHITECTURE.md`: record the snapshot authority and project-chat answer boundary.
- `docs/manual-contract.edn`: own executable clarification APIs and examples.

## Verification

Pending implementation and gates.

## Operator acceptance

- owner: operator | environment: one live project dashboard, bridge Matrix rooms and app | checks: one unseen typed clarification produces one notification, replying in Matrix answers it and wakes the project, the app marker clears, and repeated ticks plus a bridge restart do not duplicate the notice | status: pending
