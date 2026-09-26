# mutation-stamp: sha256=a7c7ad50a4a1a56f054bd27e2ef251a9739d8624190f792d3e47034ecbe53cc9
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-26T11:47:43.369933Z","feature_name":"One Sick Forge Does Not Quiet The Others","feature_path":"features/one-sick-forge-does-not-quiet-the-others.feature","background_hash":"bfc93b9b67ef0944524d94c42a73e03a03105f8978aff8d8cfa8cccc8a904406","implementation_hash":"sha256:5fbfe3752d8cd685b05822b66a2f96cac5f6334398106a08e66dcaca0d713b3c","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: One Sick Forge Does Not Quiet The Others

  # The bridge carries more than one forge from one space. A forge it cannot
  # reach - its dashboard stopped, its address stale, an endpoint its tooling
  # does not expose - or a forge that refuses an action is a forge, not the
  # whole bridge: it is reported and retried on its own, the status the bridge
  # writes names it and stops naming it once it is served again, and every other
  # forge keeps carrying its rooms in both directions. What the rooms said in a
  # tick that failed for one forge is not swallowed for the others either.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge is configured with the forge roots forge-a, forge-b
    And the bridge is configured with the operator @operator:example.org

  # One Sick Forge Does Not Quiet The Others 1: a forge that cannot be reached is named and retried while the other keeps working
  Scenario: One Sick Forge Does Not Quiet The Others 1: a forge that cannot be reached is named and retried while the other keeps working
    Given the forge forge-b's dashboard is stopped
    And the forge forge-a's dashboard already holds the chat request "is the build green?"
    And the forge forge-a's board already holds the card one-sick-forge-does-not-quiet-the-others in the project forgelet-bridge in the lane specifier
    And the bridge is started
    Then the bridge's status names the forge forge-b as the only unhappy one
    And the operator decrypts the chat message "is the build green?" in the forge forge-a's chat room sent under the name forge-a
    And the operator decrypts a card update in the forge forge-a saying the card one-sick-forge-does-not-quiet-the-others appeared in the project forgelet-bridge in the lane specifier
    When the operator sends the message "please look at the refund card" into the forge forge-a's chat room
    And the operator sends the message "retry the invoice" into the forge forge-b's chat room
    Then the forge forge-a holds the chat request "please look at the refund card" the dashboard took and typed into the lieutenant's pane
    When the forge forge-b's dashboard is started again
    Then the forge forge-b holds the chat request "retry the invoice" the dashboard took and typed into the lieutenant's pane
    And the bridge's status names no unhappy forge

  # One Sick Forge Does Not Quiet The Others 2: a forge that refuses an action is named while the other keeps working
  Scenario: One Sick Forge Does Not Quiet The Others 2: a forge that refuses an action is named while the other keeps working
    Given the forge forge-a's dashboard already holds the chat request "is the build green?"
    And the forge forge-b's dashboard already holds a pending approval for the card phone-approvals it cannot send back
    And the bridge is started
    And the bridge has caught up with the forge
    When the operator replies "bring this back" in the approval message's thread for the card phone-approvals in the forge forge-b
    Then the operator decrypts the approval reply "Could not send it back" to the approval message for the card phone-approvals in the forge forge-b
    And the bridge's status names the forge forge-b as the only unhappy one
    And the operator decrypts the chat message "is the build green?" in the forge forge-a's chat room sent under the name forge-a
    When the forge forge-b repairs the approval for the card phone-approvals
    Then the operator decrypts the approval reply "Sent back with feedback" to the approval message for the card phone-approvals in the forge forge-b
    And the bridge's status names no unhappy forge
