# the-waiting-facts-become-state

# the-waiting-facts-become-state

The forge's rooms carry what *moves* and never what *is*. A card appearing, a card moving on, an
approval posted, a question asked - each is a message, and the room keeps the story. No room holds
the current set: which cards sit in which lanes now, which approvals are still pending, which
questions are still open. The dashboard serves that state and the console reads it there; a phone
cannot reach a dashboard.

What a phone reads today was built against events nothing writes. The bridge posts none of the
com.forgelet.* types the phone's reading looks for - it posts prose and marks as messages, and room
state only for the room's name, its members and the space's children. So a phone reads a real forge
as empty, and the two shapes it could read instead are both wrong: a custom message-like event
carries its type and no content in the SDK's own model, and the only faithful JSON there is the
timeline item's debug accessor.

The operator's decision: the waiting facts become state events. A state event is the shape for a set
that updates and clears, and it is the one read the SDK's own model serves cleanly.

## What is wanted

- The bridge writes the waiting facts as state events in their own rooms - com.forgelet.board,
  com.forgelet.approval, com.forgelet.clarification - one event per item, with the item's id as its
  state_key, carrying the same facts the dashboard serves (the shape the shared reading already
  names, so a phone and a desk cannot name the same card differently).
- An item's event is updated when its facts change and cleared with empty content when it stops
  waiting, so reading the room's state never shows a card that has moved on, an approval that is
  resolved, or a question that is answered.
- Everything already in the rooms stays. The messages - the prose, the marks, the card news - are
  untouched: the state events are a current-state view beside the story, not a replacement for it.
- Restarting posts nothing the rooms' state already has; the same facts twice is not a change.

## Boundary, deliberately drawn

- What the phone's own reading does with these events, and how a phone's decisions travel back, are
  that project's; this card only makes the bridge write the state a phone can read.
- The four rooms and their names do not change, and neither does the prose those rooms already carry.

## How we will know it works

- A forge with a board, a pending approval and an open question has a state event per item in the
  right room, keyed by the item's id, carrying the facts the dashboard serves.
- An item that stops waiting has its state event cleared, so reading the state shows the current set
  and nothing stale.
- A restart posts no state the rooms already hold.
- The acceptance runs without an AI agent and without a phone, as the mission requires: a fixture
  forge whose dashboard serves a known state, and a homeserver the run stands up, reading back the
  state the bridge wrote.
