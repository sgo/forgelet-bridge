# adding-a-forge

Adding a forge to the Matrix space is a config edit, a restart and a handful of
checks — and today it lives in one person's memory, which is not good enough for
something the operator will want a person or another agent to do without them.

The operator's ask: the steps to bring the Saibill forge (`/Users/sgo/sgo`) into
the space, written down so somebody else can follow them, with what to check and
what to do when a check fails.

Desired behaviour: a runbook in the bridge's own repository covering the whole
path — what the target forge must have (a running dashboard with a current
`.swarmforge/dashboard-url`, the endpoints the bridge calls, and what to do when
it has older tooling), the config entry (root and the name the operator knows the
forge by), the restart, the checks that the space and its rooms were provisioned
and the operator invited, the smoke test in both directions (a phone message
wakes the right forge's lieutenant, an approval decided from the phone, a
clarification answered), and the rollback.

The runbook should be executable rather than prose-only. An adapter command that
makes the edit safely — something like `matrix-bridge.sh add-forge <root>
<name>`, validating the json, keeping a copy of what it replaced, restarting, and
echoing the log line that names the new forge — turns "follow these steps" into
"run this and read that". And the bridge should say on startup which configured
forges it reached and which it did not, so the verification is a command instead
of a guess about a log line that was never written.

This is the bridge's repository and the forge's own adapter script; it is not a
change to the base forge tooling.
