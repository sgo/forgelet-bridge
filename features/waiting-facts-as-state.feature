Feature: Waiting Facts As State

  # A forge's rooms carry what moves: a card appearing, a card moving on, an
  # approval posted, a question asked. What they never carried was what is - the
  # cards a board holds now, the approvals still waiting, the questions still
  # open. The desk reads that from the dashboard, and a phone cannot reach a
  # dashboard, so the bridge writes those facts into the rooms as state: one
  # event per item, the item's own id as its state key, carrying the same facts
  # the dashboard serves, in the room that item belongs to. The board room
  # carries a project's board, the approvals room an approval, and the
  # clarifications room a question. A state event is the shape for a set that
  # updates and clears - an item whose facts change has its event written again,
  # and an item that stops waiting has it cleared with empty content - so
  # reading a room's state shows the current set and never a card in the lane it
  # left, an approval already resolved, or a question already answered. The
  # messages already in the rooms stay exactly as they are: the state is a
  # current-state view beside the story, not instead of it.
  #
  # The acceptance reads this back the way a phone would, from the state events
  # the rooms hold through the homeserver the run stands up, over a fixture
  # forge whose dashboard serves a known state: no agent and no phone take part.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the forge space forge-a holds the chat room Chat
    And the forge space forge-a holds the activity room Activity
    And the operator is invited to the activity room Activity
    And the forge space forge-a holds the approvals room Approvals
    And the operator is invited to the approvals room Approvals
    And the forge space forge-a holds the clarifications room Clarifications
    And the operator is invited to the clarifications room Clarifications

  # Waiting Facts As State 1: every waiting fact reaches its room as state, keyed by the item
  Scenario: Waiting Facts As State 1: every waiting fact reaches its room as state, keyed by the item
    Given the forge's board already holds the card phone-approvals in the project forgelet-bridge in the lane coder
    And the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the forge's dashboard already holds the pending clarification from the role coder in the project forgelet-bridge asking "should the invoice card retry on its own?"
    Then the activity room Activity carries the board of the project forgelet-bridge as a com.forgelet.board state event keyed by the project, with its lanes, holding the card phone-approvals in the lane coder
    And the approvals room Approvals carries the approval for the card phone-approvals as a com.forgelet.approval state event keyed by the approval, naming the project forgelet-bridge and the gate "coder → refactorer"
    And the clarifications room Clarifications carries the question from the role coder in the project forgelet-bridge as a com.forgelet.clarification state event keyed by the question, asking "should the invoice card retry on its own?"

  # Waiting Facts As State 2: a card that moves on is read where it is now, not where it was
  Scenario: Waiting Facts As State 2: a card that moves on is read where it is now, not where it was
    Given the forge's board already holds the card phone-approvals in the project forgelet-bridge in the lane master
    And the activity room Activity carries the board of the project forgelet-bridge as a com.forgelet.board state event keyed by the project, with its lanes, holding the card phone-approvals in the lane master
    When the forge moves the card phone-approvals to the lane coder
    And the bridge has caught up with the forge
    Then the activity room Activity carries the board of the project forgelet-bridge as a com.forgelet.board state event keyed by the project, with its lanes, holding the card phone-approvals in the lane coder

  # Waiting Facts As State 3: an approval that stops waiting is cleared
  Scenario: Waiting Facts As State 3: an approval that stops waiting is cleared
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the approvals room Approvals carries the approval for the card phone-approvals as a com.forgelet.approval state event keyed by the approval, naming the project forgelet-bridge and the gate "coder → refactorer"
    When the operator taps ✅ on the approval message for the card phone-approvals
    And the bridge has caught up with the forge
    Then the approvals room Approvals carries no facts for the approval for the card phone-approvals

  # Waiting Facts As State 4: a question that stops waiting is cleared
  Scenario: Waiting Facts As State 4: a question that stops waiting is cleared
    Given the forge's dashboard already holds the pending clarification from the role coder in the project forgelet-bridge asking "should the invoice card retry on its own?"
    And the clarifications room Clarifications carries the question from the role coder in the project forgelet-bridge as a com.forgelet.clarification state event keyed by the question, asking "should the invoice card retry on its own?"
    When the operator replies "yes" in the clarification message's thread for the project forgelet-bridge
    And the bridge has caught up with the forge
    Then the clarifications room Clarifications carries no facts for the question from the role coder in the project forgelet-bridge

  # Waiting Facts As State 5: a board that stops waiting is cleared
  Scenario: Waiting Facts As State 5: a board that stops waiting is cleared
    Given the forge's board already holds the card phone-approvals in the project forgelet-bridge in the lane coder
    And the activity room Activity carries the board of the project forgelet-bridge as a com.forgelet.board state event keyed by the project, with its lanes, holding the card phone-approvals in the lane coder
    When the forge closes the project forgelet-bridge
    And the bridge has caught up with the forge
    Then the activity room Activity carries no facts for the board of the project forgelet-bridge

  # Waiting Facts As State 6: a restart writes nothing the rooms already hold
  Scenario: Waiting Facts As State 6: a restart writes nothing the rooms already hold
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the approvals room Approvals carries the approval for the card phone-approvals as a com.forgelet.approval state event keyed by the approval, naming the project forgelet-bridge and the gate "coder → refactorer"
    When the bridge is stopped and started again
    And the bridge has caught up with the forge
    Then the approvals room Approvals still carries the approval state event it already had for the card phone-approvals

  # Waiting Facts As State 7: the state is read beside the story, not instead of it
  Scenario: Waiting Facts As State 7: the state is read beside the story, not instead of it
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    Then the approvals room Approvals carries the approval for the card phone-approvals as a com.forgelet.approval state event keyed by the approval, naming the project forgelet-bridge and the gate "coder → refactorer"
    And the approval message for the card phone-approvals names the project forgelet-bridge, the gate "coder → refactorer", and the changed files internal/bridge/bridge.go and internal/relay/relay.go
