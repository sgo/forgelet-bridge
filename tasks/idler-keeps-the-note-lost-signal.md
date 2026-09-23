# idler-keeps-the-note-lost-signal

The idler check we are about to install keeps every rule except the one this forge
added last, and installing it would quietly drop that rule in two places at once.

The repository's copy already carries: each role judged against the tool its own row
records; a session alive by the process in its pane rather than by the agent's name,
because a Claude pane shows its version; work read from the agent's own record rather
than the words a terminal draws; an agent the check does not know reported as exactly
that and filing nothing; a role with an open clarification read as waiting on a
decision; and a role whose handoff is sitting with the operator read as waiting on the
operator rather than stalled.

What it does not carry is the distinction added here afterwards, and it matters
because the two cases it separates need different answers. A card in a role's lane
whose note is somewhere — its inbox, parked by the lieutenant, or still in a sender's
outbox or a pending approval — is work waiting to be taken up: the role is idle with
nothing to pick up, which is not a stall. A card whose note exists **nowhere** is the
other thing: the board says a role holds work and nothing anywhere exists to hand it,
which is a note that went missing. Today the repository's version reads the second
case as a role idle while holding a card, sharing a name with an agent that genuinely
stopped mid-task.

Desired behaviour: the note-lost distinction travels with the check — a note anywhere
counts as the card having been handed over, a note nowhere is its own verdict and its
own alarm — and the feature carries a scenario for each, so a port cannot quietly
lose it again the way this one did.
