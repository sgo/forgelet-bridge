# batch-card-news-per-tick

# batch-card-news-per-tick

The activity feed posts **one message per card event**. A tick that moves several cards posts
several messages, and on the phone each one is a notification. This card makes a tick's card
news travel as one message, so the volume is bounded by ticks rather than by the size of a
board.

## Why it is wanted now, as evidence rather than as taste

- When Saibill joined the bridge, its **228-card board drained one activity message per card**
  against a homeserver limit of one a second: minutes of catch-up, a `M_LIMIT_EXCEEDED` for
  card after card, and every one of those reported as "the forge could not be served" — honest
  but noisy, and witnessed by the operator.
- The replay fix that landed today (`onboarding-does-not-replay-history`) removed the worst
  case: a forge's arrival no longer narrates its history. What remains is steady state — a
  busy tick, or a catch-up after a restart — which still posts per card.
- The hosted homeserver will keep its rate limits at or near the defaults instead of raised as
  this laptop's are, which makes posting less **the bridge's job** rather than the server's.
  That is the decision this card follows from, not a preference.
- Fewer messages also means fewer notifications, which the operator has already asked for once:
  a "card finished" alert for every card in a large forge is not pleasant.

## What is wanted

- A tick's card news is posted as **one message**: every card appearance and finish that
  happened in that tick, named in the message for that tick. Nothing is dropped, and nothing
  arrives later than it does today — a tick is short.
- A tick with one card's news reads as it does today. The change is about several.
- Restarting must not re-post news the rooms already have, exactly as now.

## Boundary, deliberately drawn

- The related lever — reading a throttle as **backpressure** (honouring `retry_after`) instead
  of as a forge that cannot be served — stays out of this card. It is recorded separately and
  is a different behaviour.
- The rule that a routine lane-to-lane move is seen but does not notify does not change.
- This card amends `card-activity-feed.feature` rather than adding a parallel story to it: that
  feature currently asserts the room holds exactly one update per event (three for an
  appearance, a move and a finish), which is the expectation batching changes.

## How we will know it works

- A tick in which several cards appear or finish posts one message naming all of them, and the
  room holds one update for that tick rather than one per card.
- A tick with a single card's news posts one message, as today.
- Across a restart, the same news is never posted twice.
- A burst of card changes no longer produces a message per card, and so no longer produces a
  rate-limit refusal per card.
