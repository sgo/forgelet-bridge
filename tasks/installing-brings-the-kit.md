# installing-brings-the-kit

The tools that make a forge behave live only in this forge, so a forge the
bridge carries gets the channels and none of the guards.

Three of them matter here: the route gate, which asks the operator before a card
is created; the idler check, which reports at a glance whether a role holding a
card is working, waiting or stalled; and the stall watch, which notices a role
that stopped while holding work and says so on the phone. Saibill has its own,
older tooling — the bridge works there because it needs only five endpoints — but
its agents would have none of these guards, and its own operators would have no
idler check and no stall watch.

Desired behaviour: installing the bridge for a forge installs the tools with it,
the way it already installs the rules, so a forge can have the guards from its
first day rather than acquiring them later.

One distinction has to survive, and it is the whole reason this needs care: the
gate's *strictness* is not a feature, it is a policy belonging to that forge's own
lieutenant prompt. This forge requires approval for every card, deliberately
stricter than Saibill's own constitution, which asks for permission only on
fleet-wide or hard-to-reverse actions. So the installer installs the tools; it
does not decide a forge's policy, and its report should say which policy it found
and left alone.

Three things beyond copying files:

- a self-check that each installed tool reads that forge correctly — one live
  pane, one board row, one inbox — so a signal that does not fit shows up as a
  failure rather than as silence. The lesson is today's: a detector that reads
  nothing looks clean.
- the stall watch generalised to watch several forges, rather than the single
  forge path baked into today's launch agent.
- a report in the same voice as the rules installer: what it installed, what was
  already current, what it left alone.

The tools themselves have to move into this repository first, with the suite
covering them, the way the adapter did. Until they do, there is nothing an
installer could install that anyone has reviewed.

One more requirement, from the operator: the forges are not uniform about which
agent runs a role. Saibill's `roles.tsv` records a tool per role and several rows
say `claude` — saibill-sgo's specifier, saibill-spi's reviewer, saibill-neutral's
reviewer — while the rest say `codex`. So the tools must read the role's own tool
rather than assume one: whether a session is alive is judged against the tool the
role records, and the signal for "working" is per tool, because the line a
terminal shows while an agent is generating is that agent's own wording. A role
running an agent the tool does not know yet must be reported as exactly that,
never read as dead. The failure that matters here is a false stall on a healthy
session — so the self-check should print the command and the marker it looked
for, and a mismatch should be a fact on the page rather than silence.
