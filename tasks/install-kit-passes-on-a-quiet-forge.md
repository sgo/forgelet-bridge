# install-kit-passes-on-a-quiet-forge

The kit installer fails on a forge whose queue is empty, and reports it as a failed
install.

Rehearsed here tonight: with every card done and all four roles idle, `install-kit`
installed the three tools, wrote the stall watch's agent, verified the route gate and
the watch — and then exited non-zero, saying "a self-check failed: a tool that reads
nothing is not an install". The tools were fine. The idler check ran by hand reports
the four roles correctly, the watch is alive with 515 runs behind it and a heartbeat
seconds old, and the forge's diff shows exactly the three files it should. What failed
is the self-check's expectation: it wants to find a card and mail to read, and a quiet
forge has neither.

Which means the installer cannot install itself on a forge that is between cards, and a
person following the runbook would stop at a red line that means nothing is wrong.
That is the inverse of the failure it was built to catch.

Desired behaviour: the self-check proves the tool *can read* that forge and treats an
empty read as a legitimate old one — it should name the pane it looked at, the board it
read and the inbox it read, and count "nothing in flight" as read, not as unread. The
principle it was written for still holds and should still fail an install: a tool that
reads *nothing at all* — no pane, no board, no inbox — is not an install. The fixture
case with a card and mail to read stays as it is; this is the other end of the same
check.
