# Architecture

The bridge keeps transport, orchestration, domain decisions and fixture
acceptance separate. The Go package graph is the executable statement of that
separation.

## Decision

`cmd/forgelet-bridge` is the process boundary. `internal/bridge` coordinates
one relay pass. `internal/config`, `internal/dashboard`, `internal/relay`,
`internal/removal`, `internal/state` and the other policy packages answer
questions without depending on Matrix. `internal/matrix` is the only package
that adapts the `mautrix-go` client, while `internal/removal.Cleaner` is the
small Matrix boundary that forge removal needs and
`internal/removal.ProjectCleaner` is the narrower boundary for one project.
Project identity is resolved through `ProjectCatalog`; Matrix commands never
accept a project room id from their caller.

The installed `scripts/matrix-bridge.sh` adapter is the process boundary for
host project removal. Its `remove-project-plan` operation validates configured
identity before crossing into the bridge binary and reduces the binary's result
to the host's bounded status/result contract. The combined `remove-project`
operation keeps its stop, apply and restart-debt lifecycle around that planning
step. `scripts/check-adapter-conformance.sh` checks both host-facing commands.
Planning does not call the stop/start lifecycle or the Matrix boundary.

Acceptance fixtures and the Gherkin runtime are under `acceptance/`; they drive
the real bridge binary and public adapter rather than reimplementing bridge
policy.

`acceptance/fixtures.Homeserver` is the test-side Matrix boundary. Real Synapse
(`fresh` and process-scoped `shared`) and `MemoryHomeserver` implement the same
registration and base-URL contract. The in-memory implementation is not a
production transport and is never imported by `cmd/` or `internal/`; it relays
client ciphertext and supplies Matrix Client API endpoints needed by fixture
clients.

## Why

The bridge must keep carrying other forges when one homeserver or forge is
unavailable. Keeping relay, removal and restart decisions independent of the
Matrix client makes those decisions testable and keeps transport failures at
the edge. It also leaves one acceptance pipeline: the Gherkin runner exercises
the same process and adapter the operator deploys, and `scripts/acceptance.sh`
starts that runner from the current checkout.

The conversion to a single agent removes role worktrees, handoffs and pack
session machinery. It does not justify a new build shape, so the existing Go
commands and `build/acceptance/bin` output remain the contract.

## Invariants

- `internal/matrix` is the only `internal` package that imports a Matrix
  client.
- Core policy and state packages do not import `internal/bridge` or
  `internal/matrix`.
- `scripts/check-architecture.sh` executes `go vet` and rejects either
  dependency direction.
- The Gherkin runtime dispatches every step to exactly one handler and runs
  through `scripts/acceptance.sh`.
- The acceptance Matrix backend defaults to fresh Synapse; shared and memory
  are opt-in test fixtures and do not alter production bridge configuration.
- Every required gate writes evidence under `build/stages/`.
- Dashboard request source metadata is carried through `internal/dashboard` and
  `internal/relay`; only the exact `app.completed-card-report` value changes
  presentation, and Matrix still carries the event as the bridge account.
- Forge removal changes only that forge's Matrix rooms and bridge-owned state;
  the bridge configuration and forge files remain unchanged.
- Project removal uses the saved room for one exact managed project only. Its
  room, activity board fact and approval facts are cleared independently of
  forge-level and sibling project work.
- Relay entries written for a project carry project scope in addition to forge
  scope, so project removal cannot erase a sibling project in the same forge.
- A project with saved removal progress is not relayed while that removal is
  incomplete.
- Project-plan validation happens in the adapter before binary execution, and
  planning never changes adapter lifecycle, bridge state or Matrix membership.
- Malformed or nonconforming bridge plan output is sanitized at the adapter
  boundary rather than exposed to the host.

What must stay true after the refactoring.
