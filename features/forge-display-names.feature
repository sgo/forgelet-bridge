Feature: Forge Display Names

  # The operator tells the forges apart on the phone. Each forge is configured
  # with the name the operator knows it by: its Matrix space carries that name,
  # and the bridge's messages inside that forge's room show it as the sender.
  # The folder a forge happens to live in is never used.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge is configured with the operator @operator:example.org
    And the bridge is configured with the forge root forge-a named Forgelet
    And the bridge is configured with the forge root forge-b named Saibill
    And the bridge is started

  # Forge Display Names 1: each forge space carries the name the operator gave that forge
  Scenario Outline: Forge Display Names 1: each forge space carries the name the operator gave that forge
    Then the operator sees the forge space <name>
    And the operator sees no forge space named <folder>

    Examples:
      | folder  | name     |
      | forge-a | Forgelet |
      | forge-b | Saibill  |

  # Forge Display Names 2: each forge's name is the sender on everything the bridge posts there
  Scenario Outline: Forge Display Names 2: each forge's name is the sender on everything the bridge posts there
    Given the forge <folder>'s dashboard already holds the chat request "is the build green?"
    Then the operator decrypts the chat message "is the build green?" in the forge <name>'s chat room sent under the name <name>
    When the lieutenant answers the chat request "is the build green?" with "yes, the build is green"
    Then the operator decrypts the thread reply "yes, the build is green" in the forge <name>'s chat room sent under the name <name> to the chat message "is the build green?"

    Examples:
      | folder  | name     |
      | forge-a | Forgelet |
      | forge-b | Saibill  |
