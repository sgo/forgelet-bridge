# doorbell-tells-busy-from-delivered

The doorbell records a request it skipped as if it had delivered it, so a message the operator sent
that never arrived can be silently written off — the failure this tool exists to repair.

## What happens, read out of the code and then confirmed on the ledger

The pass has three outcomes and records two of them the same way. In `doorbell.bb`:

    (cond
      (or (contains? @rung id) (contains? @seen id) (delivered-evidence id pane-text))
      … "was already delivered and left alone"
      (nil? pane) …
      (busy? root role idler)
      (do (swap! seen conj id) … "was not rung because the role … was busy")
      :else (do (ring! …) (swap! seen conj id) (swap! rung conj id) …))

The busy branch writes the request into `seen`, and the first branch treats `seen` as delivery. So a
request that was *looked at* while the role was mid-turn is never rung afterwards. The tool's own help
text says "or an earlier pass saw it delivered" — the code cannot tell seen-because-delivered from
seen-and-skipped.

Confirmed against Saibill's ledger rather than argued from the code: `req-20260924T151930.032900` is
recorded as seen and never rung, it appears **nowhere** in that forge's lieutenant pane — no hit on
the visible screen, none in three thousand lines of scrollback — and a pass today reported it as
"already delivered and left alone". A request that never arrived has been written off, quietly.

## What is wanted

- The ledger keeps the two apart: what has been **delivered** (evidence found, or rung) and what has
  merely been **looked at** (seen while the role was busy). A request skipped for a busy role stays
  owed, and a later pass rings it once the role is free.
- Nothing that is true today becomes false: a request already rung is never rung again, and one whose
  delivery the pane can still prove is still left alone.
- The three outcomes stay three, and the pass says which one it reached, so a quiet run is readable.

## One question folded in, deliberately

`install-kit`'s doorbell self-check runs a **live** pass, so installing the kit into a forge with
pending requests rings them. Saibill's ledger shows four rung as a side effect of today's install —
the 13:08 clarification echo, the 13:25 approval echo, the 14:09 architect clarification and the 14:48
stall alert. The rings themselves were right, since those really had never been delivered; what is
open is whether a *self-check* should have that effect. Decide it in this card, and if ringing is
intended, say so where the installer's report can be read as a deliberate repair rather than a
surprise.

## Relationship to the card already in flight

`doorbell-evidence-outlives-the-screen` changes *where delivery evidence is read* (the scrollback as
well as the screen). This card changes *what the ledger means*. They touch the same pass, so they
should not contradict each other on the way through: evidence decides delivery, the ledger remembers
only what has been proved or rung.

## How we will know it works

- A request skipped because the role was busy is rung on a later pass, once the role is free, and the
  pass says that is what it did.
- A request whose delivery the pane can still prove is left alone, as now.
- The ledger lets the next reader tell delivered from merely-seen without re-reading a pane, and no
  request is ever both seen and silently abandoned.
