# mutation-stamp: sha256=4243cf156e78003edc4f5ccc997ddf08cb706b9c9b80fbb6deb894662b42e24b
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-23T18:58:23.351997Z","feature_name":"Bridge Space Provisioning","feature_path":"features/bridge-space-provisioning.feature","background_hash":"82f3ad327b9bd9635067c590d7165b7ec8b83bc9f03e45725cadfe3eb8a6ea02","implementation_hash":"sha256:bae091ed9ea79d24c9bb62f8ecc275da52fb460f4b60b9655945b1f6de29b6de","scenarios":[{"index":2,"name":"Bridge Space Provisioning 3: gives each configured forge root its own forge space","scenario_hash":"6da79c82650313c481ca3babba60a7dd4c73bc2de49f59d6a286976f13c61627","mutation_count":4,"result":{"Total":4,"Killed":4,"Survived":0,"Errors":0},"tested_at":"2026-09-21T21:43:38.062882Z"}]}
# acceptance-mutation-manifest-end

Feature: Bridge Space Provisioning

  # The bridge owns the Matrix side of a forge. It creates the forge's space
  # and the chat room inside it, with encryption on from creation, and it
  # reuses what it created on every later start.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge is configured with the operator @operator:example.org

  # Bridge Space Provisioning 1: creates the forge space and an encrypted chat room inside it
  Scenario: Bridge Space Provisioning 1: creates the forge space and an encrypted chat room inside it
    Given the bridge is configured with the forge root forge-a
    When the bridge is started
    Then the operator sees the forge space forge-a
    And the operator is invited to the forge space forge-a
    And the forge space forge-a holds the chat room Chat
    And the operator is invited to chat room Chat
    And chat room Chat is encrypted

  # Bridge Space Provisioning 2: reuses the forge space and chat room it created on the next start
  Scenario: Bridge Space Provisioning 2: reuses the forge space and chat room it created on the next start
    Given the bridge is configured with the forge root forge-a
    And the bridge has created the forge space forge-a and its chat room Chat
    When the bridge is stopped and started again
    Then the operator sees exactly one forge space named forge-a
    And the forge space forge-a holds exactly one chat room Chat

  # Bridge Space Provisioning 3: gives each configured forge root its own forge space
  Scenario Outline: Bridge Space Provisioning 3: gives each configured forge root its own forge space
    Given the bridge is configured with the forge roots <forge_roots>
    When the bridge is started
    Then the operator sees <forge_spaces> forge spaces

    Examples:
      | forge_roots                | forge_spaces |
      | forge-a                    | 1            |
      | forge-a, forge-b           | 2            |
