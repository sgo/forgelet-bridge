Feature: Remove One Project Without Erasing Matrix History

  # A stopped project can be retired from the operator's room list without
  # removing the Family space, its forge-level rooms, any sibling project, or
  # the project's server-side Matrix history. The public result is deliberately
  # small and sanitized; retry progress lives only in bridge state.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge configuration of the forge root forge-a names the forge roots forge-a, forge-b
    And the adapter for the forge root forge-a has started the bridge

  # Remove One Project Without Erasing Matrix History 1: one project room becomes inactive
  Scenario Outline: Remove One Project Without Erasing Matrix History 1: one project room becomes inactive
    Given the bridge has recorded managed projects <project> and sibling for the forge forge-b
    And the forge forge-b holds project-scoped relay facts for <project> and sibling relay facts
    When the adapter for the forge root forge-a removes the project <project> from the forge root forge-b
    Then the project removal result reports the status "removed"
    And the adapter restarted the bridge after the project removal
    And the removal targeted only the <project> project room exactly once
    And the removal cleared the <project> relay facts before removing the operator before leaving the room
    And the operator and the bridge are no longer members of the <project> project room
    And an observer can still read the <project> room history
    And the bridge state no longer records the <project> project and still records sibling
    And the forge-level rooms, sibling room, sibling relay facts and forge-level relay facts are preserved
    And the bridge configuration and every forge and project file are unchanged
    And the project removal result carries no Matrix ids, user ids, access tokens, project names or forge paths

    Examples:
      | project |
      | alpha   |
      | beta    |

  # Remove One Project Without Erasing Matrix History 2: every matrix failure is retryable and resumable
  Scenario Outline: Remove One Project Without Erasing Matrix History 2: every matrix failure is retryable and resumable
    Given the bridge has recorded managed projects target and sibling for the forge forge-b
    And the forge forge-b holds project-scoped relay facts for target and sibling relay facts
    And Matrix refuses <failed_operation> in the target project room
    When the adapter for the forge root forge-a removes the project target from the forge root forge-b
    Then the project removal result reports the status "retryable" with the reason "<reason>"
    And the adapter restarted the bridge after the project removal
    And the bridge state retains the target project record and its completed cleanup progress
    And the forge-level rooms, sibling room, sibling relay facts and forge-level relay facts are preserved
    And the project removal result carries no Matrix ids, user ids, access tokens, project names or forge paths
    When the forge repairs <failed_operation> in the target project room
    And the adapter for the forge root forge-a removes the project target from the forge root forge-b again
    Then the project removal result reports the status "removed"
    And the bridge state no longer records the target project and still records sibling

    Examples:
      | failed_operation                            | reason                     |
      | clearing the relay facts                     | relay_cleanup_failed       |
      | removing the operator                        | operator_removal_failed    |
      | leaving or forgetting the bridge membership  | bridge_membership_failed   |

  # Remove One Project Without Erasing Matrix History 3: repeating a completed removal changes nothing
  Scenario: Remove One Project Without Erasing Matrix History 3: repeating a completed removal changes nothing
    Given the bridge has recorded managed projects target and sibling for the forge forge-b
    And the adapter for the forge root forge-a removes the project target from the forge root forge-b
    When the adapter for the forge root forge-a removes the project target from the forge root forge-b again
    Then the project removal result reports the status "nothing_to_remove"
    And the repeated project removal touched no Matrix room and changed no bridge state
    And the adapter restarted the bridge after the project removal

  # Remove One Project Without Erasing Matrix History 4: unsafe and unknown roots are refused before Matrix changes
  Scenario Outline: Remove One Project Without Erasing Matrix History 4: unsafe and unknown roots are refused before Matrix changes
    Given the bridge has recorded managed projects target and sibling for the forge forge-b
    When the adapter for the forge root forge-a is asked to remove the <root_case> root and the target project
    Then the project removal result reports the status "refused" with the reason "<reason>"
    And the refused project removal touched no Matrix room
    And the bridge state, bridge configuration and every forge and project file are unchanged
    And the adapter restarted the bridge after the project removal

    Examples:
      | root_case                | reason       |
      | blank                    | unsafe_root  |
      | relative                 | unsafe_root  |
      | parent-traversal         | unsafe_root  |
      | extra-arguments          | unsafe_root  |
      | unknown-absolute         | unknown_root |
      | configured-non-Forgelet  | unknown_root |

  # Remove One Project Without Erasing Matrix History 5: unsafe and unknown projects are refused before Matrix changes
  Scenario Outline: Remove One Project Without Erasing Matrix History 5: unsafe and unknown projects are refused before Matrix changes
    Given the bridge has recorded managed projects target and sibling for the forge forge-b
    When the adapter for the forge root forge-a is asked to remove the forge-b root and the <project_case> project
    Then the project removal result reports the status "refused" with the reason "<reason>"
    And the refused project removal touched no Matrix room
    And the bridge state, bridge configuration and every forge and project file are unchanged
    And the adapter restarted the bridge after the project removal

    Examples:
      | project_case      | reason          |
      | blank             | unsafe_project  |
      | padded            | unsafe_project  |
      | parent-traversal  | unsafe_project  |
      | absolute          | unsafe_project  |
      | matrix-room-id    | unsafe_project  |
      | unknown           | unknown_project |

  # Remove One Project Without Erasing Matrix History 6: incomplete project state is refused before Matrix changes
  Scenario: Remove One Project Without Erasing Matrix History 6: incomplete project state is refused before Matrix changes
    Given the bridge has recorded managed projects target and sibling for the forge forge-b
    And the bridge state for the target project is missing its room id
    When the adapter for the forge root forge-a removes the project target from the forge root forge-b
    Then the project removal result reports the status "refused" with the reason "incomplete_project_state"
    And the refused project removal touched no Matrix room
    And the bridge state, bridge configuration and every forge and project file are unchanged
    And the adapter restarted the bridge after the project removal
