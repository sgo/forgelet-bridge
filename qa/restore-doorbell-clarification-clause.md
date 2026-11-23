# Restore the doorbell clarification clause E2E

Run from the `forgelet-bridge` checkout:

```sh
bash qa/restore-doorbell-clarification-clause.sh
```

The procedure builds the bridge from this checkout, then runs the exact
`get-swarm-forge lieutenant` composition in a temporary directory from this
checkout. It sets `SWARMFORGE_LIEUTENANT_AGENT=claude` and points the composer at
this bridge source through `SWARMFORGE_BRIDGE_DIR`; it does not set
`SWARMFORGE_SKIP_BRIDGE`.

The run must finish with the composition success message, install the canonical
doorbell, pass the installer's clarification-clause self-check, and leave no
Family registered or active forge changed. On success the script removes only
its own verified scratch directory. On failure it retains that directory and
prints its path for diagnosis.

