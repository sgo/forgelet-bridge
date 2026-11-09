# Matrix backend prototype evidence

Selected feature: `features/forge-display-names.feature`
Runs per backend: 3
Recorded: `20261109T133748Z`

| Backend | Ordinary | Median wall | Median max RSS | Setup median | Bridge median | Scenario median | Teardown median | Mutation wall | Mutations |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| fresh | pass | 22.730s | 222.2 MiB | 1332 ms | 2541 ms | 1312 ms | 20 ms | 170.1s | 8/8 killed, 0 survived, 0 errors |
| shared | pass | 16.250s | 230.1 MiB | 528 ms | 1627 ms | 1312 ms | 7 ms | 156.8s | 8/8 killed, 0 survived, 0 errors |
| memory | pass | 13.830s | 221.9 MiB | 524 ms | 1322 ms | 889 ms | 8 ms | 145.1s | 8/8 killed, 0 survived, 0 errors |

Shared/fresh wall-time ratio: 1.40x. In-memory/fresh wall-time ratio: 1.64x.
Backend and mutation equivalence: pass.

Recommendation: adopt the in-memory backend as an opt-in fast lane for routine process-heavy Gherkin scenarios, keep a smaller fresh-Synapse compatibility lane for Olm/device and protocol drift, and retain shared Synapse only for scenarios that need a real server without per-scenario process startup.

## Follow-up plan

1. Adopt the memory backend as an opt-in lane for routine routing, state and threading scenarios after each feature's Matrix endpoints are covered.
2. Keep a smaller fresh-Synapse suite for Olm, device identity, encrypted compatibility, restart and protocol-drift coverage.
3. Use shared Synapse only for scenarios that require real-server semantics but cannot justify per-scenario process startup; keep per-scenario rooms and unique bridge devices.
4. Migrate one scenario group at a time in a separate measured card, with ordinary and mutation equivalence checked before the fast lane becomes the routine default.
