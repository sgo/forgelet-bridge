# mutation-stamp: sha256=283069c7b7d43a095acd35a0eae8890936e84256581bcfe5935357bcc9f22104
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-24T16:43:17.117088Z","feature_name":"Forge Startup Report","feature_path":"features/forge-startup-report.feature","background_hash":"82f3ad327b9bd9635067c590d7165b7ec8b83bc9f03e45725cadfe3eb8a6ea02","implementation_hash":"sha256:50354654a649450ec1f0e89e5cf6dac50df31840661c99e0bbc8c3b06115fdda","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Forge Startup Report

  # Bringing a forge into the space is a config entry and a restart, and whoever
  # does it has to be able to tell whether it worked without guessing at a log
  # line that may never have been written. On startup the bridge reports every
  # configured forge it reached and every one it did not: a forge it reached has
  # its space and rooms provisioned and the operator invited, and a forge it did
  # not reach is named as unreached rather than passing silently.
  # Reached means the bridge got to the forge — configured, its rooms created —
  # and not that the forge's work is finished. A forge that arrives with a board
  # full of cards has a backlog, and that backlog is a separate, visible thing:
  # naming it is not the same as failing to reach the forge, or adding a forge
  # that works would report failure and send its operator to a log for a queue.
  # Each room carries its forge's name as well as its channel — Chat (forge-b),
  # Approvals (forge-b) — because a room list shows them outside their space and
  # two Chats from two forges have to be told apart at a glance. The channel comes
  # first and the forge alongside it, so a flat list groups by channel; the space
  # keeps the forge's own name.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge is configured with the operator @operator:example.org

  # Forge Startup Report 1: the forges the bridge reached are named, with their rooms
  Scenario: Forge Startup Report 1: the forges the bridge reached are named, with their rooms
    Given the bridge is configured with the forge roots forge-a, forge-b
    And the bridge is started
    Then the bridge's status names every configured forge as reached
    And the forge space forge-a holds the chat room Chat (forge-a)
    And the operator is invited to the forge space forge-b
    And the forge space forge-b holds the chat room Chat (forge-b)
    And the forge space forge-b holds the approvals room Approvals (forge-b)
    And the forge space forge-b holds the activity room Activity (forge-b)
    And the forge space forge-b holds the clarifications room Clarifications (forge-b)

  # Forge Startup Report 2: a forge the bridge could not reach is named as unreached
  Scenario: Forge Startup Report 2: a forge the bridge could not reach is named as unreached
    Given the bridge is configured with the forge roots forge-a, forge-b
    And the forge forge-b's dashboard is stopped
    And the bridge is started
    Then the bridge's status names the forge forge-a as one it reached
    And the bridge's status names the forge forge-b as one it did not reach

  # Forge Startup Report 3: a forge with a backlog is reached, and its backlog is its own
  Scenario: Forge Startup Report 3: a forge with a backlog is reached, and its backlog is its own
    Given the bridge is configured with the forge root forge-a
    And the forge's dashboard already holds 40 chat requests
    When the bridge is started
    Then the bridge's status names the forge forge-a as one it reached
    And the bridge's status reports the work it still owes the forge
