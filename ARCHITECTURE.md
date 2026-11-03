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
small Matrix boundary that forge removal needs.

Acceptance fixtures and the Gherkin runtime are under `acceptance/`; they drive
the real bridge binary and public adapter rather than reimplementing bridge
policy.

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
- Every required gate writes evidence under `build/stages/`.
- Forge removal changes only that forge's Matrix rooms and bridge-owned state;
  the bridge configuration and forge files remain unchanged.

What must stay true after the refactoring.
