# remove-forge-from-bridge

# Task

## Operator-observed problem

Forgelet can create forges, but one forge row cannot be removed. Removing only
the host catalogue entry or the bridge JSON entry leaves the forge's Matrix
space and project rooms on the phone. The bridge currently serves only the
forges still in configuration and has no lifecycle for an unconfigured forge's
persisted room state.

The operator wants removal to clear the forge from the phone as well as from
Forgelet. The forge directory must be retained for recovery.

## Desired behaviour

Add an operator-invocable forge removal path to `forgelet-bridge`.

Given one configured forge root, the bridge must:

- refuse an unknown, unsafe or ambiguous root;
- read the forge and project room ids from its persisted state;
- clear the forge's relay facts from each relevant room where possible;
- remove the operator and the bridge from the forge space, chat/activity/
  approval/clarification rooms and every recorded project room;
- leave or forget the bridge's memberships so the rooms no longer remain active
  for the bridge or operator;
- remove the forge and its project room records from bridge state only after the
  room operations are complete;
- leave the forge filesystem untouched;
- keep bridge configuration unchanged; the host owns removing its entry after
  this operation succeeds;
- return a safe, machine-readable result and preserve enough progress to retry
  after a partial Matrix failure without losing the room ids.

The command must be callable through the project-owned bridge adapter used by
`forgelet-host`. It must not require the host to know Matrix room ids or edit
bridge state JSON directly.

## Acceptance

- Fixture rooms prove every recorded forge and project room is targeted exactly
  once.
- The operator and bridge are removed from each room; the bridge then leaves or
  forgets its membership.
- Relay state for the forge/project is cleared only after successful room
  cleanup.
- A partial failure returns a sanitized retryable result and preserves the
  records needed for retry.
- The forge directory and bridge JSON remain byte-for-byte untouched by the
  bridge operation.
- Repeating the operation after success is idempotent and reports nothing left
  to remove.
- Unknown roots, incomplete state and unsafe arguments are refused before any
  Matrix mutation.
- Unit, property, acceptance, PIT, differential Gherkin mutation,
  documentation and deploy gates pass.

## Out of scope

- Deleting or moving the forge directory.
- Editing the `forgelet-bridge` JSON configuration.
- The host removal endpoint.
- The desktop removal button.
- Homeserver-admin room purge.

## Specification

The public command is the project-owned adapter command
`matrix-bridge.sh remove-forge <forge-root>`. It accepts exactly one root that
identifies one configured forge. It validates and reads the persisted forge
record before stopping a running bridge. After a valid removal request, it stops
the bridge before changing Matrix and leaves it stopped so the host can remove
the configuration entry and start it again. It does not restart a bridge whose
configuration still names the removed forge.

The command writes one JSON object to standard output. `status` is one of
`removed`, `nothing_to_remove`, `retryable`, or `refused`; a non-removed result
may carry a stable `reason`. The object carries no Matrix room or event ids,
user ids, credentials, error text from the homeserver, or filesystem paths.

For a complete removal, the bridge first reads all recorded room ids. It then
targets each distinct room exactly once. In each room it clears the
`com.forgelet.*` relay facts, removes the operator, and leaves or forgets its own
membership. Only after every room succeeds does it clear the forge's relay
bookkeeping and remove the forge and project records from bridge state. A
failure returns `retryable` and retains every room id plus per-room cleanup
progress, so a later invocation resumes without losing the rooms it has not
finished.

The validation boundary is before the first Matrix mutation. A root that is not
configured is `unknown_root`; a blank, relative, or parent-traversing root, or
one with extra arguments, is `unsafe_root`; a root whose identity does not name
exactly one configured forge is `ambiguous_root`; and a configured forge with a
partial persisted forge record is `incomplete_state`. A configured root with no
persisted forge record is already removed and returns `nothing_to_remove`.

The bridge configuration and every forge file outside the bridge state
directory retain their exact bytes. In particular, no project, worktree,
dashboard file, task note, or adapter copy is changed. The bridge state must
change on a successful removal and may change to record retry progress after a
partial failure; it is the lifecycle record this command owns.

The scenarios in `features/remove-forge-from-bridge.feature` are the acceptance
contract. Scenario 1 is an outline over the number of recorded project rooms.
Scenario 2 is an outline over each failure boundary and proves retry. Scenario 6
is an outline over unsafe and ambiguous argument forms. The remaining scenarios
are boolean lifecycle cases and intentionally have no example table.

## Implementation record

- `README.md` documents the public removal command, results, preservation
  boundary, and retry lifecycle.
- `docs/adding-a-forge.md` records the operator runbook, validation rules,
  stopped-bridge handoff to the host, idempotence, and safe result shape.
- `ARCHITECTURE.md` and the standard `docs/manual.md` / manual-contract layout
  are not present in this multi-agent project; no such files were invented.
