# the-bridge-hears-the-signal-on-either-event-it-wrote

# the-bridge-hears-the-signal-on-either-event-it-wrote

The operator pressed **Approve** on the phone. The phone's half worked - the SDK sent the encrypted check reaction,
and the bridge logged a `reaction seen` from the operator - but the forge took nothing: the handoff stayed at its
gate, no resolution was recorded, and the phone showed "Decided ... approved" while nothing had moved.

What happened, and it is proven rather than suspected: the phone reacted to the approval's **state event**
(`com.forgelet.approval`, keyed by the approval's id) - the only event it can see, because its reading is the
rooms' *state* - while the bridge listens for reactions on the **message it posted** for that approval. One item,
two events, and the decision landed on the one nobody reads. Both ids are on the record: the state event
`$1-eqe3Q9kXxrMHEwLoVVdlFEYEBPPL1EX47zk945cnE`, the bridge's message `$jXA492S7NKtDDjdQYXSOdTX8wUQ_Q5LuKz1XzqAwTZo`.

Why this belongs here rather than in the phone: the state event *is* the item - it carries the id, the card and
the gate, and it is the only thing the phone can read and act on. But Element cannot see state events, so the
operator's own path - a check reaction on the approval's *message* - has to keep working. Both must count, and
only the bridge can make that true: it writes both events for one item, and it knows both ids (the state write
hands its event id back, and the bridge does not keep it today).

What this card should do: the signal the phone can make counts, whatever event it names.

- a check reaction on the approval's own **state event** resolves the approval, exactly as a check reaction on
  the approval's message does today;
- a **reply** that names the state event - a send back, or an answer to a question - is read exactly as a reply
  that names the message is read today, and carries the operator's words the same way: these are the two choices
  that cannot be a reaction, and they have the same gap;
- both count only from the operator, and only for the item the named event is about: the state event's own id is
  remembered when it is written, so a reaction or a reply names the item it belongs to rather than being guessed
  at from the room;
- nothing about the message path changes - Element keeps working exactly as it does, because that path is the one
  the operator uses from the desk;
- and the **glyph** is settled with it, because the two sides do not agree on it today: the bridge accepts the
  check mark an Element hand sends (with the variation selector) and the one the phone sends (without it), so a
  mark that reads as a decision in the room is one the bridge acts on. If instead the mark is pinned to one form,
  that form is written down where both can read it.

How we will know it works: with a fixture forge holding one approval, a check reaction on the approval's state
event from the operator resolves it exactly as a reaction on the message does, and a reaction from anyone else
does nothing; the message path is unchanged, proved by the fixture the suite already has; a reply naming the state
event is read as the send back or the answer it is; and the same holds for a clarification's answer.
