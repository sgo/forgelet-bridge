Feature: Retire the legacy clarification path from the bridge

  # After every Family emits typed clarification rows, the automation snapshot
  # is the only clarification authority. Legacy-only, mixed, malformed and
  # unsupported projections fail closed before either notification or answer
  # routing runs. Matrix rooms and their history stay in place.

  Background:
    And the bridge checks once every second

  Scenario Outline: the projection shape decides whether clarification work runs
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a next serves a <projection> clarification projection
    And the bridge has caught up with the forge
    Then the bridge's status for the forge forge-a matches <reach>
    And the clarifications room holds exactly <notices> clarification messages
    And the bridge's last error is <error>

    Examples:
      | projection  | reach         | notices | error                               |
      | typed       | reached       | 1       | empty                               |
      | legacy      | did not reach | 0       | bounded legacy clarification error  |
      | mixed       | did not reach | 0       | bounded legacy clarification error  |
      | malformed   | did not reach | 0       | bounded malformed projection error  |
      | unsupported | did not reach | 0       | bounded unsupported projection error |

  Scenario Outline: typed lifecycle and attention decide one existing notice
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a next serves the typed clarification clarification-lifecycle in the project project-alpha with lifecycle <lifecycle> and attention <attention>
    And the bridge has caught up with the forge
    Then the clarifications room holds exactly <notices> clarification messages
    And the bridge's status names the forge forge-a as one it reached

    Examples:
      | lifecycle | attention | notices |
      | open      | unseen    | 1       |
      | open      | seen      | 0       |
      | answered  | seen      | 0       |
      | withdrawn | seen      | 0       |

  Scenario: repeated ticks and restart keep one unseen notice
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a next serves the typed clarification clarification-stable in the project project-alpha with lifecycle open and attention unseen
    And the bridge has caught up with the forge
    Then the clarifications room holds exactly 1 clarification messages
    And the fake dashboard forge-a serves at most one automation request per tick over three more ticks
    When the bridge is stopped and started again
    And the bridge has caught up with the forge
    Then the clarifications room holds exactly 1 clarification messages

  Scenario: a Matrix reply uses normal project chat and clears the typed record
    Given the fake dashboard forge-a is available
    And the fake dashboard project-alpha for the forge root forge-a serves an unresolved typed clarification clarification-answer asking "which stage should run next?"
    And the fake dashboard forge-a next serves the typed clarification clarification-answer in the project project-alpha with lifecycle open and attention unseen
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the bridge has caught up with the forge
    When the operator replies "run qa" in the typed clarification message's thread for the project project-alpha
    And the bridge has caught up with the forge
    Then the fake dashboard project-alpha records the typed answer "run qa" for clarification clarification-answer
    When the fake dashboard forge-a next serves the typed clarification clarification-answer in the project project-alpha with lifecycle answered and attention seen
    And the bridge has caught up with the forge
    Then the bridge state holds no typed clarification record for the project project-alpha

  Scenario: obsolete legacy records are reconciled without deleting room history
    Given the fake dashboard forge-a is available
    And the bridge state has an old legacy clarification mark and a typed notification mark for the project project-alpha
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a next serves the typed clarification clarification-kept in the project project-alpha with lifecycle open and attention unseen
    And the bridge has caught up with the forge
    Then the bridge state holds only the typed clarification mark for the project project-alpha
    And the clarifications room holds exactly 1 clarification messages

  Scenario: the same id remains isolated by project and Family
    Given the fake dashboards forge-a and forge-b are available
    And the bridge is configured with the forge roots forge-a and forge-b and the operator @operator:example.org
    And the bridge is started
    When the fake dashboards forge-a and forge-b next serve the same typed clarification id in different projects
    And the bridge has caught up with the forge
    Then the forge forge-a clarifications room holds the project project-alpha and not project-beta
    And the forge forge-b clarifications room holds the project project-beta and not project-alpha

  Scenario: approvals and card activity continue beside typed clarifications
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
