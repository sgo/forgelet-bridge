# one-sick-forge-does-not-quiet-the-others

One forge that is unavailable quiets every other forge the bridge serves, which
is the thing standing between us and a second forge in the space.

The bridge is being asked to carry the Saibill forge alongside this one. Adding
the entry is a config line, but the failure mode it buys is this: a tick walks
the configured forges and returns on the first one that errors — a stopped
dashboard, a stale dashboard address, an endpoint the forge's tooling does not
yet expose. So the moment a second forge is added, that forge's dashboard
becoming unreachable stops the other forge's rooms too: no chat, no approvals, no
activity, and from the phone there is no way to tell, which is the shape of the
failure that already cost us four minutes with a single forge.

Desired behaviour: forges are isolated from each other. A forge that cannot be
reached, or that refuses an action, is reported and retried on its own — the
status the bridge writes should name which forge is unhappy — while every other
forge keeps carrying its rooms. A tick that fails for one forge must not swallow
what the rooms said for the others either. The suite should show two forges where
one is broken: the broken one retried and named, the healthy one still working in
both directions.
