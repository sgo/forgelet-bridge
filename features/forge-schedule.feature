Feature: Forge Schedule

  # The doorbell is installed in both forges and nothing runs it: the machine's
  # only cadence is the stall watch's agent, which runs the watch's pass alone, so
  # a request lost while every role is healthy stays lost. One schedule per
  # machine now runs both passes over every forge root it serves — the watch's
  # pass over the roles, then a doorbell pass for each root. The tools stay
  # separate, because they answer different questions and their rules differ, and
  # there is no second agent: a watcher that dies quietly is exactly what the
  # watch's heartbeat exists to catch.
  #
  # The runner's only job is the cadence. It runs both passes even when one
  # fails, so a broken pass cannot take the other down; it leaves evidence that
  # it ran, so a schedule that stopped is visible rather than assumed; it names a
  # root it could not serve rather than skipping it in silence; and it runs the
  # doorbell's pass as the pass it is, so a role that is busy is left alone and
  # the pass says so.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the fixture forge roots forge-a and forge-b hold the project forgelet-bridge

  # Forge Schedule 1: the machine rings a lost request with nobody asking
  Scenario: Forge Schedule 1: the machine rings a lost request with nobody asking
    Given the project forgelet-bridge of the forge root forge-a is mastered by the role coder
    And the forge root forge-a gives the role coder a live session
    And the forge forge-a's dashboard already holds the chat request "is the build green?"
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the doorbell says the chat request "is the build green?" was never delivered and rung
    And the master role's pane holds the chat request "is the build green?" the doorbell typed
    And the schedule leaves evidence that it ran

  # Forge Schedule 2: a busy role is not forced, and the pass says so
  Scenario: Forge Schedule 2: a busy role is not forced, and the pass says so
    Given the project forgelet-bridge of the forge root forge-a is mastered by the role coder
    And the forge root forge-a gives the role coder a working session
    And the forge forge-a's dashboard already holds the chat request "is the build green?"
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the doorbell says the chat request "is the build green?" was not rung because the role was busy
    And the schedule leaves evidence that it ran

  # Forge Schedule 3: a pass that fails does not take the other down
  Scenario: Forge Schedule 3: a pass that fails does not take the other down
    Given the fixture forge root forge-a has no project structure
    And the project forgelet-bridge of the forge root forge-b is mastered by the role coder
    And the forge root forge-b gives the role coder a live session
    And the forge forge-b's dashboard already holds the chat request "is the build green?"
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the schedule names the forge root it could not serve
    And the doorbell says the chat request "is the build green?" was never delivered and rung
    And the schedule leaves evidence that it ran
