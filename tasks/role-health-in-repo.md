# role-health-in-repo

The idler check works in this forge and does not live here, so nothing about it can
be reviewed, covered or installed by this repository.

It is the check that answers one question at a glance: is each role holding a card
working, waiting, or stalled? It runs in this forge's own scripts, in the forge
root — outside this project's tree — so the pack cannot see it in a handoff, the
suite cannot run it, and the installer that is meant to carry it has nothing
reviewed to carry.

Desired behaviour: the check moves into this repository, with the suite covering
what `features/role-health.feature` already states — a role holding a card and
quiet is a stall and the check fails; a role with nothing assigned is not; work is
read from the agent's own record rather than the words a terminal draws; and a role
running an agent the check does not know is reported as exactly that and files
nothing.

Two things its behaviour must keep, both learned the hard way on this forge:

- A session is alive by the process in its pane, not by the agent's name: Claude
  Code shows its version in that field (2.1.277), so matching a name would call
  every Claude session dead. Work is read from the agent's own record — Claude
  Code's transcript under `~/.claude/projects/`, codex's own pane line — because
  the words a terminal draws are each agent's wording and change between versions.
- A forge that is not running at all is not a stall: no role session is a forge at
  login, not four dead agents, and the check says so once instead of filing four
  alarms. A role with an open clarification is waiting on a decision rather than
  stopped, and should read that way too.
- A card in a role's lane whose note is nowhere in its inbox has not been handed
  over yet: it is queued for later, which is not a stall. That case is live on this
  forge — the installer card waits on the three tool cards, its note held back so
  the tools are taken first — and the check has to read it as assigned-but-not-yet,
  or it files an alarm about a role that is simply waiting its turn.
