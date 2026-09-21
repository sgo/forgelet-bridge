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
