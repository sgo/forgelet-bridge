Feature: Phone Approvals

  # A pending approval reaches the operator's phone: which project and card it
  # belongs to, who is handing what to whom, and what changed. The message
  # names the handover roles when the forge reports them, and falls back to the
  # gate exactly as reported when it does not, because the bridge has to work
  # against a forge that does not expose them yet. The operator approves with
  # the check mark or in plain text, sends the approval back by replying with
  # anything else, and the room shows the outcome once the approval is resolved
  # from either device. A gesture the room cannot read is answered with the ones
  # it takes, so silence never has to be guessed at. Deleting work and tearing
  # projects down stay on the desktop.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the forge space forge-a holds the approvals room Approvals
    And the operator is invited to the approvals room Approvals
    And the approvals room Approvals is encrypted

  # Phone Approvals 1: an approval reaches the phone with what it takes to decide
  Scenario Outline: Phone Approvals 1: an approval reaches the phone with what it takes to decide
    Given the forge's dashboard already holds the pending approval for the card phone-approvals <roles>
    Then the approval message for the card phone-approvals names the project forgelet-bridge, the gate "<gate>", and the changed files internal/bridge/bridge.go and internal/relay/relay.go

    Examples:
      | roles                      | gate               |
      | with its handover roles    | coder → refactorer |
      | without its handover roles | spec → refactorer  |

  # Phone Approvals 2: the operator approves the approval by reacting
  Scenario: Phone Approvals 2: the operator approves the approval by reacting
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator taps ✅ on the approval message for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as approved
    And the operator decrypts the approval reply "Approved" to the approval message for the card phone-approvals

  # Phone Approvals 3: the operator sends the approval back by replying in its thread
  Scenario: Phone Approvals 3: the operator sends the approval back by replying in its thread
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator replies "the timesheet total is still wrong" in the approval message's thread for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as sent back with "the timesheet total is still wrong"
    And the operator decrypts the approval reply "Sent back with feedback" to the approval message for the card phone-approvals

  # Phone Approvals 4: a reply that only approves approves, and carries no feedback
  Scenario Outline: Phone Approvals 4: a reply that only approves approves, and carries no feedback
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator replies "<answer>" in the approval message's thread for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as approved
    And the operator decrypts the approval reply "Approved" to the approval message for the card phone-approvals

    Examples:
      | answer          |
      | approve         |
      | go ahead        |
      | phone-approvals |

  # Phone Approvals 5: a message in the room that only approves approves
  Scenario Outline: Phone Approvals 5: a message in the room that only approves approves
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator sends the message "<answer>" into the approvals room
    Then the forge's dashboard recorded the approval for the card phone-approvals as approved
    And the operator decrypts the approval reply "Approved" to the approval message for the card phone-approvals

    Examples:
      | answer          |
      | approve         |
      | phone-approvals |

  # Phone Approvals 6: a reply that says more than the word is still a send-back
  Scenario: Phone Approvals 6: a reply that says more than the word is still a send-back
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator replies "approve please" in the approval message's thread for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as sent back with "approve please"

  # Phone Approvals 7: a reaction that is not the check mark approves nothing, and the room answers
  Scenario: Phone Approvals 7: a reaction that is not the check mark approves nothing, and the room answers
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator reacts 🎉 to the approval message for the card phone-approvals
    Then the approval for the card phone-approvals is still pending in the forge
    And the bridge answers in the approvals room with the gestures it takes

  # Phone Approvals 8: a message that approves nothing is left alone, and the room answers
  Scenario: Phone Approvals 8: a message that approves nothing is left alone, and the room answers
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    And the matrix client @stranger:example.org has joined the approvals room
    When the operator sends the message "when is this due?" into the approvals room
    And the matrix client @stranger:example.org reacts ✅ to the approval message for the card phone-approvals
    Then the approval for the card phone-approvals is still pending in the forge
    And the forge holds 0 chat requests
    And the forge's dashboard was never asked to delete or tear down
    And the bridge answers in the approvals room with the gestures it takes

  # Phone Approvals 9: an approval resolved on the desktop cannot be approved again
  Scenario: Phone Approvals 9: an approval resolved on the desktop cannot be approved again
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator approves the approval for the card phone-approvals from the desktop
    Then the operator decrypts the approval reply "Resolved on the desktop" to the approval message for the card phone-approvals
    When the operator taps ✅ on the approval message for the card phone-approvals
    Then the forge's dashboard recorded exactly one resolution for the card phone-approvals
    And the approval message's thread holds exactly one reply
