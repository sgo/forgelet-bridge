# mutation-stamp: sha256=3ca06916e83c9516035312db410002e586640e2a66ca4071b893e2355f58e293
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-23T13:30:23.916342Z","feature_name":"Forge Display Names","feature_path":"features/forge-display-names.feature","background_hash":"a2e80a588ebd34d939f368da0e0463d73a5d83fe5b28bf91e2109ffccec81469","implementation_hash":"sha256:68fbabcf1549f27481530e8b73f1521ebed933550f0b005de7fe806fdc5489ad","scenarios":[{"index":0,"name":"Forge Display Names 1: each forge space carries the name the operator gave that forge","scenario_hash":"34b693623c70ac2036f2233bff599d6608ae595b58753bcbf1ff2dfc2dc566f0","mutation_count":4,"result":{"Total":4,"Killed":4,"Survived":0,"Errors":0},"tested_at":"2026-09-22T13:05:58.925482Z"},{"index":1,"name":"Forge Display Names 2: each forge's name is the sender on everything the bridge posts there","scenario_hash":"86ecab59f5d41d8f21d3f28ff3bbbaf9c6efa4df1b42a1709d2d9c16595f7ee4","mutation_count":4,"result":{"Total":4,"Killed":4,"Survived":0,"Errors":0},"tested_at":"2026-09-22T13:03:30.304569Z"}]}
# acceptance-mutation-manifest-end

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
