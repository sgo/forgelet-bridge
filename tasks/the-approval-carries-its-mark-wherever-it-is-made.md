# the-approval-carries-its-mark-wherever-it-is-made

An approval resolved away from the room should carry its mark too.

What the operator sees now: approving with the check mark in the room gets the right arrow, but an
approval they ask the lieutenant to make - or make on the dashboard - gets a reply saying "Resolved on
the desktop" instead. The bridge writes its mark from the resolution it recorded, and a resolution it
did not carry out leaves it no resolution at all: only the fact that the approval stopped being
pending.

What changed underneath, so this card can be built: the dashboard now writes down how it resolved an
approval, both endings, keyed by the approval's own id, and serves the newest hundred as
resolved_approvals beside the pending ones - tagged with the project when it serves a forge root. The
words are the bridge's own: "approved" and "sent_back".

What it should do:

- when an approval the bridge posted stops being pending, and the dashboard's resolved_approvals says
  how it ended, report that ending the way the bridge reports its own: the right arrow for an approval,
  the left arrow for a send-back, on the approval's own message, with no reply for that path;
- keep the desk case as it is: an approval with no record - resolved before this change, or by
  something that writes none - still speaks in the thread ("Resolved on the desktop");
- a resolution the bridge has already reported is not reported a second time, whichever way it was
  reported - its own state already guards that;
- the marks are the ones the room already has, single-sourced as the last cards left them.

What is already there: the sending half and the single-sourced mark (the card that finished today), the
report's one home (bridge/approvals.go's reportResolution), the resolution report's own state fields
(MessageID, ReplyID, ReactionID), and the dashboard's new record. What this card adds is reading the
dashboard's answer for resolutions the bridge did not make, and choosing the mark from it.

Known: the same hole exists on the clarification side - an answer given on the desk is reported as
"Resolved on the desktop" there too - and it is not this card's; the clarification card in flight
covers only the room's own answers.
