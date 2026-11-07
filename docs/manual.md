# Operator manual

This manual describes the current public behaviour of the Go bridge. It is
maintained alongside the executable Gherkin features and the package APIs.

## Installation and quick start

The project module is `github.com/unclebob/forgelet-bridge`. Build the commands
and run the bridge from the checkout:

```sh
./scripts/build.sh
./build/acceptance/bin/forgelet-bridge --config forgelet-bridge.json
```

The build output remains under `build/acceptance/bin`. A configuration can be
checked without starting the Matrix connection by adding `--validate`.

## Bridge operation

The `cmd/forgelet-bridge` process reads one JSON configuration, connects to the
configured Matrix homeserver, and serves each configured forge. It creates or
reuses the forge space and the chat, approvals, activity and clarifications
rooms. The status file under `state_dir` reports the device, current tick,
reached forges, unavailable forges, and the work still owed to each forge.

The bridge keeps relay facts in Matrix room state so a restart does not replay
delivered messages. A temporarily unavailable forge is reported under
`unhappy_forges` while the other forges continue.

## Configuration

The `internal/config.Config` type owns the bridge configuration. Its JSON
fields are the public `homeserver_url`, `user_id`, `password` or `access_token`,
optional `device_id`, `operator`, `forges`, and optional `state_dir`.

Each `forges` entry names a forge root and the name shown to the operator. The
root is also the key for that forge's persisted rooms and projects. Credentials
and configuration values must stay in runtime state and must never be committed.

The project adapter is `scripts/matrix-bridge.sh`. It derives the forge root
from its installed location and uses the forge's
`.swarmforge/matrix-bridge.json` unless `MATRIX_BRIDGE_CONFIG` overrides it.

## Forge removal

`swarmforge/scripts/matrix-bridge.sh remove-forge <forge-root>` is the adapter
entry point for the bridge's `remove-forge` operation. The
`internal/removal.Service` validates one configured root and the saved room
ids before stopping the bridge or changing Matrix. It clears relay facts,
removes the operator, and leaves and forgets each recorded room, persisting
per-room progress so a retry can continue.

The public result is one sanitized JSON object. Its status is `removed`,
`nothing_to_remove`, `retryable`, or `refused`. The adapter leaves the bridge
configuration and the forge files unchanged, and it keeps the bridge stopped
after a removal so the host can remove the configuration entry.

## Project removal

`swarmforge/scripts/matrix-bridge.sh remove-project <forge-root>
<project-name>` retires one stopped, managed single-agent project without
removing its Family space, its forge-level rooms, any sibling project, or the
project's server-side Matrix history.

`internal/removal.Service.RemoveProject` owns the state transition. The
`ProjectCatalog` boundary resolves the configured Forgelet root and managed
project marker before `ProjectCleaner` is allowed to change Matrix.

The root must be the exact absolute root configured for a Forgelet forge, and
the project name must match the managed
`projects/<name>/.swarmforge/single-agent` marker. Relative or traversing roots,
unknown or non-Forgelet roots, unsafe or unknown project names, and incomplete
or colliding saved project-room state are refused before Matrix changes. A
caller cannot supply a Matrix room id; the room id comes only from bridge state.

For a ready project, the bridge clears that project's `com.forgelet.*` facts,
removes the operator, and leaves and forgets the bridge membership. It then
removes only the project's bridge record and project-scoped relay bookkeeping.
Each stage is saved as completed so a retry resumes at the first unfinished
stage. No administrator purge, room deletion, or event deletion is issued.

The result is one sanitized JSON object with `status` set to `removed`,
`nothing_to_remove`, `retryable`, or `refused`. Stable reasons include
`unsafe_root`, `unknown_root`, `unsafe_project`, `unknown_project`,
`incomplete_project_state`, `relay_cleanup_failed`,
`operator_removal_failed`, `bridge_membership_failed`, `state_unavailable`,
`matrix_unavailable`, and `bridge_restart_failed`.

The adapter plans before stopping a running bridge and restarts a bridge it
stopped after every apply attempt, including retryable Matrix failures. It
records a restart debt when that restart fails, and the next project-removal
invocation retries it. A second successful removal while the project directory
is still present returns `nothing_to_remove` and touches no Matrix room. The
bridge does not move the directory, change the forge's open-project list, or
edit bridge configuration.

## Project pipeline

`scripts/stage-gate.sh` is the single entry point for project gates. It selects
the project's native scripts:

- `code` runs `scripts/test.sh`.
- `clean` and `clean-final` run `scripts/check-clean.sh`, which invokes
  `crap4go` with the Go test command and `dry4go`.
- `architect` runs `scripts/check-architecture.sh`, which combines `go vet`
  with the package-boundary checks.
- `harden` runs `scripts/check-mutation.sh`, differential `mutate4go` over
  changed Go functions followed by Gherkin mutation through the one acceptance
  pipeline.
- `qa` runs `scripts/property.sh` and `scripts/acceptance.sh`.
- `docs` runs `scripts/check-documentation.sh`.
- `deploy` runs `scripts/build.sh`, restarts `scripts/matrix-bridge.sh`, and
  checks that the live process accounts for every configured forge and exposes
  `remove-forge`.

Every case writes `build/stages/<stage>.log`. No Maven reactor or second Java
acceptance harness participates in this project.

## Acceptance and verification

`scripts/acceptance.sh` parses every feature with `gherkin-parser`, generates
entry points with the Go generator, starts the fixture homeserver, and runs the
real bridge through the handlers in `acceptance/steps`. The
`acceptance/runtime.Registry` requires every Gherkin step to match exactly one
handler. Mutation uses the same runtime and acceptance entry points.

The property suite is separate from unit coverage:
`scripts/property.sh` runs tests tagged `property`. Unit tests use
`scripts/test.sh` and the `goolm` build tag. Documentation checks execute the
examples declared in `docs/manual-contract.edn`.

## Extension points

New bridge commands belong under `cmd/`, policy and durable questions belong in
`internal/`, and Matrix transport behaviour belongs only in `internal/matrix`.
New acceptance vocabulary belongs in `acceptance/steps` and must call the same
public path a user exercises. Mutation manifests are tool-owned and are
committed with the code they describe.
