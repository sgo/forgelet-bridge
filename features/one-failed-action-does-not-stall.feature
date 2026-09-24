# mutation-stamp: sha256=8aecbf6c151efc0534a1562fe41d85a6540b8cd7fb63a717c35b10b3d5c3a535
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-24T17:21:09.273906Z","feature_name":"One Failed Action Does Not Stall","feature_path":"features/one-failed-action-does-not-stall.feature","background_hash":"0957242f3378ed649219759eae4a7458319ffb474195e865247a544bbf0769a4","implementation_hash":"sha256:757cea3e590a205975a4203c18a0623b5d054f54017d3de28417b11c51f1dfbd","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: One Failed Action Does Not Stall

  # An action the bridge cannot carry out is reported and retried, and the rest
  # of the work goes ahead: an approval, the card activity and the other
  # messages are not held hostage by one message the forge refused. What a tick
  # took from the rooms is not lost when the work it planned then fails, and a
  # bridge that is stuck is visible from the phone instead of looking quiet.
  # A forge's card feed is the action most likely to fail on a forge that has
  # just arrived - a board too big to read in one turn - and it is the least
  # important: the chat, approvals and clarifications a forge was just given must
  # not wait behind it.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started

  # One Failed Action Does Not Stall 1: a refused send-back is reported, retried, and holds nothing else up
  Scenario: One Failed Action Does Not Stall 1: a refused send-back is reported, retried, and holds nothing else up
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the forge's dashboard already holds a pending approval for the card card-activity-feed it cannot send back
    And the forge's board already holds the card card-activity-feed in the project forgelet-bridge in the lane coder
    And the bridge has caught up with the forge
    When the operator replies "bring this back" in the approval message's thread for the card card-activity-feed
    Then the operator decrypts the approval reply "Could not send it back" to the approval message for the card card-activity-feed
    And the approval message for the card phone-approvals names the project forgelet-bridge, the gate "coder → refactorer", and the changed files internal/bridge/bridge.go and internal/relay/relay.go
    And the operator decrypts a card update saying the card card-activity-feed appeared in the project forgelet-bridge in the lane coder
    When the forge repairs the approval for the card card-activity-feed
    Then the forge's dashboard recorded the approval for the card card-activity-feed as sent back with "bring this back"

  # One Failed Action Does Not Stall 2: a gesture that arrives while the failure is in flight still lands
  Scenario: One Failed Action Does Not Stall 2: a gesture that arrives while the failure is in flight still lands
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the forge's dashboard already holds a pending approval for the card card-activity-feed it cannot send back
    And the bridge has caught up with the forge
    When the operator replies "bring this back" in the approval message's thread for the card card-activity-feed
    Then the operator decrypts the approval reply "Could not send it back" to the approval message for the card card-activity-feed
    When the operator taps ✅ on the approval message for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as approved

  # One Failed Action Does Not Stall 3: a card feed that cannot be read holds nothing else up
  Scenario: One Failed Action Does Not Stall 3: a card feed that cannot be read holds nothing else up
    Given the forge's board cannot be read
    And the forge's dashboard already holds the chat request "is the build green?"
    And the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    When the bridge is started
    Then the operator decrypts the chat message "is the build green?"
    And the approval message for the card phone-approvals names the project forgelet-bridge, the gate "coder → refactorer", and the changed files internal/bridge/bridge.go and internal/relay/relay.go
    And the bridge's status names the forge forge-a as the only unhappy one
