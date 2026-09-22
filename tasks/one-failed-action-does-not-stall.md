# one-failed-action-does-not-stall

One action the bridge cannot carry out stops everything the bridge was going to
do, and can also lose what the operator sent while it is stuck.

What happened tonight: a single reply the room refused failed the whole tick.
Every later tick failed the same way, so nothing else in the bridge ran either —
the pending approval from the specifier never reached the approvals room while
the dashboard showed it waiting, the card activity feed went quiet, and the
phone had no way to tell the difference between "nothing is happening" and "the
bridge is stuck on one message". It stayed that way for about four minutes until
the bridge was restarted with a fix for the offending message.

There is a second hazard in the same shape, which the code shows rather than
tonight's log: the bridge takes what the rooms have said before it does the work
of a tick, so anything it took that way is dropped if the work then fails. No
message was lost tonight because nothing arrived during those four minutes, but
an operator writing while the bridge is stuck would have their words swallowed
without a trace.

Desired behaviour: one action that cannot be carried out is reported and
retried, and the rest of the tick goes ahead — approvals, activity and the other
messages are not held hostage by it. What a tick took from the rooms is not lost
when the work it planned then fails, and a stuck bridge is visible from the
phone or the dashboard rather than looking like a quiet one. The suite should
show all of that: a failing action with healthy work behind it, and a message
that arrives while a failure is in flight and still lands.
