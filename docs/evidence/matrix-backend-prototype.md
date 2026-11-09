# Matrix backend prototype evidence

Selected feature: `features/forge-display-names.feature`
Runs per backend: 3
Recorded: `20261109T144327Z`

| Backend | Ordinary | Median wall | Median max RSS | Setup median | Bridge median | Scenario median | Teardown median | Mutation wall | Mutations |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| fresh | pass | 23.120s | 232.9 MiB | 1331 ms | 2440 ms | 1292 ms | 18 ms | 168.6s | 8/8 killed, 0 survived, 0 errors |
| shared | pass | 17.060s | 226.6 MiB | 530 ms | 1630 ms | 1310 ms | 8 ms | 154.3s | 8/8 killed, 0 survived, 0 errors |
| memory | pass | 13.750s | 225.9 MiB | 525 ms | 1322 ms | 936 ms | 7 ms | 146.2s | 8/8 killed, 0 survived, 0 errors |

Shared/fresh wall-time ratio: 1.36x. In-memory/fresh wall-time ratio: 1.68x.
Backend and mutation equivalence: pass.

Recommendation: adopt the in-memory backend as an opt-in fast lane for routine process-heavy Gherkin scenarios, keep a smaller fresh-Synapse compatibility lane for Olm/device and protocol drift, and retain shared Synapse only for scenarios that need a real server without per-scenario process startup.

## Follow-up plan

1. Adopt the memory backend as an opt-in lane for routine routing, state and threading scenarios after each feature's Matrix endpoints are covered.
2. Keep a smaller fresh-Synapse suite for Olm, device identity, encrypted compatibility, restart and protocol-drift coverage.
3. Use shared Synapse only for scenarios that require real-server semantics but cannot justify per-scenario process startup; keep per-scenario rooms and unique bridge devices.
4. Migrate one scenario group at a time in a separate measured card, with ordinary and mutation equivalence checked before the fast lane becomes the routine default.
