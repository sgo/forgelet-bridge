# verify-forgelet-bridge-single-agent-conversion

# Task

## Operator-observed problem

`forgelet-bridge` has completed its four-pack work and the operator wants it
converted to the single-agent project model. The converter has applied its
structural changes and archived the pack, but the generated pipeline is still a
Maven template and therefore does not describe this Go project.

## Desired behaviour

Finish and verify the conversion without changing product behaviour.

- Inspect `.swarmforge/archive/pack-conversion-20261103T115540`, its manifest,
  the pre-conversion runtime-refresh archive and the current dirty tree.
- Confirm the existing source, features, tasks, QA, documentation, history,
  branch and completed board rows survived.
- Keep `master`, its existing origin, and all commits through `f0b2f5e`.
- Replace every template Maven assumption in `stages.edn`, `AGENTS.md`,
  `STAGES.md`, docs and wrappers with this project's real Go pipeline.
- Preserve the bridge's existing gates: Go unit tests, property tests, the
  Gherkin acceptance runner, acceptance mutation, CRAP/DRY, function mutation,
  cleanup, documentation and deployment.
- Keep one acceptance pipeline. Do not introduce a second Java/Maven harness.
- Preserve the bridge adapter, build output layout, completion/push behavior and
  the deployed bridge service.
- Commit the conversion as its own reviewed checkpoint and push `master`.
- Leave no old pack worktree, branch, session, handoff daemon or dashboard.

Suggested real commands, to verify rather than copy blindly:
- unit: `scripts/test.sh`
- property: `scripts/property.sh`
- acceptance: `scripts/acceptance.sh`
- acceptance mutation: `scripts/acceptance-mutation.sh` over changed features
- CRAP/DRY: `crap4go` / `dry4go`
- function mutation: `mutate4go` over changed covered Go files
- architecture: `go vet` plus the actual package-boundary invariants
- deploy: `scripts/build.sh`, then the bridge adapter restart with status

Add evidence logs under `build/stages/` and make every stage prove it ran a
non-zero amount of work. A conversion-only card may use an E2E waiver when the
existing bridge acceptance/E2E procedures remain runnable and no user-facing
behaviour changes.

## Acceptance

- `get-agent-project --upgrade <project> --forge <forge> --check` is clean.
- The pack archive and manifest are present.
- Completed board cards retain names, IDs and done state.
- `stages.edn`, `AGENTS.md`, `STAGES.md`, docs and scripts describe and run the
  real Go toolchain, not Maven defaults.
- Unit, property, acceptance, CRAP/DRY, function mutation, differential Gherkin
  mutation, documentation and deploy checks pass or have an explicit approved
  conversion-only reason.
- The bridge still builds, starts from the project adapter, reaches the
  configured forges and exposes `remove-forge`.
- No pack role worktree, branch, session or daemon remains.
- The conversion checkpoint is committed, pushed and clean.

## Out of scope

- Changing bridge behaviour.
- Host or desktop forge-removal code.
- Deleting forge directories.

## Conversion evidence

The pack conversion archive is
`.swarmforge/archive/pack-conversion-20261103T115540`. Its manifest records the
standard single-agent profile and the exact archive path. Comparing its
`tasks.before.tsv` with `.swarmforge/board/tasks.tsv` retains every completed
card's order, name, `done` state, timestamps and task ID; the only added row is
this routed conversion card. The pre-conversion runtime-refresh archive holds
the tracked and untracked runtime changes, including the per-pack role copies,
so the converter's runtime overlay can be distinguished from product source.

The branch is `master`, its origin is
`https://github.com/sgo/forgelet-bridge.git`, and `f0b2f5e` remains in its
history. Existing source, features, tasks, acceptance components, QA,
documentation, commits and completed board rows are preserved. The conversion
changes only project pipeline, agent/stage documentation, executable
documentation contracts and pack-runtime cleanup.

The project language is declared as `Go` in `swarmforge/language.conf`. The
real gates use `go test` with the `goolm` tag, the existing Gherkin acceptance
runner and mutation adapter, the Go constitution tools (`crap4go`, `dry4go`,
`mutate4go`) and the existing bridge adapter. No Java/Maven build or second
acceptance pipeline is introduced.

## Quality and mutation policy

Value-driven pipeline assertions are Scenario Outlines with example tables, so
Gherkin mutation has mutable cells. Function mutation uses `mutate4go`
differentially against the generated source manifest over changed, covered Go
functions; it never uses `--mutate-all`. CRAP uses the project's real Go test
command and the ceiling of 6. DRY uses `dry4go` over the current Go tree.

## Documentation

`README.md` remains the installation, coordinates, quick-start and project
status surface with corrected Go instructions. `docs/manual.md` gains the
detailed bridge, configuration, deployment and verification behaviour.
`ARCHITECTURE.md` records the existing Go boundaries and executable
package-boundary invariants. `docs/manual-contract.edn` maps each maintained
section to its public APIs and to examples that run against this checkout.

## E2E waiver

This is a conversion-only card. It changes no bridge behaviour, and the
existing bridge acceptance suite remains the same end-to-end home-server path.
The deploy gate also rebuilds and restarts the ordinary project adapter, then
checks the live bridge's forges and `remove-forge` capability. A second
card-specific Matrix procedure would duplicate the acceptance path without
testing new user-facing behaviour.
