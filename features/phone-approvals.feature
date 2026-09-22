# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-22T18:54:34.342860Z","feature_name":"Phone Approvals","feature_path":"features/phone-approvals.feature","background_hash":"e11fbb1a65b036249f94bb008cdd0c50b30ce0e5c692957378a28ce82ba55d08","implementation_hash":"sha256:cc30eac780b5a6758e0c83d307311b9c3207ea09777a6ccd95063a2d5e971891","scenarios":[{"index":0,"name":"Phone Approvals 1: an approval reaches the phone with what it takes to decide","scenario_hash":"de45fd8612c8d3cf0b90541eb562ed67d4a4fd071b7b2e59ce5ddd925370241d","mutation_count":4,"result":{"Total":4,"Killed":4,"Survived":0,"Errors":0},"tested_at":"2026-09-22T13:59:31.809760Z"}]}
# acceptance-mutation-manifest-end

Feature: Phone Approvals

  # A pending approval reaches the operator's phone: which project and card it
  # belongs to, who is handing what to whom, and what changed. The message
  # names the handover roles when the forge reports them, and falls back to the
  # gate exactly as reported when it does not, because the bridge has to work
  # against a forge that does not expose them yet. The operator approves by
  # reacting, sends the approval back by replying in its thread, and the room
  # shows the outcome once the approval is resolved from either device.
  # Deleting work and tearing projects down stay on the desktop.

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
    Then the forge recorded the approval for the card phone-approvals as approved
    And the operator decrypts the approval reply "Approved" to the approval message for the card phone-approvals

  # Phone Approvals 3: the operator sends the approval back by replying in its thread
  Scenario: Phone Approvals 3: the operator sends the approval back by replying in its thread
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator replies "the timesheet total is still wrong" in the approval message's thread for the card phone-approvals
    Then the forge recorded the approval for the card phone-approvals as sent back with "the timesheet total is still wrong"
    And the operator decrypts the approval reply "Sent back with feedback" to the approval message for the card phone-approvals

  # Phone Approvals 4: only the operator's ✅ reaction acts on the approval
  Scenario Outline: Phone Approvals 4: only the operator's ✅ reaction acts on the approval
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    And the matrix client @stranger:example.org has joined the approvals room
    When <reactor> reacts <reaction> to the approval message for the card phone-approvals
    And the operator sends the message "when is this due?" into the approvals room
    Then the approval for the card phone-approvals is still pending in the forge
    And the forge holds 0 chat requests
    And the forge was never asked to delete or tear down

    Examples:
      | reactor               | reaction |
      | the operator          | 🎉        |
      | @stranger:example.org | ✅        |

  # Phone Approvals 5: an approval resolved on the desktop cannot be approved again
  Scenario: Phone Approvals 5: an approval resolved on the desktop cannot be approved again
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator approves the approval for the card phone-approvals from the desktop
    Then the operator decrypts the approval reply "Resolved on the desktop" to the approval message for the card phone-approvals
    When the operator taps ✅ on the approval message for the card phone-approvals
    Then the forge recorded exactly one resolution for the card phone-approvals
    And the approval message's thread holds exactly one reply
