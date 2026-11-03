# mutation-stamp: sha256=7be0b7c5b0de34dce5768cfd6ca9d853914fcea6baca661d880a4812559ad638
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-11-03T00:31:23.140080Z","feature_name":"Remove A Forge From The Bridge","feature_path":"features/remove-forge-from-bridge.feature","background_hash":"ea1bff45dfa681f02cfcebb1aed35123bdd8d08adc59e548a592d46f9eb56ce5","implementation_hash":"sha256:5d417f41af5585192f1369bc3ac00ffe20313bddc338c0bfb936bcbd4c74b1f8","scenarios":[{"index":0,"name":"Remove A Forge From The Bridge 1: every recorded forge and project room is cleaned once","scenario_hash":"8ab0ba226cc11f1b7381e086b806a776a263ea7a6e8b8abd631a8852f739f8e1","mutation_count":4,"result":{"Total":4,"Killed":4,"Survived":0,"Errors":0},"tested_at":"2026-11-03T00:30:44.491115Z"},{"index":1,"name":"Remove A Forge From The Bridge 2: a partial Matrix failure can be repaired and retried","scenario_hash":"d39976dfc8a4ab9f0ef3fc93bd39e9f068b4bc5244aa9901d1376f14765b89f7","mutation_count":6,"result":{"Total":6,"Killed":6,"Survived":0,"Errors":0},"tested_at":"2026-11-03T00:23:00.541731Z"},{"index":5,"name":"Remove A Forge From The Bridge 6: unsafe or ambiguous roots are refused before Matrix changes","scenario_hash":"dd5ea21d4a51a219fb2b9f26082bc6126492bf1b66634bda8396945b0a8dd564","mutation_count":10,"result":{"Total":10,"Killed":10,"Survived":0,"Errors":0},"tested_at":"2026-11-03T00:23:00.541731Z"}]}
# acceptance-mutation-manifest-end

Feature: Remove A Forge From The Bridge

  # Removing a forge from configuration alone leaves its space and rooms on the
  # operator's phone. The adapter removes exactly one configured forge from
  # Matrix and from the bridge's bookkeeping, while retaining the forge files,
  # the bridge configuration, and enough state to retry a partial cleanup.
  #
  # The command returns one JSON object. Its status is "removed",
  # "nothing_to_remove", "retryable", or "refused". A non-removed result also
  # names a stable reason when there is one. It never carries credentials,
  # Matrix ids, or a forge path.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge configuration of the forge root forge-a names the forge roots forge-a, forge-b
    And the adapter for the forge root forge-a has started the bridge

  # Remove A Forge From The Bridge 1: every recorded forge and project room is cleaned once
  Scenario Outline: Remove A Forge From The Bridge 1: every recorded forge and project room is cleaned once
    Given the bridge has recorded <project_rooms> project rooms for the forge forge-b
    And the forge forge-b holds relay facts in its forge and project rooms
    When the adapter for the forge root forge-a removes the forge root forge-b
    Then the removal result reports the status "removed"
    And the removal targeted the forge-b space and its chat, approvals, activity and clarifications rooms, and exactly <expected_project_rooms> project rooms, each exactly once
    And the removal cleared the relay facts before leaving each recorded room
    And the removal removed the operator before the bridge left each recorded room
    And the operator and the bridge are no longer members of the forge-b space or any recorded room
    And the bridge's relay bookkeeping for forge-b was cleared only after every recorded room was cleaned
    And the bridge state records no forge or project rooms for forge-b
    And the bridge state still records forge-a and its project rooms
    And the bridge configuration is unchanged byte for byte
    And the forge-b files outside the bridge state are unchanged byte for byte

    Examples:
      | project_rooms | expected_project_rooms |
      | 1             | 1                      |
      | 3             | 3                      |

  # Remove A Forge From The Bridge 2: a partial Matrix failure can be repaired and retried
  Scenario Outline: Remove A Forge From The Bridge 2: a partial Matrix failure can be repaired and retried
    Given the bridge has recorded 2 project rooms for the forge forge-b
    And the forge forge-b holds relay facts in its forge and project rooms
    And Matrix refuses <failed_operation> in one recorded room
    When the adapter for the forge root forge-a removes the forge root forge-b
    Then the removal result reports the status "retryable" with the reason "<reason>"
    And the removal result carries no Matrix ids, user ids, access tokens or forge paths
    And the bridge state retains the forge-b space and every recorded forge and project room id
    And the bridge state retains the completed room cleanup progress
    And the bridge configuration is unchanged byte for byte
    And the forge-b files outside the bridge state are unchanged byte for byte
    When the forge repairs <failed_operation> in that room
    And the adapter for the forge root forge-a removes the forge root forge-b again
    Then the removal result reports the status "removed"
    And the bridge state records no forge or project rooms for forge-b

    Examples:
      | failed_operation                            | reason                     |
      | clearing the relay facts                     | relay_cleanup_failed       |
      | removing the operator                        | operator_removal_failed     |
      | leaving or forgetting the bridge membership  | bridge_membership_failed    |

  # Remove A Forge From The Bridge 3: repeating a completed removal changes nothing
  Scenario: Remove A Forge From The Bridge 3: repeating a completed removal changes nothing
    Given the bridge has recorded 1 project room for the forge forge-b
    And the adapter for the forge root forge-a has removed the forge root forge-b
    When the adapter for the forge root forge-a removes the forge root forge-b again
    Then the removal result reports the status "nothing_to_remove"
    And the repeated removal targeted no Matrix room
    And the bridge state, bridge configuration and forge-b files are unchanged by the repeated removal

  # Remove A Forge From The Bridge 4: an unknown root is refused before Matrix changes
  Scenario: Remove A Forge From The Bridge 4: an unknown root is refused before Matrix changes
    Given the fixture forge root forge-c has its dashboard running
    When the adapter for the forge root forge-a removes the forge root forge-c
    Then the removal result reports the status "refused" with the reason "unknown_root"
    And the refused removal targeted no Matrix room
    And the bridge state, bridge configuration and fixture forge files are unchanged

  # Remove A Forge From The Bridge 5: incomplete room state is refused before Matrix changes
  Scenario: Remove A Forge From The Bridge 5: incomplete room state is refused before Matrix changes
    Given the bridge has recorded 1 project room for the forge forge-b
    And the bridge state for forge-b is missing its activity room id
    When the adapter for the forge root forge-a removes the forge root forge-b
    Then the removal result reports the status "refused" with the reason "incomplete_state"
    And the refused removal targeted no Matrix room
    And the bridge state, bridge configuration and fixture forge files are unchanged

  # Remove A Forge From The Bridge 6: unsafe or ambiguous roots are refused before Matrix changes
  Scenario Outline: Remove A Forge From The Bridge 6: unsafe or ambiguous roots are refused before Matrix changes
    Given the bridge has recorded 1 project room for the forge forge-b
    When the adapter for the forge root forge-a is asked to remove the <argument_case> forge root
    Then the removal result reports the status "refused" with the reason "<reason>"
    And the refused removal targeted no Matrix room
    And the bridge state, bridge configuration and fixture forge files are unchanged

    Examples:
      | argument_case                              | reason          |
      | blank                                      | unsafe_root     |
      | parent-traversal                           | unsafe_root     |
      | relative                                   | unsafe_root     |
      | basename shared by two configured forges   | ambiguous_root  |
      | extra                                      | unsafe_root     |
