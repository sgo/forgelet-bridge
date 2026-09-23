# mutation-stamp: sha256=9c817a1bf597fe32990055b4820b13cd66c53d322d629e40c9da1d011f8fac78
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-23T17:09:46.647462Z","feature_name":"Stall Watch","feature_path":"features/stall-watch.feature","background_hash":"6eb52f5a96020b6037784710db5477e6f665699690ac0b411b96946309e323d6","implementation_hash":"sha256:f2071cca306c3a858f05f7f2e93a7fda738c1fb248ea6bfd5bfb4387aa3a8714","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Stall Watch

  # The watch is what notices a role that stopped while holding work when nobody
  # is looking. It is this repository's tool, and the forge's own copy is a
  # deployment of it. It runs the idler check on a timer, leaves a heartbeat so a
  # silent watcher is visible rather than assumed, and raises one chat request
  # naming the stall, which reaches the operator through the bridge's chat room
  # the way any forge question does. It never nudges the role: it asks the
  # operator, who decides whether to ask the role what blocks it, send a card
  # back, or leave it. The schedule is the machine's own, so the agent it writes
  # starts at login and runs the watch for every forge it was installed for, and
  # an alert says which forge it came from.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org

  # Stall Watch 1: a stall reaches the chat room once
  Scenario: Stall Watch 1: a stall reaches the chat room once
    Given the fixture forge root forge-a holds the project forgelet-bridge
    And the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    And the project forgelet-bridge of the forge root forge-a has handed the card refund-card to the role coder
    And the bridge is started
    When the stall watch runs for the forge root forge-a
    Then the forge holds a chat request naming the role coder and the card refund-card
    And the operator decrypts a chat message naming the role coder and the card refund-card
    And another pass of the stall watch files nothing new about the card refund-card

  # Stall Watch 2: one watch covers several forges, and each alert says which one
  Scenario: Stall Watch 2: one watch covers several forges, and each alert says which one
    Given the fixture forge roots forge-a and forge-b hold the project forgelet-bridge
    And the watch's agent was installed for the forge roots forge-a, forge-b
    And the project forgelet-bridge of the forge root forge-b records the role reviewer running claude
    And the forge root forge-b gives the role reviewer a live session
    And the board of the forge root forge-b holds the card export-card in the project forgelet-bridge in the lane reviewer
    And the project forgelet-bridge of the forge root forge-b has handed the card export-card to the role reviewer
    When the stall watch runs for the forge roots forge-a, forge-b
    Then the watch's agent runs the watch for the forge roots forge-a, forge-b
    And the forge forge-b holds a chat request naming the role reviewer and the forge it came from

  # Stall Watch 3: a silent watcher is visible
  Scenario: Stall Watch 3: a silent watcher is visible
    Given the fixture forge root forge-a holds the project forgelet-bridge
    When the stall watch runs for the forge root forge-a
    Then the watch reports it is alive
    When the watch's heartbeat goes stale
    Then the watch reports it has gone quiet

  # Stall Watch 4: the watch asks the operator and never the role
  Scenario: Stall Watch 4: the watch asks the operator and never the role
    Given the fixture forge root forge-a holds the project forgelet-bridge
    And the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    And the project forgelet-bridge of the forge root forge-a has handed the card refund-card to the role coder
    And the bridge is started
    When the stall watch runs for the forge root forge-a
    Then the forge holds a chat request naming the role coder and the card refund-card
    And the watch left the role coder's pane alone
