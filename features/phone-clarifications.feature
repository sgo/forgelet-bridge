Feature: Phone Clarifications

  # A card blocked on a question is invisible on the phone: the forge's
  # dashboard holds the clarification, and nothing carries it to the operator's
  # space. The clarifications room carries each pending clarification - the
  # project it belongs to, the role that is blocked, and the question itself -
  # and the operator's reply in its thread is the answer: the reply is carried
  # back as the answer through the forge's own dashboard, so the blocked agent
  # wakes with it. A clarification answered from the desktop is reported in the
  # room rather than left looking open. What a reply means is what the room
  # says: in an approval's thread a reply sends the work back, in a
  # clarification's thread a reply is the answer.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the forge space forge-a holds the clarifications room Clarifications
    And the operator is invited to the clarifications room Clarifications
    And the clarifications room Clarifications is encrypted

  # Phone Clarifications 1: a pending clarification reaches the phone with what it takes to answer
  Scenario: Phone Clarifications 1: a pending clarification reaches the phone with what it takes to answer
    Given the forge's dashboard already holds the pending clarification from the role specifier in the project forgelet-bridge asking "which lane should the refund card start in?"
    Then the clarification message for the project forgelet-bridge names the project forgelet-bridge, the blocked role specifier, and the question "which lane should the refund card start in?"
    And the clarifications room holds exactly 1 clarification message
    And the approvals room holds no messages

  # Phone Clarifications 2: the operator's reply is the answer, carried back and woken
  Scenario: Phone Clarifications 2: the operator's reply is the answer, carried back and woken
    Given the forge's dashboard already holds the pending clarification from the role coder in the project forgelet-bridge asking "should the invoice card retry on its own?"
    And the bridge has caught up with the forge
    When the operator replies "yes" in the clarification message's thread for the project forgelet-bridge
    Then the forge's dashboard recorded the clarification for the project forgelet-bridge as answered with "yes"
    And the blocked role coder is woken with the answer "yes"
    And the operator decrypts the clarification reply "Answered" in the clarification message for the project forgelet-bridge

  # Phone Clarifications 3: a clarification answered on the desktop is marked resolved in the room
  Scenario: Phone Clarifications 3: a clarification answered on the desktop is marked resolved in the room
    Given the forge's dashboard already holds the pending clarification from the role refactorer in the project forgelet-bridge asking "may I delete the stale branch?"
    And the bridge has caught up with the forge
    When the operator answers the clarification for the project forgelet-bridge from the desktop with "keep it manual"
    Then the operator decrypts the clarification reply "Resolved on the desktop" in the clarification message for the project forgelet-bridge
    And the clarification message's thread holds exactly one reply

  # Phone Clarifications 4: each room says what a reply means
  Scenario: Phone Clarifications 4: each room says what a reply means
    Given the forge's dashboard already holds the pending clarification from the role coder in the project forgelet-bridge asking "should the retry wait for the nightly build?"
    And the forge's dashboard already holds the pending approval for the card phone-clarifications with its handover roles
    And the bridge has caught up with the forge
    Then the clarification message for the project forgelet-bridge tells the operator a reply is the answer
    And the approval message for the card phone-clarifications tells the operator a reply sends it back

  # Phone Clarifications 5: only the operator's reply answers the clarification
  Scenario: Phone Clarifications 5: only the operator's reply answers the clarification
    Given the forge's dashboard already holds the pending clarification from the role refactorer in the project forgelet-bridge asking "is the invoice card urgent?"
    And the bridge has caught up with the forge
    And the matrix client @stranger:example.org has joined the clarifications room
    When the matrix client @stranger:example.org replies "yes, very" in the clarification message's thread for the project forgelet-bridge
    Then the clarification for the project forgelet-bridge is still pending in the forge

  # Phone Clarifications 6: a message outside the clarification's thread answers nothing
  Scenario: Phone Clarifications 6: a message outside the clarification's thread answers nothing
    Given the forge's dashboard already holds the pending clarification from the role coder in the project forgelet-bridge asking "can I skip the fixture snapshot?"
    And the bridge has caught up with the forge
    When the operator sends the message "yes, skip it" into the clarifications room
    Then the clarification for the project forgelet-bridge is still pending in the forge
    And the forge holds 0 chat requests
