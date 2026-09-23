# mutation-stamp: sha256=8893652c93967ce04c58260e48140e40e3e1b5f1825ea0b547547799a20cd828
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-23T19:01:01.414306Z","feature_name":"Role Health","feature_path":"features/role-health.feature","background_hash":"1d989f071c65a110cd748176f85254577c6f592812d666af2643c2fe11b3bca4","implementation_hash":"sha256:f8a95a0020f95ba9364f81418032e9644e0f3482161e62942d65811d784b43e7","scenarios":[]}
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
  # note is nowhere in its inbox has not been handed over yet: it is queued for
  # later, and calling that a stall files an alarm about a role that is simply
  # waiting its turn.

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

  # Role Health 7: a card whose note has not arrived is queued, not a stall
  Scenario: Role Health 7: a card whose note has not arrived is queued, not a stall
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    When the idler check runs for the project forgelet-bridge of the forge root forge-a
    Then the idler check reports the role coder as assigned the card refund-card but not yet handed over
    And the idler check passes
