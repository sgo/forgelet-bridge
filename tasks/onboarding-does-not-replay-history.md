# onboarding-does-not-replay-history

Bringing a forge in for the first time replays its whole card history as notifications,
which buries the phone and starves the channels it was just given.

Saibill was added tonight, the bridge created its space and all four rooms, and then it
began posting a card-activity message for every card on that forge's boards — 228 of
them, none of which is news to anyone. Each message is also a *notification*, so the
operator's phone buzzed once per card for cards that finished days or weeks ago. Their
reading, recorded because it should shape what this fixes: notifications about cards that
are new are welcome; a bulk replay of cards already finished is not.

Two separate faults, and the flood is the greater:

- **The replay.** Onboarding should not narrate history. The feed exists so the operator
  can see what is moving now; a forge joining should say what is in flight, or say
  nothing, and then report from the moment it joined. The property to hold: a forge's
  first appearance must not produce a burst proportional to its board's size.
- **What deserves a notification.** Notifications and room messages are different things
  and should be decided differently. A card arriving and a card finishing are worth a
  buzz; the routine lane-to-lane steps are worth seeing in the room and not worth waking
  anyone for. Both are achievable — routine transitions can be posted as an event type the
  clients do not notify on, with the meaningful ones as ordinary messages — and the choice
  of which is which belongs in the spec rather than in a client setting.
- **The starvation.** While those posts fail, the forge's chat, approvals and
  clarification actions sit behind them, so the channels a forge has just been given are
  the last thing to work. A failing action must not hold up the rest of that forge's
  turn — a rule this project already agreed, so it is worth checking whether the activity
  post fails as an action or takes the whole forge's turn with it.

Worth fixing at the same time: `add-forge` waits for the forge to appear in the bridge's
startup report, and that report counts a forge as reached only once its work is done. So
a successful onboarding reported failure, and the operator was told to read a log for a
problem that was only a backlog. The report should say what is true — configured, rooms
created — and let the backlog be a separate, visible thing.
