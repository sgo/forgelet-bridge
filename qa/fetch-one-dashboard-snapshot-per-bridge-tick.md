# Fetch one dashboard snapshot per bridge tick

This procedure exercises the real bridge binary and deterministic fake
dashboards:

1. Run the snapshot acceptance feature through the normal generated Gherkin
   runner and pinned Matrix fixtures.
2. Require empty and full snapshots to carry pending approvals, desk
   resolutions and clarifications as expected.
3. Require a failed snapshot to post nothing and make no same-tick retry, then
   require the next healthy tick to recover.
4. Require two forge dashboards and two project dashboards to remain isolated
   while each receives at most one state read per tick.
5. Run a fixed 32-tick synthetic measurement with two 1 MiB dashboard
   responses. The former four reads per tick become two complete snapshots.
6. Record request count, response bytes and process CPU before and after, and
   compare those numbers with the checked-in evidence.

The procedure uses loopback fixtures and needs no operator-supplied secret.
