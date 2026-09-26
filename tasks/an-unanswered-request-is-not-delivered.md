# an-unanswered-request-is-not-delivered

A stall alert reached the operator's phone on the morning of 2026-09-26 and never
reached their lieutenant. The idler raised it at 08:52 because a coder had been idle
holding a card; the doorbell rang it into the master pane; no turn followed, so the
session never acted on it, and the request stayed pending for eighty minutes while
every pass looked at it and left it alone. The card it was about had finished a minute
after the alert was written, so nothing was actually stuck - what failed was the one
path that carries an alert to the session that has to do something about it.

Two things are wrong, and they are different in kind.

The doorbell treats delivery as done. It rings a request the pane does not prove, and
it accepts three kinds of proof - the pane's screen, its scrollback, or the ledger it
writes for itself - after which it prints that the request "was already delivered from
<whichever>" and leaves it alone. That is right for a request that has been *answered*
and wrong for one that is still awaiting an answer: the pane's text proves that words
arrived, not that a session read them, and the ledger's memory is forever. In this
forge the log holds thousands of lines saying exactly that sentence. So a request that
is delivered and unanswered is never rung again, and the operator's phone shows a
question the lieutenant has never seen.

The request should be rung again while it is unanswered. The pass already holds both
facts it needs - the request is pending in the dashboard's store, and the ledger says
it was delivered - and they are simply never compared. Ringing it again wants a cap
and a gap rather than a bare loop: a session that is genuinely away should not be
battered, and a request that has been rung its fill should be reported as still
unanswered rather than rung forever. Say which attempt a ring is, so a pane that has
seen the same alert three times knows it is not new.

The second fault is that an alert has only one way to arrive. Every request the
operator's phone sends is created by the dashboard, and the dashboard wakes the master
pane as it creates it, so that request has two chances to be seen: the wake, and the
doorbell's ring. The idler does not do that - it builds the request file itself and
drops it into the dashboard's pending directory - so a stall alert has exactly one
chance, and everything rests on a single ring into a single pane. Raising the alert
the way every other request is raised gives it the same two chances, and the reason to
prefer the direct file write (a dashboard that is not running cannot accept a request)
can be kept as a fallback rather than as the only path.

The two fixes are one card because they answer one failure, but they should be read as
the two halves they are: the re-ring is the net, which catches every request whose
first delivery was missed, the operator's messages included; and the idler's path is
the hole, which is that an alert no session has ever acted on should not depend on one
ring to be seen at all.
