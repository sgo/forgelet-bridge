# doorbell-redelivers-a-lost-request

# doorbell-redelivers-a-lost-request

The operator's own message to a forge can be written down and never delivered, with nothing
anywhere saying so. Observed in Saibill on 2026-09-24 at 15:22, and it is why this card exists.

## What happened, as evidence rather than as impression

- The bridge posts the operator's chat message to the forge's dashboard (`POST /api/chat`). The
  dashboard writes `.swarmforge/dashboard/requests/pending/<id>.request`, types
  `[<id>] <text>` into the master role's pane, and answers 200.
- Three requests in Saibill's store sat pending with **no trace in that pane**, while their
  neighbours — sent minutes before and after — were delivered and answered. Two of the three
  were the operator's phone messages at 15:22:35 and 15:23:21. The pane was **idle** at the
  time, so "the agent was busy" does not explain it.
- Re-typing those two by hand delivered them immediately, and the lieutenant answered both.
- Nothing reported the failure: the dashboard prints an injection error to stderr, which its
  log does not keep; the bridge sees HTTP 200 and counts the work done; and the stall watch
  examines only roles *holding cards*, so a request that never arrived is invisible to all
  three. A silence that had to be guessed at is the exact thing this project exists to remove.

## What is wanted

- A request that is pending with no evidence it was ever delivered is **rung again**, and what
  the doorbell did is written down where the operator and the next reader can see it. A repair
  that is visible, not a retry that is hoped for.
- It is **our tooling in forgelet's kit**, alongside the route gate, the idler check and the
  stall watch, installed into a forge's own scripts the same way. Not a change to base
  swarmforge, which has deliberately been left alone.

## Things it must get right

- **Never ring twice for one request.** A request that has been delivered and simply not yet
  answered must be left alone; a pass should leave evidence of what it saw and did.
- **An idle role is a prerequisite.** Injecting into a role mid-turn is exactly the case that
  failed, so it waits for a window and says so when it cannot find one.
- **Say what it found.** For each pending request: delivered already, never delivered and rung,
  or not rung because the role was busy — the three verdicts, so a quiet run is readable.

## Grounding that may help, not a design

The pane's own text is the only delivery evidence we have: the id is typed in as `[<id>]`, and
`capture-pane` can be searched for it (that is how the loss above was established). Captures
are bounded, so a ledger of what has already been seen — and of what has already been rung —
is what stops a delivered request from being rung again.

## How we will know it works

- A request pending with no delivery evidence is rung once, said so, and gets answered.
- A request pending that was already delivered is left alone, and the pass says why.
- With the role mid-turn, it waits rather than injecting, and says that too.
