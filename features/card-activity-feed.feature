# mutation-stamp: sha256=ad853bbf5c0a1c5e05bdf89c31b6812539e8e7a402e14520bf203eca7d9ed8ea
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-24T17:21:05.530086Z","feature_name":"Card Activity Feed","feature_path":"features/card-activity-feed.feature","background_hash":"3a21e1611adb7d74b7f72e2f10edaeb952848835a2c3cc745f7a8c3d2b8bbdd8","implementation_hash":"sha256:e213ba696a2ebf2a3e0490b40d444727b817bceb5de9fe0d883e68c1a4ae69ec","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Card Activity Feed

  # As cards move through a project, short updates reach the operator's phone:
  # the project, the card, and where it moved. The updates live in their own
  # room, a log to keep quiet, apart from the room where approvals wait, and
  # nothing arrives unless a card actually changed, because the quiet between
  # updates is what tells the operator an agent may be stuck.
  # Not every update deserves a buzz, and which is which belongs here rather
  # than in a client setting: a card appearing and a card finishing are ordinary
  # messages that notify, because they are the news — work starting, work done —
  # while a routine lane-to-lane step is posted as the kind of event clients do
  # not notify on, worth seeing in the room and not worth waking anyone for.
  # A tick's card news travels as one message: every card that appeared or
  # finished in that tick is named in the message for that tick, so the volume is
  # bounded by ticks rather than by the size of a board. A tick with one card's
  # news reads as it always did; the change is about several. Nothing is dropped,
  # nothing arrives later than it would have, and a restart still re-posts
  # nothing.
  # A tick that carries both kinds keeps them apart: the cards that appeared or
  # finished are a notifying message of their own, and a lane move in the same
  # tick is the notice it always was, so sharing a tick with news can never
  # promote a move into something that wakes the operator.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started
    And the forge space forge-a holds the activity room Activity
    And the operator is invited to the activity room Activity
    And the activity room Activity is encrypted

  # Card Activity Feed 1: a card's progress reaches the phone as it moves
  Scenario: Card Activity Feed 1: a card's progress reaches the phone as it moves
    Given the forge's board already holds the card card-activity-feed in the project forgelet-bridge in the lane specifier
    Then the operator decrypts a card update saying the card card-activity-feed appeared in the project forgelet-bridge in the lane specifier
    When the forge moves the card card-activity-feed to the lane coder
    Then the operator decrypts a card update saying the card card-activity-feed moved on in the project forgelet-bridge to the lane coder
    When the forge finishes the card card-activity-feed
    Then the operator decrypts a card update saying the card card-activity-feed finished in the project forgelet-bridge
    And the card updates for the card card-activity-feed notify for its appearance and its finish, and not for the move between lanes
    And the approvals room holds no messages

  # Card Activity Feed 4: a tick's card news travels as one message
  Scenario: Card Activity Feed 4: a tick's card news travels as one message
    Given the forge's board already holds the cards first-card, second-card and third-card in the project forgelet-bridge in the lane specifier
    Then the operator decrypts one card update for the tick naming the cards first-card, second-card and third-card
    And the activity room holds exactly 1 card update
    When the bridge is stopped and started again
    And a quiet stretch passes with nothing changing
    Then the restart added no card update

  # Card Activity Feed 5: a tick that carries news and a move keeps them apart
  Scenario: Card Activity Feed 5: a tick that carries news and a move keeps them apart
    Given the forge's board already holds the card first-card in the project forgelet-bridge in the lane specifier
    And the forge's board already holds the card second-card in the project forgelet-bridge in the lane specifier
    And the bridge has caught up with the forge
    When the forge moves the card second-card to the lane coder and finishes the card first-card at once
    Then the operator decrypts a card update saying the card first-card finished in the project forgelet-bridge
    And the card update for the card first-card finished as a message rather than a notice
    And the card update for the card second-card moved on as a notice rather than a message

  # Card Activity Feed 2: a forge with nothing changing stays quiet
  Scenario: Card Activity Feed 2: a forge with nothing changing stays quiet
    Given the forge's board already holds the card card-activity-feed in the project forgelet-bridge in the lane specifier
    And the bridge has caught up with the forge
    Then the operator decrypts a card update saying the card card-activity-feed appeared in the project forgelet-bridge in the lane specifier
    When the operator sends the message "keep this quiet" into the activity room
    And a quiet stretch passes with nothing changing
    Then the activity room holds exactly 1 card update
    And the forge holds 0 chat requests

  # Card Activity Feed 3: a restart does not replay the updates it delivered
  Scenario: Card Activity Feed 3: a restart does not replay the updates it delivered
    Given the forge's board already holds the card card-activity-feed in the project forgelet-bridge in the lane specifier
    And the bridge has caught up with the forge
    Then the operator decrypts a card update saying the card card-activity-feed appeared in the project forgelet-bridge in the lane specifier
    When the bridge is stopped and started again
    And a quiet stretch passes with nothing changing
    Then the activity room holds exactly 1 card update
