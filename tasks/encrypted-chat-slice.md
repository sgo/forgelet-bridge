# encrypted-chat-slice

Today the forge is only reachable from the desktop dashboard. I want the
first working piece of forgelet-bridge: from my phone, in an encrypted Matrix
room the bridge creates and keeps for this forge, I can see the forge's chat
channel and reply to it — my reply reaches the lieutenant as a chat request,
and the answer comes back as a reply in that thread. Nobody else's messages
may reach the forge, and restarting the bridge must not duplicate or replay
anything. The bridge creates its own space and room; I do not hand-create
them in Element, and encryption has to be on from creation, since that is
what keeps the notification payload opaque.

Scope for this card: the chat channel only, one forge configured. Approvals,
clarifications, and anything that changes forge state come later.
Configuration should accept a list of forge roots even though only one is
used now, so adding the second forge later is not a redesign.

Acceptance must run without AI agents and without my phone, as the mission
requires: a fixture forge root with the dashboard's tmux stub, and a homeserver
the tests start themselves — a pinned Synapse in a project-local environment
with a throwaway database — plus a second client standing in for me, so the
encrypted round-trip is proven by decrypting what the bridge actually sent.
Me seeing it arrive on my phone is a manual check afterwards.
