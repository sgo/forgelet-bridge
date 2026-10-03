# the-approval-answers-with-a-reaction

Approving an approval should answer with a reaction on the approval, not a message under it.

What the operator sees now: they react with a checkmark on the approval message - that is already how
approving works, the message says so - and when the forge has resolved it the bridge posts a reply in
the thread. That reply is the "approved" message.

What the operator wants: no such message. The bridge reacts on the approval's own message instead, so
the approval carries the operator's checkmark and the bridge's own mark on the same line and the
thread stays quiet.

Their words: "Currently when we approve an approval request a chat message 'approved' follows. Can we
change that to an emoji on the approval request chat message itself? So the flow would be I approve by
attaching a checkmark emoji on the chat message then when the approval was successful in the dashboard
the chatbot adds another emoji of let's say a right arrow to indicate the approval was completed and
the work is now with the coder."

What it should do:

- the bridge's reaction means "the forge confirmed this and the work has moved on"; a right arrow is
  the operator's example, and the glyph is the spec's to settle;
- it goes on the approval's own message - the one the operator reacted to - and not into the thread;
- the approve path posts no message when it resolves; the send-back path keeps its threaded reply,
  because a send-back carries the operator's words and words need a message;
- a resolution the bridge has already reported is not reported twice, whichever way it is reported.

What is already there, so the spec leans on it rather than building a second path: the room already
approves by reaction (relay/approvals.go: approveReactions and approvedByReaction), the bridge already
reads reactions (matrix/connect.go captureReaction) and already keeps each approval's message id and
resolution in its state (ApprovalState.MessageID, Resolution, ReplyID), and bridge/approvals.go's
reportResolution is the one place the resolution is reported today. What is missing is the sending
half: the bridge's client sends text only (matrix/connect.go SendText), so it gains the way to send a
reaction to the message it already remembers.

Known: the approvals room is one of the four rooms every forge has, and the bridge serves all of them,
so this is the bridge's change rather than one forge's.
