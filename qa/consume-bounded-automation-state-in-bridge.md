# Consume bounded automation state in the bridge E2E

Run from the `forgelet-bridge` checkout:

```sh
bash qa/consume-bounded-automation-state-in-bridge.sh
```

The procedure uses temporary fake dashboards and Matrix fixtures. It does not
contact an active dashboard, change forge files or use credentials.

The script first runs the card's executable Gherkin through the real bridge. It
covers all four authoritative approval endings, a shared projection containing
approvals, resolutions, legacy and typed clarifications, card activity,
malformed projection failure and recovery, and an unsupported endpoint.

It then runs the fixed synthetic measurement over 32 ticks and two 1 MiB full
state dashboards. It verifies that both paths make one request per dashboard
per tick, that automation bytes and CPU fall, and prints the measured totals.
