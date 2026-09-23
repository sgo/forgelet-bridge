# mutation-stamp: sha256=33162114cb756afe284e189a576ae8acf38c4aa694b6c3e3effcd8726302d154
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-23T20:25:10.724861Z","feature_name":"Card Activity Feed","feature_path":"features/card-activity-feed.feature","background_hash":"3a21e1611adb7d74b7f72e2f10edaeb952848835a2c3cc745f7a8c3d2b8bbdd8","implementation_hash":"sha256:e213ba696a2ebf2a3e0490b40d444727b817bceb5de9fe0d883e68c1a4ae69ec","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Card Activity Feed

  # As cards move through a project, short updates reach the operator's phone:
  # the project, the card, and where it moved. The updates live in their own
  # room, a log to keep quiet, apart from the room where approvals wait, and
  # nothing arrives unless a card actually changed, because the quiet between
  # updates is what tells the operator an agent may be stuck.

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
    And the activity room holds exactly 3 card updates
    And the approvals room holds no messages

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
