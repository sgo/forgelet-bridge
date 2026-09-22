# approve-by-plain-text

A check mark is the only way to approve from the phone, and anything else in the
approvals room is either a send-back or silence. The operator asked for the plain
word instead: reply "approve" on the message and have it count, the way they have
said they want it on mobile.

What happened while this was being found, and why it matters: the operator sent
"approve" into the approvals room twice and nothing happened at all. That room
ignores anything that is not a check-mark reaction or a reply inside the
approval's thread, and it answers nothing, so from the phone a working relay and
a dead one look identical. Their reaction then failed on a glyph — the bridge
took one check mark and their picker sent another. The pattern is the same in
both: the room's grammar is narrower than the operator's natural gesture, and
when it cannot read a message it says nothing.

Desired behaviour: an approval can be given in plain text. A reply, or a message
in the approvals room, whose whole text is an affirmative word or the card's name
approves. A reply carrying anything else stays what it is today — a send-back
with that text as feedback — and that half must not be weakened, because a reply
is the destructive gesture that rewinds the sender's lane. The check-mark
reaction keeps working for whoever prefers it.

And when a message in that room can be read as neither, the room answers briefly
with the gestures it does take, instead of leaving the operator to guess whether
anything arrived. The scenario that asserts a plain message in the approvals room
changes nothing becomes "nothing is approved or sent back by accident, and the
room says so" — its point still stands, and it now covers the reply the operator
gets.
