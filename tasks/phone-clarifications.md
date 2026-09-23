# phone-clarifications

Clarification requests never reach the phone, so a card blocked on a question is
invisible there.

When an agent cannot continue without an answer it raises a clarification in the
forge's dashboard, and the operator can only answer it at the desktop dashboard
or in the forge chat. The phone space carries chat, approvals and activity rooms
and nothing for clarifications, and the bridge has no code for them at all —
every clarification raised so far has been answered from this machine.

Desired behaviour: a clarifications room per forge space, alongside the others,
carrying each pending clarification as a message that names the project, the role
that is blocked and the question itself. The operator answers by replying in its
thread — and unlike an approval, a reply is exactly the right gesture here,
because a clarification's answer is free text: the reply is the answer, carried
back as the answer, and it resolves through the dashboard so the blocked agent
wakes with it.

A clarification answered from the desktop should be marked resolved in the room
rather than left looking open, the way approvals already handle being decided
elsewhere. And the difference between the two rooms should be visible in what
the room says, not only in the code: in an approval's thread a reply means send
it back, in a clarification's thread a reply is the answer.
