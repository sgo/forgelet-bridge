# doorbell-evidence-outlives-the-screen

# doorbell-evidence-outlives-the-screen

The doorbell decides a request was delivered when its id appears in the master role's pane. It
reads that pane with `capture-pane -p`, which returns the **visible screen** — 104 lines on this
machine, while the same pane's scrollback holds **2,104**. Its ledger only knows what it has
watched since it was installed.

So the brief's "never ring twice for one request" holds for what the ledger has seen, and not for
what was delivered before the doorbell existed or has since scrolled away. A first pass can
re-nudge a request the role already answered — which is a small harm, but it is the exact harm the
rule was written to prevent, and it will happen on the first pass of every forge.

## Measured rather than reasoned

Of Saibill's six pending requests, checked without running a pass:

- four ids are on the visible screen → left alone, correctly;
- one (an approval echo from 16:59) is in the scrollback but not on screen → **would be rung
  again**;
- one (a stall alert from 17:19) appears nowhere → never delivered, so ringing it is the repair
  working.

## What is wanted

A first pass that does not re-ring what has already been delivered. Either read further back than
the visible screen — the scrollback is there to be read — or seed the ledger from the pane's
scrollback on the first pass, or both, with the trade named in the spec. The bound may stay: the
rule is not "remember everything forever", but "do not re-ring what the pane can still prove".

## How we will know it works

- A request delivered long ago, still pending, past the visible screen: left alone, and the pass
  names the evidence it found.
- A request that genuinely never arrived: still rung.
- The ledger says which was which, so the next reader can tell the two apart without re-reading
  the pane.
