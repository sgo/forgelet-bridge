# the-clarification-answers-with-a-reaction

A clarification's answer should land as a reaction on the clarification, not as a reply under it.

What the operator sees now: they reply in the clarification's thread with the answer - that is how
answering works, because the answer is their own words - and when the forge has taken it the bridge
posts a reply in the thread saying "Answered". That second reply is what should go.

What the operator wants: the treatment the approvals room just got. The bridge reacts on the
clarification's own message instead, so the question carries the bridge's mark on the same line, the
answer stays the only thing in the thread, and the room stays quiet.

Their words: "I believe something similar happens when a clarification request is answered. I'd like
to do the same there."

What it should do:

- the answer itself is unchanged: a reply in the thread - or a reply that quotes the message - is
  still how the operator answers, because words need a message;
- what changes is the report: the bridge reacts on the clarification's own message instead of replying
  "Answered", and posts no message for that path;
- a clarification the bridge finds already answered on the desk keeps its threaded message ("Resolved
  on the desktop") and takes no reaction, exactly as the approval's desk case does - so the reaction
  means "the forge confirmed what was asked and answered here";
- an answer the bridge has already reported is not reported twice, whichever way it was reported.

What is already there, so the spec leans on it rather than building a second mechanism: the report has
one home (bridge/clarifications.go's reportClarificationAnswer), the state already keeps each
clarification's message id and reply id (ClarificationState.MessageID, ReplyID), the planner already
carries the double-report guard (answersToReport, unreportedAnswer), and - as of the card that finished
today - the bridge can send a reaction at all, with its mark single-sourced beside the approval's. The
glyph is the spec's to settle; the approval's arrow is the precedent.

Known: the clarifications room is one of the four rooms every forge has, and the bridge serves all of
them, so this is the bridge's change rather than one forge's.
