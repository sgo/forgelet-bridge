# forgelet-bridge

forgelet-bridge is the transport for forgelet: the Matrix bridge that lets an
operator handle a forge's chat channel from their phone.

The first slice is the encrypted chat channel. For every configured forge root
the bridge creates a Matrix space (named after the forge) and an encrypted chat
room (`Chat`), an encrypted approvals room (`Approvals`), an encrypted activity
room (`Activity`), and an encrypted clarifications room (`Clarifications`)
inside it, invites the configured operator, and then:

- a chat request in the forge's dashboard appears as an encrypted chat message;
- the lieutenant's answer arrives as a reply in that message's thread;
- a message the operator sends becomes a chat request for the lieutenant, and
  the answer comes back in the operator's own thread;
- messages from anyone but the operator never reach the forge;
- a restart repeats nothing, in either direction.

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
`state_dir`.

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
- the Gherkin mutator keeps its per-scenario manifest in a comment block at the
  top of the feature file.

Each tool writes and updates its own manifest. Commit them with the code; do not
hand-edit them.

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
