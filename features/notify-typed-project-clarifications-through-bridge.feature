Feature: Notify Typed Project Clarifications Through The Bridge

  # Typed project-chat clarifications use the same Clarifications room without
  # duplicating notices. The automation projection is the only clarification
  # authority; Matrix carries only its bounded question excerpt.

  Background:
    And the bridge checks once every second

  Scenario Outline: typed lifecycle and attention decide the notice
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a next serves the typed clarification clarification-shared in the project project-alpha with lifecycle <lifecycle> and attention <attention>
    And the bridge has caught up with the forge
    Then the clarifications room holds exactly <notices> clarification messages
    And the bridge's status names the forge forge-a as one it reached

    Examples:
      | lifecycle | attention | notices |
      | open      | unseen    | 1       |
      | open      | seen      | 0       |
      | answered  | seen      | 0       |
      | withdrawn | seen      | 0       |

  Scenario: an unseen typed clarification is not repeated across ticks or restart
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

  Scenario Outline: answered and withdrawn typed clarifications clear their notification record
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a next serves the typed clarification clarification-stable in the project project-alpha with lifecycle open and attention unseen
    And the bridge has caught up with the forge
    Then the clarifications room holds exactly 1 clarification messages
    When the fake dashboard forge-a next serves the typed clarification clarification-stable in the project project-alpha with lifecycle <ending> and attention seen
    And the bridge has caught up with the forge
    Then the bridge state holds no typed clarification record for the project project-alpha
    And the clarifications room holds exactly 1 clarification messages

    Examples:
      | ending    |
      | answered  |
      | withdrawn |

  Scenario: a failed projection sends no stale notice and recovery uses only fresh state
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a fails its state reads
    And the bridge has caught up with the forge
    Then the bridge's status names the forge forge-a as one it did not reach
    And the clarifications room holds exactly 0 clarification messages
    And the fake dashboard forge-a serves at most one failed automation request per tick over three more ticks
    When the fake dashboard forge-a next serves the typed clarification clarification-recovered in the project project-alpha with lifecycle open and attention unseen
    And the bridge has caught up with the forge
    Then the bridge's status names the forge forge-a as one it reached
    And the clarifications room holds exactly 1 clarification messages

  Scenario: a Matrix reply follows the owning project chat path
    Given the fake dashboard forge-a is available
    And the fake dashboard project-alpha for the forge root forge-a serves an unresolved typed clarification clarification-answer asking "which stage should run next?"
    And the fake dashboard forge-a next serves the typed clarification clarification-answer in the project project-alpha with lifecycle open and attention unseen
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the bridge has caught up with the forge
    Then the clarifications room holds exactly 1 clarification messages
    Then the clarification message for the project project-alpha names the project project-alpha, the blocked role coder, and the question "which stage should run next?"
    When the operator replies "run qa" in the typed clarification message's thread for the project project-alpha
    And the bridge has caught up with the forge
    Then the fake dashboard project-alpha records the typed answer "run qa" for clarification clarification-answer
    When the fake dashboard forge-a next serves the typed clarification clarification-answer in the project project-alpha with lifecycle answered and attention seen
    And the bridge has caught up with the forge
    Then the bridge state holds no typed clarification record for the project project-alpha
    And the clarifications room shows no unread typed clarification for the project project-alpha

  Scenario: malformed typed rows fail closed without notifications
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a serves a typed clarification with <problem>
    And the bridge has caught up with the forge
    Then the bridge's status names the forge forge-a as one it did not reach
    And the clarifications room holds exactly 0 clarification messages

    Examples:
      | problem                       |
      | invalid lifecycle             |
      | invalid attention             |
      | a missing clarification id    |
      | a blank question excerpt      |
      | duplicate ids in one project  |
      | a legacy collision            |
      | a truncated typed section     |

  Scenario: the same clarification id in two projects has two identities
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a next serves the typed clarification clarification-shared in the project project-alpha with lifecycle open and attention unseen
    And the fake dashboard forge-a also serves the typed clarification clarification-shared in the project project-beta with lifecycle open and attention unseen
    And the bridge has caught up with the forge
    Then the clarifications room holds exactly 2 clarification messages
    And the clarifications room carries typed clarifications for both project-alpha and project-beta

  Scenario: legacy and typed clarifications coexist
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a next serves a legacy clarification and a typed clarification together
    And the bridge has caught up with the forge
    Then the clarifications room holds exactly 2 clarification messages

  Scenario: typed notifications exclude text outside the bounded excerpt
    Given the fake dashboard forge-a is available
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    When the fake dashboard forge-a next serves a redacted typed clarification in the project project-alpha
    And the bridge has caught up with the forge
    Then the clarification message for the project project-alpha carries no unrelated transcript

  Scenario: two Families with the same clarification id stay isolated
    Given the fake dashboards forge-a and forge-b are available
    And the bridge is configured with the forge roots forge-a and forge-b and the operator @operator:example.org
    And the bridge is started
    When the fake dashboards forge-a and forge-b next serve the same typed clarification id in different projects
    And the bridge has caught up with the forge
    Then the forge forge-a clarifications room holds the project project-alpha and not project-beta
    And the forge forge-b clarifications room holds the project project-beta and not project-alpha
