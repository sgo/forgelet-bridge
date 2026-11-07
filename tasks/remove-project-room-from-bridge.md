# remove-project-room-from-bridge

# Task

## Operator-observed problem

Forgelet Host can stop one project runtime, but the bridge has no operation for
retiring that project's Matrix room. Removing a whole forge is too broad:
project removal must leave the Family space, its chat, approvals, activity and
clarifications rooms, and every sibling project intact.

The host will call the bridge before moving the stopped project's directory to
the operating system Trash. The bridge therefore owns only the Matrix and
bridge-state portion of that lifecycle. It must not move the directory, edit
the Forgelet Host, add an HTTP endpoint, or add app UI.

## Desired behaviour

Add one project-removal command to `cmd/forgelet-bridge` and one operation to
the installed `scripts/matrix-bridge.sh` adapter. Both accept one configured,
absolute Forgelet forge root and one exact managed single-agent project name.
The caller never supplies a Matrix room id.

The adapter first runs a read-only planning phase. It stops the bridge only
when the plan is ready and the bridge is currently running. It always attempts
to restart a bridge it stopped after the apply attempt, including a retryable
Matrix failure. If restart fails, it reports `retryable` with the stable reason
`bridge_restart_failed`, leaves the log/status available, and records that a
restart is owed so the next project-removal invocation retries the restart.

For a valid project the bridge:

1. clears only that project's `com.forgelet.*` relay facts;
2. removes the operator from the project room;
3. leaves and forgets the bridge membership in that room;
4. makes no administrator purge, room deletion or event deletion request;
5. after every Matrix operation succeeds, removes only the project's bridge
   record and relay bookkeeping, preserving the Family space, its four
   forge-level rooms, sibling project rooms and sibling relay facts.

The bridge state records each completed Matrix stage on the project's record,
so a retry starts at the first unfinished stage. The project record remains
until all Matrix stages succeed. If final state saving fails, a retry can reuse
the completed stages and finalize without touching Matrix again.

The public result is exactly one sanitized JSON object. `status` is one of
`removed`, `nothing_to_remove`, `retryable`, or `refused`. A non-removed result
may contain a stable `reason`. The result never contains Matrix room or event
ids, user ids, credentials, filesystem paths, project names echoed from unsafe
arguments, or homeserver error text.

## Specification

The public adapter operation is:

```sh
swarmforge/scripts/matrix-bridge.sh remove-project <forge-root> <project-name>
```

The corresponding process flags are `--remove-project-plan <forge-root>` with
the project name as its one positional argument, and `--remove-project
<forge-root>` with the project name as its one positional argument. The plan
prints `ready` when apply may proceed and otherwise prints the same sanitized
JSON result the command would return.

Validation happens before any Matrix mutation. A blank, relative,
parent-traversing, duplicate-argument, or otherwise unsafe root is
`unsafe_root`. A root that is not the exact configured absolute root, is not a
Forgelet root, or is otherwise not available as a configured forge is
`unknown_root`. A project name that is blank, padded, absolute, contains a path
separator or NUL, is `.` or `..`, or looks like a Matrix room id is
`unsafe_project`. A safe name with no managed single-agent project directory
under that forge is `unknown_project`. A recorded project whose room id is
missing, a forge record missing one of its required room ids, or a project room
id shared with a forge-level room or another project is
`incomplete_project_state`.

A configured Forgelet root and managed project with no project record returns
`nothing_to_remove`. This makes the second removal idempotent while the project
directory is still present for the host to move. Bridge configuration and every
project/forge file except bridge state stay byte-for-byte unchanged.

Matrix stage failures use `relay_cleanup_failed`, `operator_removal_failed`, and
`bridge_membership_failed`. State and process lifecycle failures use
`state_unavailable`, `matrix_unavailable`, and `bridge_restart_failed`. A
configuration that cannot be loaded is `configuration_unavailable`.

The bridge must not recreate the project room while removal is incomplete. A
project record with cleanup progress is skipped by the project relay until its
record is finalized. The host's precondition is that the project runtime has
been stopped and the project is no longer listed as open; the bridge does not
enforce or mutate the forge's open-project list.

The scenarios in `features/remove-project-room-from-bridge.feature` are the
acceptance contract. The success, failure and refusal cases use example tables
so acceptance mutation has value-driven cells. Lifecycle boolean scenarios stay
plain and are identified in this note.

## Acceptance

The executable scenarios cover:

- successful removal and inactive target-room membership;
- failure at each Matrix stage, retained progress, retry and resume;
- idempotent second removal;
- planning before a stop and bridge restart after success or retryable failure;
- exact configured-root and managed-project enforcement;
- unsafe roots and project names;
- incomplete project-room state;
- exact target-room targeting, sibling-room and sibling-relay preservation;
- preservation of the Family space and every forge-level room;
- sanitized results and server-side history retention.

Unit and property coverage independently prove room-id collisions, root
normalization, project-local relay selection, the three-stage retry state
machine, and that no operation can select a sibling project's room.

## E2E waiver

This card deliberately adds no app UI or Forgelet Host HTTP operation. Its
user-facing path is the installed bridge adapter, which the acceptance suite
already runs end to end against a real Synapse homeserver and real Matrix
clients. A separate UI e2e would require the host and app work named out of
scope by the card.

## Documentation

- `README.md` documents the adapter command, validation/result contract, and
  stop/restart lifecycle.
- `docs/manual.md` documents the detailed public operation, refusal reasons,
  progress/resume behavior, and preservation boundary.
- `ARCHITECTURE.md` records the project-removal Matrix boundary and the
  project-scoped relay-state decision.
- `docs/manual-contract.edn` gains an executable project-removal API example.

## Implementation record

- `cmd/forgelet-bridge` exposes plan/apply flags, and the installed adapter
  plans, stops only a running bridge, applies, and restarts it with a persisted
  restart-debt marker on restart failure.
- `internal/removal` validates configured Forgelet roots and managed project
  markers, resolves the room only from bridge state, persists all three Matrix
  stages, resumes at the first unfinished stage, and purges only target relay
  bookkeeping.
- `internal/state` and `internal/relay` carry project cleanup progress and
  project owner scope. `internal/matrix` clears the project room, its activity
  board fact, and only its approval facts.
- Unit/property coverage includes unsafe roots and names, configured
  Forgelet-only enforcement, room collisions, exact room selection, all three
  failure stages, resume, idempotence, sibling preservation and sanitized
  results.
- Go mutation hardening killed 96/96 project-state mutants, 11/11 relay-scope
  mutants and 6/6 persisted-state mutants with zero survivors or uncovered
  sites. Acceptance mutation killed all 34 example-value mutants.
- README now documents the adapter command and lifecycle; `docs/manual.md`
  owns the detailed refusal/result/preservation contract; `ARCHITECTURE.md`
  records the project cleaner and project-scope boundary;
  `docs/manual-contract.edn` executes an example for the new manual section.
- The four pre-existing uncommitted shared `swarmforge/scripts/*.bb` changes
  remain untouched and are excluded from card commits.
