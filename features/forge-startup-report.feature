# mutation-stamp: sha256=e30ae48b43c7e82a8631d7b6d7664ac31f1f0d0747688937dff486ab03042704
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-23T18:25:51.134928Z","feature_name":"Forge Startup Report","feature_path":"features/forge-startup-report.feature","background_hash":"82f3ad327b9bd9635067c590d7165b7ec8b83bc9f03e45725cadfe3eb8a6ea02","implementation_hash":"sha256:50354654a649450ec1f0e89e5cf6dac50df31840661c99e0bbc8c3b06115fdda","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Forge Startup Report

  # Bringing a forge into the space is a config entry and a restart, and whoever
  # does it has to be able to tell whether it worked without guessing at a log
  # line that may never have been written. On startup the bridge reports every
  # configured forge it reached and every one it did not: a forge it reached has
  # its space and rooms provisioned and the operator invited, and a forge it did
  # not reach is named as unreached rather than passing silently.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge is configured with the operator @operator:example.org

  # Forge Startup Report 1: the forges the bridge reached are named, with their rooms
  Scenario: Forge Startup Report 1: the forges the bridge reached are named, with their rooms
    Given the bridge is configured with the forge roots forge-a, forge-b
    And the bridge is started
    Then the bridge's status names every configured forge as reached
    And the operator is invited to the forge space forge-b
    And the forge space forge-b holds the chat room Chat
    And the forge space forge-b holds the approvals room Approvals
    And the forge space forge-b holds the activity room Activity
    And the forge space forge-b holds the clarifications room Clarifications

  # Forge Startup Report 2: a forge the bridge could not reach is named as unreached
  Scenario: Forge Startup Report 2: a forge the bridge could not reach is named as unreached
    Given the bridge is configured with the forge roots forge-a, forge-b
    And the forge forge-b's dashboard is stopped
    And the bridge is started
    Then the bridge's status names the forge forge-a as one it reached
    And the bridge's status names the forge forge-b as one it did not reach
