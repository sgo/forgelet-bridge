Feature: Fetch one dashboard snapshot per bridge tick

  # The bridge needs pending approvals, resolved approvals and typed clarifications
  # from the same dashboard payload. It reads that payload once in a tick and
  # carries every consumer from the one response. The snapshot belongs to that
  # tick and that dashboard: a failed read leaves its consumers unserved until
  # a later tick, and no dashboard can stand in for another.

  Background:
    And the bridge checks once every second

  Scenario Outline: one fresh snapshot carries every waiting category
    Given the fake dashboard forge-a is available
    And the fake dashboard forge-a first serves the pending approval for the card card-a in the project forgelet-bridge
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the bridge has caught up with the forge
    When the fake dashboard forge-a next serves <state>
    And the bridge has caught up with the forge
    Then the approvals room holds <approval_messages> approval messages
    And the clarifications room holds exactly <clarification_messages> clarification messages
    And the approval for the card card-a carries <resolution>
    And the fake dashboard forge-a serves at most one state request per tick over three more ticks

    Examples:
      | state                                                                | approval_messages | clarification_messages | resolution     |
      | no waiting items                                                     | 1                  | 0                      | no bridge mark |
      | a pending approval, a resolved approval, and a pending clarification  | 2                  | 1                      | ➡              |

  Scenario: a failed snapshot fails its consumers and the next healthy tick recovers
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the bridge has caught up with the forge
    When the fake dashboard forge-a fails its state reads
    And the bridge has caught up with the forge
    Then the bridge's status names the forge forge-a as one it did not reach
    And the approvals room holds 0 approval messages
    And the clarifications room holds exactly 0 clarification messages
    And the fake dashboard forge-a serves at most one failed state request per tick over three more ticks
    When the fake dashboard forge-a next serves a pending approval, a resolved approval, and a pending clarification
    And the bridge has caught up with the forge
    Then the bridge's status names the forge forge-a as one it reached
    And the approvals room holds 1 approval messages
    And the clarifications room holds exactly 1 clarification messages
    And the fake dashboard forge-a serves at most one state request per tick over three more ticks

  Scenario Outline: two forges and two projects keep their snapshots isolated
    Given the fake dashboards forge-a and forge-b are available
    And the fake dashboard project-alpha for the forge root forge-a serves a pending clarification from the role coder asking "which alpha stage is next?"
    And the fake dashboard project-beta for the forge root forge-b serves a pending clarification from the role coder asking "which beta stage is next?"
    And the bridge is configured with the forge roots forge-a, forge-b
    And the bridge is configured with the operator @operator:example.org
    And the bridge is started
    And the bridge has caught up with the forge
    When the fake dashboards forge-a and forge-b next serve approvals in <approval-seed-a> and <approval-seed-b>
    And the fake dashboards forge-a and forge-b next serve clarifications in <clarification-seed-a> and <clarification-seed-b>
    And the bridge has caught up with the forge
    Then the forge forge-a approval room holds the card card-<approval-expected-a> and not the card card-<approval-expected-b>
    And the forge forge-b approval room holds the card card-<approval-expected-b> and not the card card-<approval-expected-a>
    And the forge forge-a clarifications room holds the project <clarification-expected-a> and not <clarification-expected-b>
    And the forge forge-b clarifications room holds the project <clarification-expected-b> and not <clarification-expected-a>
    And the fake dashboards forge-a and forge-b each serve at most one state request per tick over three more ticks

    Examples:
      | approval-seed-a | approval-expected-a | approval-seed-b | approval-expected-b | clarification-seed-a | clarification-expected-a | clarification-seed-b | clarification-expected-b |
      | alpha-room      | alpha-room          | beta-room       | beta-room           | alpha-room           | alpha-room              | beta-room            | beta-room               |
