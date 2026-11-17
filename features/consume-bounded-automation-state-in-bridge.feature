Feature: Consume bounded automation state in the bridge

  # The bridge reads one bounded automation projection per dashboard in a tick.
  # Approvals, resolutions, clarifications and board activity all come from that
  # one response. The projection is checked before any consumer uses it, and a
  # malformed projection leaves the dependent consumers unserved until a later
  # healthy tick rather than inventing state. An older dashboard may use the
  # temporary bounded full-state compatibility path during controlled rollout.

  Background:
    And the bridge checks once every second

  Scenario Outline: one projection carries every authoritative ending
    Given the fake dashboard forge-a is available
    And the fake dashboard forge-a first serves the pending approval for the card card-a in the project forgelet-bridge
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the bridge has caught up with the forge
    When the fake dashboard forge-a next serves a resolved approval for the card card-a ending as <resolution>
    And the bridge has caught up with the forge
    Then the bridge's status names the forge forge-a as one it reached
    And the approval for the card card-a carries <mark>
    And the fake dashboard forge-a serves at most one automation request per tick over three more ticks

    Examples:
      | resolution              | mark           |
      | approved                | ➡              |
      | sent_back               | ⬅              |
      | automatic_spec_approval | no bridge mark |
      | auto_approved           | no bridge mark |
      | superseded              | no bridge mark |

  Scenario: one projection carries approvals, clarifications and card activity
    Given the fake dashboard forge-a is available
    And the fake dashboard forge-a first serves the pending approval for the card card-a in the project forgelet-bridge
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the bridge has caught up with the forge
    When the fake dashboard forge-a next serves a bounded automation projection with every waiting category
    And the bridge has caught up with the forge
    Then the approvals room holds 2 approval messages
    And the clarifications room holds exactly 1 clarification messages
    And the approval for the card card-a carries ➡
    And the operator decrypts a card update saying the card card-automation appeared in the project forgelet-bridge in the lane coder
    And the fake dashboard forge-a serves at most one automation request per tick over three more ticks

  Scenario: a malformed projection fails its consumers and recovery is fresh
    Given the fake dashboard forge-a is available
    And the fake dashboard forge-a first serves the pending approval for the card card-a in the project forgelet-bridge
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the bridge has caught up with the forge
    When the fake dashboard forge-a serves a malformed automation projection
    And the bridge has caught up with the forge
    Then the bridge's status names the forge forge-a as one it did not reach
    And the approvals room holds 1 approval messages
    And the clarifications room holds exactly 0 clarification messages
    And the fake dashboard forge-a serves at most one failed automation request per tick over three more ticks
    When the fake dashboard forge-a next serves a bounded automation projection with every waiting category
    And the bridge has caught up with the forge
    Then the bridge's status names the forge forge-a as one it reached
    And the approvals room holds 2 approval messages
    And the clarifications room holds exactly 1 clarification messages

  Scenario: an unsupported automation endpoint reports bounded compatibility
    Given the fake dashboard forge-a is available
    And the fake dashboard forge-a first serves the pending approval for the card card-a in the project forgelet-bridge
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the bridge has caught up with the forge
    When the fake dashboard forge-a does not support automation state and serves bounded compatibility state with every waiting category
    And the bridge has caught up with the forge
    Then the bridge's status names the forge forge-a as one it reached
    And the bridge's status reports a projection fallback for the forge forge-a
    And the approvals room holds 2 approval messages
    And the clarifications room holds exactly 1 clarification messages
    And the approval for the card card-a carries ➡

