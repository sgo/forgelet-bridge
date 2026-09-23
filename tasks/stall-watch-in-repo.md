# stall-watch-in-repo

The stall watch works in this forge and does not live here, so nothing about it can
be reviewed, covered or installed by this repository.

It is what notices a role that stopped while holding work when nobody is looking:
it runs the idler check on a timer, leaves a heartbeat so a silent watcher is
visible rather than assumed, and raises one chat request naming the stall, which
reaches the operator through the bridge's chat room. It runs from this forge's own
scripts with a launch agent on this machine — outside this project's tree — so the
pack cannot see it in a handoff, the suite cannot run it, and the installer that is
meant to carry it has nothing reviewed to carry.

Desired behaviour: the watch moves into this repository, with the suite covering
what `features/stall-watch.feature` already states — a stall reaches the chat room
once and a second pass files nothing new; one watch covers several forges and each
alert says which forge it came from; and a silent watcher is visible, with a stale
heartbeat reported rather than passing quietly.

Two things it must keep. It never nudges a role: it asks the operator, who decides
whether to ask the role what blocks it, send a card back, or leave it — a tool that
resumes a session with no new information is a loop waiting to happen. And the
schedule is the machine's own, so it starts at login and reports what it did rather
than depending on a process of ours to stay up; today that is a launch agent with
one forge path baked into it, which is the part that has to generalise.
