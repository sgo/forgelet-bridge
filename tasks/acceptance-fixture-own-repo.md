# acceptance-fixture-own-repo

The acceptance suite wipes the lane it runs in, and it has now done so eight
times.

Each run of the bridge's acceptance suite leaves the lane it ran from reset to
the project's scaffold, with a fresh snapshot branch named
`rejected/20260922T124152671989Z-phone-approvals/<n>` behind it — eight
snapshots, one per suite run, the latest at 19:36:50 landing on the coder's lane
while it was working. The scenario that does it is the phone-approvals one where
the operator sends an approval back with feedback: that exercises the bridge's
own send-back path, which drives the forge's dashboard's Retry, and in
production that action snapshots the sender's worktree and resets it — which is
the right thing for the real dashboard to do.

What makes it land on real work is that the fixture forge root is not a
repository of its own. It is a directory tree the suite creates inside whichever
lane runs it, so every git command the dashboard runs on the fixture's behalf
resolves upward, out of the fixture and into that lane's real worktree: the
snapshot branch lands in the real repo's ref namespace and the reset moves the
lane's head. The fixture's roles give no entry for the project holding the
approvals either, so the dashboard falls back to treating the plain project
directory as the worktree it should reset.

Desired behavior: driving the forge's real dashboard against a fixture root must
be safe from any lane. No acceptance run may create branches in, or move the
head of, the worktree it runs from — a fixture's git world has to end at the
fixture. Keep exercising the production path (the bridge's send-back and the
dashboard's retry handling) rather than stepping around it, and give the suite a
way to show the isolation holds, so this cannot return unnoticed.

This is not a forge-side tooling change: nothing here should patch the base
forge, only the bridge's own acceptance harness.

Context: the phone-message-wake card is held until this lands, because a wipe
between dequeue and handoff makes its recorded base unreachable and the card
unhandoffable.
