# one-schedule-runs-both-passes

# one-schedule-runs-both-passes

The doorbell is installed in both forges and **nothing runs it**. The machine's only cadence is the
stall watch's launchd agent, which runs the watch's pass alone. So the delivery repair exists and
never fires: a request lost while every role is healthy stays lost, which is what happened at 15:22
and 15:23 today.

## What is wanted

One schedule per machine that runs **both passes** for every configured forge root — the watch's
pass over the roles, then a doorbell pass for each root — and leaves evidence that it ran.

## Already decided, not to be reopened here

The tools stay separate. The doorbell's logic does not move into the watch: they answer different
questions (is a role holding work and not moving, versus did the operator's message arrive), their
rules differ (grace and staleness versus a single delivery rule), and this codebase splits modules
by job. And there is no second launchd agent: a watcher that dies quietly is exactly what the
watch's heartbeat exists to catch, and two agents double that surface.

## The shape

A thin runner whose only job is the cadence. It invokes the watch's pass over the roots and then a
doorbell pass for each root, and it:

- **runs both even when one fails**, so a broken pass cannot take the other down — the pattern the
  bridge already uses when one forge cannot be served;
- **leaves evidence that it ran**, the way the watch's heartbeat does today, so a schedule that
  stopped is visible rather than assumed;
- **is what the agent points at**: today `stall_watch.sh install` writes an agent running
  `stall_watch.sh run`, and `install-kit` writes the agent record beside it, so both must agree on
  what is scheduled rather than one of them quietly keeping the old target;
- **covers several roots** (this machine serves two forges today) and names a root it could not
  serve rather than skipping it in silence.

Name it for what it does rather than for either tool.

## Do not defeat the doorbell's own rules

It refuses to ring into a role that is working, which is the case that lost a request in the first
place. The schedule runs the pass; it does not force a ring, and a pass that rang nothing because
the role was busy must say so.

## How we will know it works

- With the agent installed, a request that was never delivered is rung by the machine with nobody
  asking, and the evidence shows the pass ran.
- A pass where the master role is busy rings nothing and says so.
- When one pass fails, the other has still run, and the failure is visible rather than swallowed.
