# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-10-03T12:20:36.266282Z","feature_name":"Phone Approvals","feature_path":"features/phone-approvals.feature","background_hash":"e11fbb1a65b036249f94bb008cdd0c50b30ce0e5c692957378a28ce82ba55d08","implementation_hash":"sha256:cc30eac780b5a6758e0c83d307311b9c3207ea09777a6ccd95063a2d5e971891","scenarios":[{"index":0,"name":"Phone Approvals 1: an approval reaches the phone with what it takes to decide","scenario_hash":"de45fd8612c8d3cf0b90541eb562ed67d4a4fd071b7b2e59ce5ddd925370241d","mutation_count":4,"result":{"Total":4,"Killed":4,"Survived":0,"Errors":0},"tested_at":"2026-09-23T11:52:50.482849Z"}]}
# acceptance-mutation-manifest-end

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
  # A reply the phone makes by quoting the approval message counts as a reply
  # too: it sends the work back unless its own words approve, and the quote the
  # phone writes into the body is not read as the operator's words.
  # The room answers the outcome of an approval on the approval's own message
  # rather than under it. An approval is reported by reacting ➡ on the message
  # the operator marked, and a send-back by reacting ⬅ on that same message, so
  # the bridge's mark sits on the line the operator acted on and the thread
  # stays quiet. The operator's own reply stays, because it is their words and
  # it is what went to the forge. A resolution made away from the room and
  # written down on the desk is reported the same way, with the mark for that
  # ending and no reply; one the desk wrote nothing down for keeps its threaded
  # "Resolved on the desktop" and takes no reaction. A resolution the bridge has
  # already reported is not reported again, whichever mark or message reported
  # it.
  # The bridge writes two events for one approval: the message the operator
  # reads and acts on from Element, and the state event that carries the
  # approval - its id, its card and its gate - which is the only thing the phone
  # can read, because its reading is the rooms' state. A signal on either event
  # is the same decision, so a check reaction the phone puts on the state event
  # approves exactly as one on the message does, and a reply that names the
  # state event is the send back a reply that names the message is. The state
  # event's own id is remembered when it is written, so a signal names the
  # approval it belongs to rather than being guessed at from the room, and only
  # the operator's signal counts on either event. The check mark counts in
  # either form the two sides send it - the phone's, and Element's with the
  # variation selector - so a mark the room reads as a decision is one the
  # bridge acts on.

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
    And the approval message for the card phone-approvals carries the bridge's ➡ reaction
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 3: the operator sends the approval back by replying in its thread
  Scenario: Phone Approvals 3: the operator sends the approval back by replying in its thread
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator replies "the timesheet total is still wrong" in the approval message's thread for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as sent back with exactly "the timesheet total is still wrong"
    And the approval message for the card phone-approvals carries the bridge's ⬅ reaction
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 4: a reply that only approves approves, and carries no feedback
  Scenario Outline: Phone Approvals 4: a reply that only approves approves, and carries no feedback
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator replies "<answer>" in the approval message's thread for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as approved
    And the approval message for the card phone-approvals carries the bridge's ➡ reaction
    And the approval message's thread holds no reply from the bridge

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
    And the approval message for the card phone-approvals carries the bridge's ➡ reaction
    And the approval message's thread holds no reply from the bridge

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

  # Phone Approvals 9: an approval approved on the desk carries its mark, and cannot be approved again
  Scenario: Phone Approvals 9: an approval approved on the desk carries its mark, and cannot be approved again
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator approves the approval for the card phone-approvals from the desktop
    Then the approval message for the card phone-approvals carries the bridge's ➡ reaction
    And the approval message's thread holds no reply from the bridge
    When the operator taps ✅ on the approval message for the card phone-approvals
    Then the forge's dashboard recorded exactly one resolution for the card phone-approvals
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 10: the operator sends the approval back by quoting it in a reply
  Scenario: Phone Approvals 10: the operator sends the approval back by quoting it in a reply
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator swipes a reply "refund figures do not add up" to the approval message for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as sent back with exactly "refund figures do not add up"
    And the approval message for the card phone-approvals carries the bridge's ⬅ reaction
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 11: a reply that quotes the approval is decided by its own words
  Scenario: Phone Approvals 11: a reply that quotes the approval is decided by its own words
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator swipes a reply "approve" to the approval message for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as approved
    And the approval message for the card phone-approvals carries the bridge's ➡ reaction
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 12: an approval already resolved cannot be sent back again
  Scenario: Phone Approvals 12: an approval already resolved cannot be sent back again
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator approves the approval for the card phone-approvals from the desktop
    Then the approval message for the card phone-approvals carries the bridge's ➡ reaction
    When the operator replies "send it back" in the approval message's thread for the card phone-approvals
    Then the forge's dashboard recorded exactly one resolution for the card phone-approvals
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 13: an approval sent back on the desk carries its mark
  Scenario: Phone Approvals 13: an approval sent back on the desk carries its mark
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator sends the approval for the card phone-approvals back from the desktop with "the totals do not add up"
    Then the forge's dashboard recorded the approval for the card phone-approvals as sent back with exactly "the totals do not add up"
    And the approval message for the card phone-approvals carries the bridge's ⬅ reaction
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 14: an approval resolved with no record speaks in the thread
  Scenario: Phone Approvals 14: an approval resolved with no record speaks in the thread
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the approval for the card phone-approvals is resolved on the desktop with no record
    Then the operator decrypts the approval reply "Resolved on the desktop" to the approval message for the card phone-approvals
    And the approval message's thread holds exactly one reply
    And the approval message for the card phone-approvals carries no reaction from the bridge

  # Phone Approvals 15: the operator approves the approval by reacting to its state event
  Scenario: Phone Approvals 15: the operator approves the approval by reacting to its state event
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator taps ✅ on the approval's state event for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as approved
    And the approval message for the card phone-approvals carries the bridge's ➡ reaction
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 16: a reaction to the approval's state event from anyone else decides nothing
  Scenario: Phone Approvals 16: a reaction to the approval's state event from anyone else decides nothing
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    And the matrix client @stranger:example.org has joined the approvals room
    When the matrix client @stranger:example.org reacts ✅ to the approval's state event for the card phone-approvals
    Then the approval for the card phone-approvals is still pending in the forge
    And the forge's dashboard was never asked to delete or tear down

  # Phone Approvals 17: the operator sends the approval back by replying to its state event
  Scenario: Phone Approvals 17: the operator sends the approval back by replying to its state event
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator replies "the timesheet total is still wrong" to the approval's state event for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as sent back with exactly "the timesheet total is still wrong"
    And the approval message for the card phone-approvals carries the bridge's ⬅ reaction
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 18: a check mark with the variation selector approves too
  Scenario: Phone Approvals 18: a check mark with the variation selector approves too
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator taps the check mark with the variation selector on the approval message for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as approved
    And the approval message for the card phone-approvals carries the bridge's ➡ reaction
    And the approval message's thread holds no reply from the bridge

  # Phone Approvals 19: a signal on one approval's state event decides only that approval
  Scenario: Phone Approvals 19: a signal on one approval's state event decides only that approval
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the forge's dashboard already holds the pending approval for the card refund-card with its handover roles
    And the bridge has caught up with the forge
    When the operator taps ✅ on the approval's state event for the card refund-card
    Then the forge's dashboard recorded the approval for the card refund-card as approved
    And the approval for the card phone-approvals is still pending in the forge
