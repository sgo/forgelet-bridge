# the-kit-tells-a-dead-agent-from-an-idle-one

# the-kit-tells-a-dead-agent-from-an-idle-one

A role's session outlives its agent, and the kit cannot see the difference. Saibill's coder sat on 2026-10-05 with
its agent gone - the pane at a shell after `Error: turn/steer failed: Server is draining` - while the forge's
checks all said nothing: the role health found the pane (there), the stall watch found no work in its lane (so
"idle with nothing assigned", which is not a stall), and the window watchdog found the window (there). The
operator found it by looking.

The kit is closer than it looks: `role_health.bb` already reads what the pane is running - it has a `pane-command`
helper over `#{pane_current_command}` and uses it for `alive?` and `forge-up?`. But it only asks whether the pane
is *there*, and a pane sitting at a shell is there.

What is wanted: the role health tells a dead agent from an idle one.

- a pane whose foreground is a shell, where a known agent should be, is "the agent has gone" - its own verdict
  beside `:session-gone`, `:idle-holding-card` and the rest, with its own words;
- it is a stall *whether or not the role holds a card*: with nothing in the lane the check today says
  "idle-nothing-assigned" and rings nothing, which is how this one went silent;
- and with a card in the lane it must not read as "idle-holding-card", which rings but sends the operator to the
  wrong question - an agent that stopped is not an agent that is waiting between turns;
- the stall watch rings this verdict as it rings the others, so a dead agent reaches the operator rather than
  waiting for someone to look.

What is already there, so this leans on it rather than inventing a second rule: the layer now carries the
decision - `dead-agent?`, a shell where a known agent should be, with the pane read twice because a tool that is
itself a shell is over in a moment - added for `--restart-role` and `--restart-dead-roles`. The kit should read
that rule (or carry the same one, with the same words) rather than growing a second opinion about what a dead
agent looks like.

Boundary, deliberately drawn: this card is the *noticing*, not the *mending*. Bringing an agent back is the
layer's `--restart-role` / `--restart-dead-roles`, and this card does not run them.

How we will know it works: with a fixture whose session is alive and whose pane is at a shell, the role health
reports the agent as gone - with a card in the lane and without one - and the stall watch rings it with those
words; with the agent at its prompt the same fixture reports it working; and a role whose session is genuinely
gone still reads as `:session-gone`, unchanged.
