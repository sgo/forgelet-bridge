# room-names-carry-the-forge

# room-names-carry-the-forge

From the operator's phone (2026-09-24 15:58): "In Element the rooms are often shown next to
each other outside of the context of the space in which they exist. This makes it hard to tell
the Saibill Chat from the Forgelet Chat. Perhaps we could rename the rooms to have a suffix that
contains the forge name? For example 'Chat' becomes 'Chat (Forgelet)'?"

## What is wanted

- Each of a forge's four rooms carries the forge's configured name as well as its channel:
  `Chat (Forgelet)`, `Approvals (Saibill)`, `Activity (Forgelet)`, `Clarifications (Saibill)`.
- The space keeps the name it has; it is already the forge's name and the rooms are its children.
- The suffix, not a prefix, because the channel comes first — the order already chosen for these
  rooms. A flat list then groups by channel with the forge alongside; the space view reads
  naturally too.

## What it must not break

- **Applying it to rooms that already exist.** The name a forge is configured with is already a
  fact on every start, not a one-time act of creating rooms (`naming-applies-on-reuse.feature`),
  and that property has to hold for the new names: existing rooms are renamed in place, and no
  second room per channel appears.
- **Finding a room and naming it must agree.** Provisioning from scratch looks a space's children
  up *by name* to decide whether a room already exists, then applies the name it expects. The
  name used to find a room and the name it is given therefore have to be the same, or a rename
  becomes a duplicate.
- **The fixture resolves rooms by literal name**, so this is not a one-line change and should not
  pretend to be: the acceptance steps look rooms up with the config constants
  (`roomNamed`, `chatRoom`, `waitForSpaceChild`, `approvalsRoom`), and the features name rooms by
  hand. Expect the expected name to be computed in one place from the forge and the channel, and
  the features that assert names (`naming-applies-on-reuse.feature` — "holds exactly one chat
  room Chat" — `bridge-space-provisioning.feature`, `forge-startup-report.feature`) to be
  amended rather than joined by a parallel story.

## Not in scope

- The space's own name, and the sender name on what the bridge posts (already the forge's name,
  pinned by `forge-display-names.feature`).
- Anything about which rooms exist, what lives in them, or how they are found by the bridge at
  run time — this is a naming change.

## How we will know it works

- With two forges served, the four rooms of each are distinguishable at a glance in a flat room
  list, without opening a space.
- A forge whose rooms already exist carries the new names after a restart, with exactly one room
  per channel and no duplicates, and its old messages intact.
- A forge renamed in the configuration shows the new name in its rooms on the next start.
