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
