# Retire the legacy clarification path from the bridge E2E

Run from the `forgelet-bridge` checkout:

```sh
bash qa/retire-legacy-clarification-path-from-bridge.sh
```

The procedure runs the focused Gherkin through the real bridge and Matrix
fixture. It proves typed-only notification and reply behavior, projection-shape
fail-closed handling, restart deduplication, project/Family isolation and
reconciliation of obsolete legacy bridge marks.

It then runs the real typed-clarification project/Family E2E, builds the bridge
binary, and invokes the read-only
`--typed-clarification-preflight` against every Family in the configured
bridge. Finally one composed `Bridge.Tick` uses the production dashboard
adapters and the current configured dashboards with a test Matrix room boundary.
All configured Families must be reached with no unhappy Family or
clarification-consumer failure.

The output contains only bounded Family names, counts and redacted E2E fields.
It never prints dashboard addresses, credentials or full question bodies.
