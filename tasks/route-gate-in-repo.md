# route-gate-in-repo

The route gate works in this forge and does not live here, so nothing about it can
be reviewed, covered or installed by this repository.

It is the tool that stands between a proposal and a card: the proposal is recorded,
the card is created only when the operator's own words say so, and one approval is
single-use. It runs in this forge's own scripts, in the forge root — outside this
project's tree — so the pack cannot see it in a handoff, the suite cannot run it,
and the installer that is meant to carry it has nothing reviewed to carry.

Desired behaviour: the route gate moves into this repository, with the suite
covering what `features/route-gate.feature` already states — a proposal creates no
card until the operator says so, their own words create it once, and a second
attempt on the same words is refused. Its behaviour as a tool is the contract; what
changes is where it lives and that it is covered. The forge's own copy then becomes
a deployment of this one, the way the bridge's adapter already is.

One boundary that must survive the move: the tool carries no policy. Which
proposals need asking is the forge's own lieutenant prompt — this forge asks for
every card, deliberately stricter than another forge might — so the moved tool must
behave the same in a forge whose gate is looser or tighter.
