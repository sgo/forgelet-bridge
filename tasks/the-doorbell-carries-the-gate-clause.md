# the-doorbell-carries-the-gate-clause

The doorbell is one of the four tools this project's kit installs into a forge,
and the installer replaces a forge's copy whenever it differs from the kit's, so
the kit's copy is what a forge ends up with on every install. The message the
doorbell rings with names the command that answers the request - and nothing else.

The forge this was found on carries one thing more: when the request is an
approval the operator forwarded, the ring says so and says the decision is the
operator's; when it is a clarification, it says the answer is the operator's to
give. The dashboard's wake already carries the same clause, and that half survives
because it lives in a file the forge owns - which is exactly why the doorbell's
half keeps being discarded: it lives in a file the kit owns, so the next install
writes the kit's version over it, without the clause, and says nothing.

So the kit should ship the clause. A ring is the moment a session that never read
its prompt - or read a stale copy, in a pane that has been up for days - meets the
request, which is the same reason the answering command rides with the message
rather than waiting in a prompt. The clause is deliberate duplication and should
be treated as the tool's own words, not as something to remove because the
lieutenant's prompt also carries the rule.

Two things belong with it. The doorbell's self-check runs a pass and looks for the
words the doorbell says when it reads a pane, so it proves the tool ran rather
than what the message says; the check should ring an approval-shaped request and a
clarification-shaped one and look for the clause, so a kit whose doorbell lost the
clause fails its own install instead of shipping quietly. And the installer writes
every `.bb` it installs with mode 0644 and every `.sh` with 0755, so a tool the
forge keeps executable loses its bit on each install - harmless while the `.sh`
wrapper is what runs, permanent noise in every update's diff, and a trap if
anything ever executes the `.bb` directly; the installer should preserve the mode
the kit's own copy carries.

Not this card, but worth recording as its own question: nothing notices when the
kit's copy of a tool falls behind the forge's until an install overwrites the
forge - the drift that caused this. Whether the answer is the bridge's completion
hook carrying the forge's current tooling into the kit, or the installer reporting
a difference rather than replacing it, is a decision for later.
