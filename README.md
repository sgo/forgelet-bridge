# forgelet-bridge

forgelet-bridge is the transport for forgelet: the Matrix bridge that lets an
operator handle a forge's chat channel from their phone.

## Project status

`forgelet-bridge` is a single-agent Go project. Its module is
`github.com/unclebob/forgelet-bridge`, its build output is
`build/acceptance/bin`, and its gates use `scripts/acceptance.sh` for Gherkin
and the Go constitution tools for unit, property, quality and mutation checks.
The `cmd/forgelet-bridge` process is the deployed bridge. The detailed operator
contract is in [docs/manual.md](docs/manual.md), and the package boundaries and
invariants are in [ARCHITECTURE.md](ARCHITECTURE.md).

## Installation and quick start

Go 1.26 or newer is required. The pure-Go Olm implementation needs no libolm
system package.

```sh
git clone https://github.com/sgo/forgelet-bridge.git
cd forgelet-bridge
make build
./build/acceptance/bin/forgelet-bridge --config forgelet-bridge.json
```

`make build` delegates to `scripts/build.sh`.
`make test`, `make property`, `make acceptance` and
`make acceptance-mutation` run the individual gates. The stage runner in
`scripts/stage-gate.sh` writes the evidence for each gate under
`build/stages/`.

The first slice is the encrypted chat channel. For every configured forge root
the bridge creates a Matrix space (named after the forge) and an encrypted chat
room (`Chat`), an encrypted approvals room (`Approvals`), an encrypted activity
room (`Activity`), and an encrypted clarifications room (`Clarifications`)
inside it, invites the configured operator, and then:

- a chat request in the forge's dashboard appears as an encrypted chat message;
- a completed-card report explicitly sourced by the Forgelet app appears with
  a `From the Forgelet app:` prefix rather than looking like an operator
  message;
- the lieutenant's answer arrives as a reply in that message's thread;
- a message the operator sends becomes a chat request for the lieutenant, and
  the answer comes back in the operator's own thread;
- messages from anyone but the operator never reach the forge;
- a restart repeats nothing, in either direction.

The app prefix is presentation metadata, not impersonation: the event remains
under the bridge's Matrix account and keeps a normal thread root for the
Lieutenant's answer. Requests without the exact
`app.completed-card-report` source keep their ordinary message body, and the
bridge never queues a second dashboard request merely to add the prefix.

In the approvals room the operator sees what each open project in the forge is
waiting for: the card, the project, the gate (the handover roles when the
handoff reports them, otherwise the gate as reported), and the changed files.
Reacting with ✅ approves the approval exactly as the desktop does: the pending
handoff moves to the project's outbox carrying `approved: true`. Replying in its
thread sends the card back with that feedback: the feedback lands in the card's
review history, the same store the desktop's comments use, and the approval
stops being pending. The repository work the desktop also does when it retries a
card — rewinding to the task's base commit and re-seeding the lane — stays on
the desktop. Once an approval is resolved, from the phone or from the desktop,
the thread says so and nothing can approve it twice. Deleting work and tearing
projects down stay on the desktop too.

The clarifications room carries the questions a forge's agents are blocked on.
Each pending clarification arrives as a message naming the project, the role
that is blocked, and the question itself. A reply in its thread is the answer -
unlike an approval, where a reply sends the work back - and the operator's
words are carried back through the dashboard, which is what wakes the blocked
role with them and resolves the clarification; the thread then says it was
answered. A reply the phone makes by quoting the message counts as a reply as
well, and the quote the phone writes into the body is not read as the answer. A
clarification answered from the desktop is marked resolved in the room rather
than left looking open.

The activity room is a log to keep quiet: a card appearing, moving to another
lane, or finishing in one of the forge's open projects arrives as one short
update naming the project, the card, and where it moved. Nothing arrives while
nothing changes — the quiet between updates is what says an agent may be stuck —
and a restart never replays an update it already delivered.

One bridge can carry more than one forge, and the forges are kept apart. A forge
whose dashboard has stopped, whose address is stale, or whose tooling does not
expose an endpoint yet is that forge's problem: what it could not do is kept and
tried again, `status_dir/status.json` names it under `unhappy_forges` until it is
served again, and every other forge keeps carrying its rooms in both
directions. Nothing the rooms said is lost with it either: what the operator
sends to a forge that is down reaches it once it is back.

`reached_forges` in the same status names the forges the bridge is carrying, and
the bridge logs both lists once at startup. [Adding a forge](docs/adding-a-forge.md)
is the runbook for bringing another one in: what its dashboard has to serve, the
adapter command that edits the configuration
(`swarmforge/scripts/matrix-bridge.sh add-forge <root> <name>`), the checks
afterwards, the smoke test in both directions, and the rollback.

A restart also keeps the bridge on the same Matrix device: it reuses the device
and the crypto store under `state_dir`, so the operator never meets a new
unverified device, and chat that happens after a restart stays readable.
`state_dir/status.json` reports the device the bridge is using, so what the
operator's phone shows can be checked against it.

## Configuration

```json
{
  "homeserver_url": "https://matrix.example.org",
  "user_id": "@forgelet-bridge:example.org",
  "password": "…",
  "operator": "@operator:example.org",
  "forges": [
    {"root": "/srv/forges/sgo", "name": "Saibill"},
    {"root": "/srv/forges/forgelet", "name": "Forgelet"}
  ],
  "state_dir": "build/forgelet-bridge-state"
}
```

`forges` is a list even though one forge is enough for now, so a second forge
is configuration rather than a redesign. Each forge carries the name the
operator knows it by: that name is the forge's Matrix space, and the bridge
posts in the forge's chat room under that same name, so a phone can tell the
forges apart. A forge with no configured name falls back to the name of its
folder. The bridge keeps its Matrix state and its restart bookkeeping under
`state_dir`. Run the bridge with `--validate` to check a configuration without
connecting to Matrix.

Adding another forge to the same bridge is one entry in `forges` and a restart.
[docs/adding-a-forge.md](docs/adding-a-forge.md) is the runbook for it, and
`scripts/matrix-bridge.sh add-forge <root> <name>` makes the edit safely:
it validates the configuration before it writes, keeps a copy of what it
replaced, restarts the bridge, and names the new forge in what it echoes.

Removing a configured forge is:

```sh
MATRIX_BRIDGE_FORGE_ROOT="$root" "$adapter" remove-forge "<forge root>"
```

The adapter validates the root and saved room ids before stopping the bridge.
It then clears `com.forgelet.*` relay facts, removes the operator, and leaves
and forgets the bridge's membership in the forge space and every recorded room.
Only after those room operations succeed does it remove the forge and project
records from bridge state. The bridge configuration and the forge's files are
left byte-for-byte unchanged, and the bridge stays stopped for the host to
remove the configuration entry. The command prints one JSON object with status
`removed`, `nothing_to_remove`, `retryable`, or `refused`; a retryable result
keeps the room ids and completed per-room progress needed for another attempt.

Removing one stopped project from the bridge is:

```sh
MATRIX_BRIDGE_FORGE_ROOT="$root" "$adapter" \
  remove-project "<forge root>" "<project name>"
```

The host performs its read-only preflight through the same adapter:

```sh
MATRIX_BRIDGE_FORGE_ROOT="$root" "$adapter" \
  remove-project-plan "<forge root>" "<project name>"
```

`remove-project-plan` validates the exact configured Forgelet root and managed
project marker before it calls the bridge's existing planning operation. It
returns `ready` or one sanitized status object, never stops or restarts the
bridge, and changes no Matrix or bridge state. Malformed bridge output becomes
`retryable` with `invalid_plan`; a failed bridge invocation becomes `retryable`
with `state_unavailable`. The deployment check verifies that the installed
adapter still dispatches both host-facing removal commands.

The adapter plans against the configured Forgelet root and managed project
marker before it stops a running bridge. For the target project only, it clears
`com.forgelet.*` facts, removes the operator, and leaves and forgets the bridge
membership. Only after those Matrix operations succeed does it remove the
project's bridge record and project-scoped relay facts. The Family space, its
chat/approvals/activity/clarifications rooms, sibling projects and all project
files are preserved; no room purge or event deletion is requested. The adapter
restarts the bridge after the apply attempt and reports `retryable` with
`bridge_restart_failed` if a restart it owes still cannot be completed.

Run it with:

```sh
./scripts/build.sh
./build/acceptance/bin/forgelet-bridge --config forgelet-bridge.json
```

## Building

The bridge uses `mautrix-go`'s pure-Go Olm implementation, so everything builds
with the `goolm` tag and needs no libolm on the machine:

```sh
make build             # every command
make test              # unit tests (-tags goolm)
```

Plain `go build ./...` and `go test ./...` need `-tags goolm`; the scripts and
the Makefile pass it. `scripts/build.sh` writes every command to the same
`build/acceptance/bin` layout the adapter uses.

## Acceptance

The Gherkin features in `features/` are the specification. The acceptance run
needs no AI agent and no phone: it starts a pinned Synapse in a project-local
environment (`build/acceptance/synapse/venv`, pinned in
`acceptance/synapse/requirements.txt`) on a throwaway database, creates fixture
forge roots with their dashboard stubs, runs the real bridge as a separate
process, and drives a second Matrix client that stands in for the operator, so
the encrypted round trip is proven by decrypting what the bridge actually sent.

```sh
./scripts/acceptance.sh                 # every feature
./scripts/acceptance.sh features/chat-channel-relay.feature
./scripts/acceptance-mutation.sh features/chat-channel-relay.feature
```

The first run installs the pinned homeserver into the project-local environment;
later runs reuse it.

## Property tests

The invariants the bridge leans on — restart bookkeeping repeats nothing, the
dashboard request file survives a round trip, a save loads back unchanged,
example values expand, strangers never reach a forge — are property tests
written with the standard library's `testing/quick`, behind the `property`
build tag so they stay out of the unit coverage, CRAP, and mutation runs:

```sh
make property          # ./scripts/property.sh
make test && make property && make acceptance   # everything, in order
```

## Mutation manifests

Both mutation tools keep the state that makes their next run differential, and
that state travels with the code:

- the language mutation tool embeds its per-function manifest in the source file
  it mutates, between `// mutate4go-manifest-begin` and
  `// mutate4go-manifest-end`;
- `gherkin-mutator` keeps its per-scenario manifest in a comment block at the
  top of the feature file.

Each tool writes and updates its own manifest. Commit them with the code; do not
hand-edit them.

## Layout

- `cmd/forgelet-bridge` — the bridge process.
- `cmd/acceptance-entrypoint-generator` — turns parser JSON IR into generated
  acceptance entry points plus their metadata.
- `cmd/acceptance-runner` — the runner adapter the Gherkin mutator drives.
- `cmd/forge-dashboard-stub` — the fixture dashboard that wakes the lieutenant.
- `internal/config`, `internal/dashboard`, `internal/relay`, `internal/removal`,
  `internal/state` — the testable core: configuration, the dashboard queue, the
  relay decisions, forge removal, and the restart bookkeeping.
- `internal/bridge` — the orchestrator that carries the decisions out.
- `internal/matrix` — the mautrix-go adapter (spaces, encryption, relay).
- `acceptance/runtime`, `acceptance/generator`, `acceptance/steps`,
  `acceptance/fixtures` — the acceptance pipeline and its fixtures.
