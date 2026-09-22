# naming-applies-on-reuse

After the naming change my phone still shows the bridge as "Bridge" rather than
"Forgelet". The name is only applied the first time a forge is ever seen: once
the bridge knows a forge's rooms, a restart reuses them without applying
anything, so the name never reaches a forge that already exists. I want a
restart to leave a forge named the way its configuration says it is, whether
the rooms are new or already there — and the same for anything else that has to
be true of a forge every time the bridge starts, not only the first time.

The acceptance suite should have caught this and did not: it proved the name on
a fresh run and proved that reuse does not duplicate rooms, but never proved the
name after a restart. A first run and a restart with existing state are the two
cases that matter, and the suite only checked one of them.
