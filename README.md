# forgelet-bridge

forgelet-bridge is the transport for forgelet: the Matrix bridge that lets an
operator handle a forge's chat channel from their phone.

The first slice is the encrypted chat channel. For every configured forge root
the bridge creates a Matrix space (named after the forge) and an encrypted chat
room (`Chat`) inside it, invites the configured operator, and then:

- a chat request in the forge's dashboard appears as an encrypted chat message;
- the lieutenant's answer arrives as a reply in that message's thread;
- a message the operator sends becomes a chat request for the lieutenant, and
  the answer comes back in the operator's own thread;
- messages from anyone but the operator never reach the forge;
- a restart repeats nothing, in either direction.

## Configuration

```json
{
  "homeserver_url": "https://matrix.example.org",
  "user_id": "@forgelet-bridge:example.org",
  "password": "…",
  "operator": "@operator:example.org",
  "forge_roots": ["/srv/forges/forge-a"],
  "state_dir": "build/forgelet-bridge-state"
}
```

`forge_roots` is a list even though one forge is enough for now, so a second
forge is configuration rather than a redesign. The bridge keeps its Matrix
state and its restart bookkeeping under `state_dir`.

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
the Makefile pass it.

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

## Layout

- `cmd/forgelet-bridge` — the bridge process.
- `cmd/acceptance-entrypoint-generator` — turns parser JSON IR into generated
  acceptance entry points plus their metadata.
- `cmd/acceptance-runner` — the runner adapter the Gherkin mutator drives.
- `cmd/forge-dashboard-stub` — the fixture dashboard that wakes the lieutenant.
- `internal/config`, `internal/dashboard`, `internal/relay`, `internal/state` —
  the testable core: configuration, the dashboard queue, the relay decisions,
  and the restart bookkeeping.
- `internal/bridge` — the orchestrator that carries the decisions out.
- `internal/matrix` — the mautrix-go adapter (spaces, encryption, relay).
- `acceptance/runtime`, `acceptance/generator`, `acceptance/steps`,
  `acceptance/fixtures` — the acceptance pipeline and its fixtures.
