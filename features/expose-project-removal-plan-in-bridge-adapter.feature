Feature: Expose Project Removal Planning In The Bridge Adapter

  # The host performs project-removal preflight through the installed adapter.
  # Planning validates the request and reports readiness without stopping the
  # bridge, connecting to Matrix or changing any persisted state.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge configuration of the forge root forge-a names the forge roots forge-a, forge-b
    And the adapter for the forge root forge-a has started the bridge

  # Expose Project Removal Planning In The Bridge Adapter 1: a ready plan is read only
  Scenario Outline: Expose Project Removal Planning In The Bridge Adapter 1: a ready plan is read only
    Given the bridge has recorded managed projects <project> and sibling for the forge forge-b
    When the adapter for the forge root forge-a plans removal of the project <project> from the forge root forge-b
    Then the project removal plan reports the status "ready"
    And the project removal plan left the bridge running
    And the project removal plan changed no bridge state, configuration, files or room membership
    And the project removal result carries no Matrix ids, user ids, access tokens, project names or forge paths

    Examples:
      | project |
      | alpha   |

  # Expose Project Removal Planning In The Bridge Adapter 2: bridge plan results are sanitized
  Scenario Outline: Expose Project Removal Planning In The Bridge Adapter 2: bridge plan results are sanitized
    Given the bridge has recorded managed projects target and sibling for the forge forge-b
    And the bridge will answer the next project removal plan with <binary_case>
    When the adapter for the forge root forge-a plans removal of the project target from the forge root forge-b
    Then the project removal plan reports the status "<status>" with the reason "<reason>"
    And the project removal plan invoked the bridge's --remove-project-plan operation for the target project
    And the project removal plan left the bridge running
    And the project removal plan changed no bridge state, configuration, files or room membership
    And the project removal result carries no Matrix ids, user ids, access tokens, project names or forge paths

    Examples:
      | binary_case        | status  | reason                   |
      | ready              | ready   |                          |
      | retryable          | retryable | matrix_unavailable     |
      | refused            | refused | incomplete_project_state |
      | extra-field        | refused | incomplete_project_state |
      | malformed          | retryable | invalid_plan           |
      | nonzero            | retryable | state_unavailable       |

  # Expose Project Removal Planning In The Bridge Adapter 3: unsafe requests never reach the bridge
  Scenario Outline: Expose Project Removal Planning In The Bridge Adapter 3: unsafe requests never reach the bridge
    Given the bridge has recorded managed projects target and sibling for the forge forge-b
    And the remove-project-plan bridge is watched and must not run
    When the adapter for the forge root forge-a plans removal of the <root_case> root and the <project_case> project
    Then the project removal plan reports the status "refused" with the reason "<reason>"
    And the project removal plan did not invoke the bridge
    And the project removal plan left the bridge running
    And the project removal plan changed no bridge state, configuration, files or room membership

    Examples:
      | root_case               | project_case     | reason          |
      | blank                   | target           | unsafe_root     |
      | unknown-absolute        | target           | unknown_root    |
      | target                  | blank            | unsafe_project  |
      | target                  | unknown          | unknown_project |

  # Expose Project Removal Planning In The Bridge Adapter 4: deployment carries every host command
  Scenario: Expose Project Removal Planning In The Bridge Adapter 4: deployment carries every host command
    Given the project's deployment conformance check
    Then the installed bridge adapter exposes remove-project-plan and remove-project
