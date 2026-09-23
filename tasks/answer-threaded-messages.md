# answer-threaded-messages

The bridge could not answer a message the operator wrote inside a thread, and it
could not recover from that on its own.

What happened on the phone tonight: the operator replied "approve" inside a
thread. A message like that already carries a relation, and a thread cannot start
from an event that has one, so the room answered
`M_UNKNOWN (HTTP 400): Cannot start threads from an event with a relation`. The
bridge retried the same reply every tick and never got past it, so the handoff
that was waiting for the operator sat unannounced in the dashboard while their
phone showed nothing at all.

A hand fix is in place and live (`8ec8e558`): the bridge now remembers the thread
an operator message was written in and anchors the answer at its root, leaving a
message that starts its own thread exactly as it was. The stuck reply's anchor
was corrected in the bridge's state by hand, because the bad value was already
persisted, and the approval it was blocking posted on the next tick.

Desired behaviour, which is the part that needs your treatment rather than my
patch: answering a message written inside a thread joins that thread, and the
suite says so — a scenario where the operator replies in a thread and the answer
arrives there, not at the top of the room and not as a failure. Treat the hand
fix as a starting point to be checked and reworked, not as settled work, and
cover the case where the operator writes in a thread from the phone the way they
actually did.
