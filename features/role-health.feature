# mutation-stamp: sha256=aa1171419b2a7169517aa843199999ff4a899eab0f0086679f2adfdd264a20e4
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-26T11:50:21.983479Z","feature_name":"Role Health","feature_path":"features/role-health.feature","background_hash":"1d989f071c65a110cd748176f85254577c6f592812d666af2643c2fe11b3bca4","implementation_hash":"sha256:f8a95a0020f95ba9364f81418032e9644e0f3482161e62942d65811d784b43e7","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Role Health

  # The idler check answers one question at a glance: is each role holding a card
  # working, waiting, or stalled? It is this repository's tool, and the forge's
  # own copy is a deployment of it. It reads the forge the way the forge is —
  # every role judged against the tool its own row records, a session judged
  # alive by the process in its pane rather than by the agent's name (a Claude
  # pane shows its version, not its name), and work read from the agent's own
  # record rather than from the words a terminal draws, because those words are
  # each agent's own and change between versions. A role running an agent the
  # check does not know is reported as exactly that and files nothing: a false
  # stall on a healthy session is the failure worth avoiding.
  #
  # Three waits are not stalls, and this forge has met all three. A forge that is
  # not running at all — no role session is a forge at login, not four dead
  # agents — is said once instead of filed as four alarms. A role with an open
  # clarification is waiting on a decision. And a card in a role's lane whose
  # note is somewhere — its inbox, parked by the lieutenant, still in a sender's
  # outbox or sitting in a pending approval — is work waiting to be taken up: the
  # role is idle with nothing to pick up, and calling that a stall files an alarm
  # about a role that is simply waiting its turn. A card whose note exists
  # nowhere is the other thing: the board says a role holds work and nothing
  # anywhere exists to hand it over, so it is a note that went missing, with its
  # own verdict and its own alarm — not the same name as an agent that genuinely
  # stopped mid-task.
  #
  # A request has two ways to reach a session, and a stall alert had one. Every
  # request the operator's phone sends is created by the dashboard, and the
  # dashboard wakes the master pane as it creates it, so the request waits to be
  # picked up and is already in the pane. The check built the request file itself
  # and dropped it into the same pending directory, so its alarm rested on a
  # single ring into a single pane, and one missed ring left the operator's phone
  # showing a stall no session had ever seen. A stall alert is raised the way
  # every other request is now - the dashboard takes it and wakes the pane as it
  # writes the request - and the direct write stays as the fallback for a forge
  # whose dashboard is not running, so an alarm is never lost for want of one
  # path.

  Background:
    Given the fixture forge root forge-a holds the project forgelet-bridge

  # Role Health 1: a role holding a card and quiet is a stall
  Scenario: Role Health 1: a role holding a card and quiet is a stall
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    And the project forgelet-bridge of the forge root forge-a has handed the card refund-card to the role coder
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the role coder as idle holding the card refund-card
    And the idler check names the tool codex for the role coder
    And the idler check fails

  # Role Health 2: a role with nothing assigned is not a stall
  Scenario: Role Health 2: a role with nothing assigned is not a stall
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the role coder as idle with nothing assigned
    And the idler check passes

  # Role Health 3: work is read from the agent's own record, not the pane's words
  Scenario: Role Health 3: work is read from the agent's own record, not the pane's words
    Given the project forgelet-bridge of the forge root forge-a records the role specifier running claude
    And the forge root forge-a gives the role specifier a working session whose pane shows a version rather than a name
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the role specifier as working
    And the idler check names the tool claude for the role specifier

  # Role Health 4: a role running an agent the check does not know is not dead
  Scenario: Role Health 4: a role running an agent the check does not know is not dead
    Given the project forgelet-bridge of the forge root forge-a records the role reviewer running gemini
    And the forge root forge-a gives the role reviewer a live session
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the role reviewer as running a tool it does not know
    And the idler check passes

  # Role Health 5: a forge that is not running is not four dead agents
  Scenario: Role Health 5: a forge that is not running is not four dead agents
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the forge as not running
    And the idler check passes

  # Role Health 6: a role waiting on a decision is not stalled
  Scenario: Role Health 6: a role waiting on a decision is not stalled
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the project forgelet-bridge of the forge root forge-a has an open clarification from the role coder
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the role coder as waiting on a decision
    And the idler check passes

  # Role Health 7: a card whose note is somewhere is waiting to be taken up
  Scenario: Role Health 7: a card whose note is somewhere is waiting to be taken up
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    And the project forgelet-bridge of the forge root forge-a has the note for the card refund-card still in a sender's outbox
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the role coder as idle with nothing to pick up
    And the idler check passes

  # Role Health 8: a card whose note is nowhere is a missing note, and it alarms
  Scenario: Role Health 8: a card whose note is nowhere is a missing note, and it alarms
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the note for the card refund-card as missing
    And the idler check fails

  # Role Health 9: a handoff sitting with the operator is waiting, not stalled
  Scenario: Role Health 9: a handoff sitting with the operator is waiting, not stalled
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the project forgelet-bridge of the forge root forge-a has the handoff for the card refund-card waiting for the operator
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the role coder as waiting on the operator
    And the idler check passes

  # Role Health 10: a stall alert is raised the way every other request is, and the dashboard wakes the pane
  Scenario: Role Health 10: a stall alert is raised the way every other request is, and the dashboard wakes the pane
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    And the fixture forge root forge-a has its dashboard running
    When the idler check raises the alerts for the project forgelet-bridge of the forge root forge-a
    Then the forge holds the alert the idler raised about the role coder being idle holding the card refund-card
    And the forge's dashboard typed the idler's alert into the master role's pane

  # Role Health 11: a forge with no dashboard still raises the alert
  Scenario: Role Health 11: a forge with no dashboard still raises the alert
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    When the idler check raises the alerts for the project forgelet-bridge of the forge root forge-a
    Then the forge holds the alert the idler raised about the role coder being idle holding the card refund-card
