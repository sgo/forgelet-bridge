Feature: Phone Clarifications

  # A typed project-chat clarification is carried into the Family's
  # Clarifications room with only its project, role and bounded question
  # excerpt. An operator reply is carried back through the owning project's
  # normal chat endpoint, which wakes that project and marks the row answered.
  # The bridge never consults the legacy clarification store.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the forge space forge-a holds the clarifications room Clarifications
    And the operator is invited to the clarifications room Clarifications
    And the clarifications room Clarifications is encrypted

  Scenario: Phone Clarifications 1: a typed clarification reaches the phone
    Given the forge's dashboard already holds the pending clarification from the role project-agent in the project forgelet-bridge asking "which lane should the refund card start in?"
    Then the clarification message for the project forgelet-bridge names the project forgelet-bridge, the blocked role project-agent, and the question "which lane should the refund card start in?"
    And the clarifications room holds exactly 1 clarification messages
    And the approvals room holds no messages

  Scenario: Phone Clarifications 2: a reply answers through project chat
    Given the forge's dashboard already holds the pending clarification from the role project-agent in the project forgelet-bridge asking "should the invoice card retry on its own?"
    And the bridge has caught up with the forge
    When the operator replies "yes" in the clarification message's thread for the project forgelet-bridge
    Then the forge's dashboard recorded the clarification for the project forgelet-bridge as answered with exactly "yes"
    And the blocked role project-agent is woken with the answer "yes"
    And the clarification message for the project forgelet-bridge carries the bridge's ➡ reaction
    And the clarification message's thread holds no reply from the bridge

  Scenario: Phone Clarifications 3: a desktop answer produces no new Matrix work
    Given the forge's dashboard already holds the pending clarification from the role project-agent in the project forgelet-bridge asking "may I delete the stale branch?"
    And the bridge has caught up with the forge
    When the operator answers the clarification for the project forgelet-bridge from the desktop with "keep it manual"
    And the bridge has caught up with the forge
    Then the bridge state holds no typed clarification record for the project forgelet-bridge
    And the clarifications room holds exactly 1 clarification messages
    And the clarification message for the project forgelet-bridge carries no reaction from the bridge

  Scenario: Phone Clarifications 4: each room says what a reply means
    Given the forge's dashboard already holds the pending clarification from the role project-agent in the project forgelet-bridge asking "should the retry wait for the nightly build?"
    And the forge's dashboard already holds the pending approval for the card phone-clarifications with its handover roles
    And the bridge has caught up with the forge
    Then the clarification message for the project forgelet-bridge tells the operator a reply is the answer
    And the approval message for the card phone-clarifications tells the operator a reply sends it back

  Scenario: Phone Clarifications 5: only the operator's reply answers
    Given the forge's dashboard already holds the pending clarification from the role project-agent in the project forgelet-bridge asking "is the invoice card urgent?"
    And the bridge has caught up with the forge
    And the matrix client @stranger:example.org has joined the clarifications room
    When the matrix client @stranger:example.org replies "yes, very" in the clarification message's thread for the project forgelet-bridge
    Then the clarification for the project forgelet-bridge is still pending in the forge

  Scenario: Phone Clarifications 6: a plain room message answers nothing
    Given the forge's dashboard already holds the pending clarification from the role project-agent in the project forgelet-bridge asking "can I skip the fixture snapshot?"
    And the bridge has caught up with the forge
    When the operator sends the message "yes, skip it" into the clarifications room
    Then the clarification for the project forgelet-bridge is still pending in the forge
    And the forge holds 0 chat requests

  Scenario: Phone Clarifications 7: a quoted reply is the answer
    Given the forge's dashboard already holds the pending clarification from the role project-agent in the project forgelet-bridge asking "should the invoice card retry on its own?"
    And the bridge has caught up with the forge
    When the operator swipes a reply "yes" to the clarification message for the project forgelet-bridge
    Then the forge's dashboard recorded the clarification for the project forgelet-bridge as answered with exactly "yes"
    And the blocked role project-agent is woken with the answer "yes"
    And the clarification message for the project forgelet-bridge carries the bridge's ➡ reaction
    And the clarification message's thread holds no reply from the bridge

  Scenario: Phone Clarifications 8: a state-event reply is the answer
    Given the forge's dashboard already holds the pending clarification from the role project-agent in the project forgelet-bridge asking "should the invoice card retry on its own?"
    And the bridge has caught up with the forge
    When the operator replies "yes" to the clarification's state event for the project forgelet-bridge
    Then the forge's dashboard recorded the clarification for the project forgelet-bridge as answered with exactly "yes"
    And the blocked role project-agent is woken with the answer "yes"
    And the clarification message for the project forgelet-bridge carries the bridge's ➡ reaction
    And the clarification message's thread holds no reply from the bridge
