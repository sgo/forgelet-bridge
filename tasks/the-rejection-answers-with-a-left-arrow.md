# the-rejection-answers-with-a-left-arrow

A send-back should land as a left arrow on the approval, the way an approval lands as a right arrow.

What the operator wants, in their words: "Now sometimes we'll do a rejection on an approval request. In
that case I'd like to see a left arrow."

What happens now: when the operator replies in an approval's thread to send the work back, the bridge
posts "Sent back with feedback" as a reply. The approvals room had that reply removed for the approving
case just now - the bridge reacts with a right arrow on the approval's own message instead - and this is
the other half: a send-back carries a left arrow on the same message.

Where the words go, which is what makes this safe: the operator's feedback is their own reply, which
they send and which the bridge carries to the forge; the bridge's "Sent back with feedback" is a fixed
sentence with no words in it. So the mark replaces a sentence that carried nothing but the outcome.

What it should do:

- a send-back the operator gives in the room - a reply in the thread, or a reply that quotes the
  message - is reported by reacting ⬅ on the approval's own message, with no reply from the bridge,
  exactly as an approval is reported with ➡;
- the operator's own reply stays, because it is their words and it is what went to the forge;
- a resolution the bridge finds already made on the desk keeps its threaded message ("Resolved on the
  desktop") and takes no reaction;
- a resolution the bridge has already reported is not reported twice, whichever mark or message
  reported it.

What is already there, so the spec leans on it rather than building anything: the bridge can send a
reaction and single-sources its mark (the card that finished today), the report has one home
(bridge/approvals.go's reportResolution), the state keeps each approval's message id and resolution,
and the planner carries the double-report guard. What changes is the pair of marks, not the machinery.

Known: the room's own sentence tells the operator that a reply sends the work back; the marks are the
bridge's answer, and the two must not contradict each other.
