# expose-project-removal-plan-in-bridge-adapter

# Expose project-removal planning in the bridge adapter

## Operator-observed problem

Removing `my-test-project` from the Saibill Family through the app fails at the
host's `preflight` stage with `project_plan_failed` and the visible message
`project removal plan failed`.

The failed host operation is
`op-c9896e00-3000-42a1-a162-bb80a36a17f8`. Nothing was stopped or moved. The
host invokes:

```sh
swarmforge/scripts/matrix-bridge.sh remove-project-plan <family-root> <project>
```

The installed adapter has no top-level `remove-project-plan` command, so its
dispatcher prints usage and exits nonzero. The bridge binary already implements
`--remove-project-plan`, and the combined `remove-project` command calls it
internally.

## Desired behaviour

The bridge adapter exposes:

```sh
matrix-bridge.sh remove-project-plan <forge-root> <project-name>
```

The command validates one exact, absolute configured Forgelet root and one
safe managed single-agent project before calling the bridge binary. The root
must name a configured forge containing `.swarmforge/`; the project must contain
the regular `projects/<name>/.swarmforge/single-agent` marker.

Validation failures return one sanitized JSON object and do not invoke the
bridge binary:

- blank, non-absolute, parent-traversing or extra arguments: `unsafe_root`;
- an absolute root omitted from configuration or configured but not Forgelet:
  `unknown_root`;
- blank, padded, absolute, path-shaped, room-id-shaped, `.` or `..` project:
  `unsafe_project`;
- a safe project without its managed marker: `unknown_project`;
- unreadable configuration or bridge executable: `configuration_unavailable`.

For a validated request, the adapter calls:

```sh
<bridge-binary> --config <config> \
  --remove-project-plan <forge-root> <project-name>
```

from the served forge root. A `ready` result is printed unchanged. Existing
`removed`, `nothing_to_remove`, `retryable` and `refused` result objects pass
through only after the adapter checks that they contain a permitted status and
an optional lowercase stable reason, dropping every other field. A nonzero
binary exit returns `{"status":"retryable","reason":"state_unavailable"}`.
Empty, malformed, multi-object or unknown-status output returns
`{"status":"retryable","reason":"invalid_plan"}`.

Planning is read-only. It never calls the adapter's stop/start lifecycle, never
connects to Matrix, and changes no project directory, bridge configuration,
bridge state, log, restart marker or Matrix room. It works whether the bridge
is running or stopped. The existing `remove-project` command retains its
planning, stop, apply and restart-debt lifecycle.

Deployment conformance verifies that the installed adapter implements every
host-facing command used by project removal: `remove-project-plan` and
`remove-project`.

## Acceptance

`features/expose-project-removal-plan-in-bridge-adapter.feature` is the
executable contract. Value-driven outlines cover:

- ready plans for distinct project names;
- `ready`, `removed`, `nothing_to_remove`, `retryable`, `refused`, malformed
  and nonzero binary results;
- unsafe and unconfigured roots;
- unsafe and unknown projects;
- proof that invalid requests never invoke the binary;
- unchanged bridge configuration, state, forge/project files and room
  membership, with the bridge left running.

A plain scenario asserts deployment conformance for the two host-facing adapter
commands; it is boolean and has no independently mutable example value. The
fixed `refused` bridge result is also a plain scenario because its one value is
the setup itself, leaving no independently mutable assertion cell.

## E2E procedure

The card registers `qa/e2e.edn` and ships
`qa/expose-project-removal-plan-in-bridge-adapter.{md,sh}`. Required inputs are
named, read only at run time and redacted from captured output:

`FORGELET_E2E_FAMILY_ROOT`, `FORGELET_E2E_PROJECT`,
`FORGELET_E2E_HOST_URL`, `FORGELET_E2E_HOST_TOKEN_FILE`, and
`FORGELET_E2E_SIBLING_PROJECT`.

The procedure installs the completed adapter at the host's configured
`bridge_adapter` path in the Forgelet forge root, captures the project's bridge
room and history, and submits the same authenticated project removal request
the app uses. It then verifies the host progresses past preflight, the runtime
stops, the directory is recoverable from macOS Trash, the project disappears
only after the Family refresh, and its Matrix room is inactive while
server-side history remains.

## Documentation

- `README.md` adds the planning command beside `remove-project`, its read-only
  boundary and host use.
- `docs/manual.md` owns validation order, output normalization, malformed-plan
  handling and the adapter/deployment relationship.
- `ARCHITECTURE.md` records the adapter as the sanitizing process boundary
  around the bridge planning operation.
- `docs/manual-contract.edn` gives the project-removal section an executable
  adapter usage example.
