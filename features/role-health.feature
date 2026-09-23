# mutation-stamp: sha256=0c8bf16c63361536c341555a58f8fd7b6a6b1502b16a1449665811c485ac7873
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-23T17:09:45.919543Z","feature_name":"Role Health","feature_path":"features/role-health.feature","background_hash":"1d989f071c65a110cd748176f85254577c6f592812d666af2643c2fe11b3bca4","implementation_hash":"sha256:f8a95a0020f95ba9364f81418032e9644e0f3482161e62942d65811d784b43e7","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Role Health

  # The idler check answers one question at a glance: is each role holding a card
  # working, waiting, or stalled? It reads the forge the way the forge is —
  # every role judged against the tool its own row records, a session judged
  # alive by the process in its pane rather than by the agent's name (a Claude
  # pane shows its version, not its name), and work read from the agent's own
  # record rather than from the words a terminal draws, because those words are
  # each agent's own and change between versions. A role running an agent the
  # check does not know is reported as exactly that and files nothing: a false
  # stall on a healthy session is the failure worth avoiding.

  Background:
    Given the fixture forge root forge-a holds the project forgelet-bridge

  # Role Health 1: a role holding a card and quiet is a stall
  Scenario: Role Health 1: a role holding a card and quiet is a stall
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
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
    And the forge root forge-a gives the role specifier a working session
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
