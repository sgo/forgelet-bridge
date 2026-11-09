Feature: Adopt memory Matrix backend for waiting facts

  # The backend policy belongs to acceptance fixtures. It selects a homeserver
  # per feature while the bridge, dashboards, adapters and Matrix clients remain
  # real. Fresh Synapse remains the default and the compatibility lane.

  Scenario Outline: Policy maps the waiting facts feature to memory
    Given a Matrix backend policy maps <listed_feature> to <listed_backend>
    When that policy resolves <requested_feature>
    Then the selected Matrix backend is <selected_backend>

    Examples:
      | listed_feature                       | listed_backend | requested_feature                    | selected_backend |
      | features/waiting-facts-as-state.feature | memory      | features/waiting-facts-as-state.feature | memory         |

  Scenario: An unlisted feature uses fresh Synapse
    Given a Matrix backend policy maps features/waiting-facts-as-state.feature to memory
    When that policy resolves features/bridge-device-continuity.feature
    Then the selected Matrix backend is fresh

  Scenario: A focused run can force fresh Synapse over the policy
    Given a Matrix backend policy maps features/waiting-facts-as-state.feature to memory
    When that policy resolves features/waiting-facts-as-state.feature with forced backend fresh
    Then the selected Matrix backend is fresh

  Scenario: A focused run can force memory over the policy
    Given a Matrix backend policy maps features/waiting-facts-as-state.feature to fresh
    When that policy resolves features/waiting-facts-as-state.feature with forced backend memory
    Then the selected Matrix backend is memory

  Scenario: The checked-in policy keeps waiting facts fast and compatibility real
    Given the checked-in Matrix backend policy
    Then features/waiting-facts-as-state.feature selects memory
    And the fresh compatibility features select fresh Synapse

  Scenario: Malformed policy fails instead of weakening coverage
    Given a malformed Matrix backend policy
    When the Matrix backend policy is resolved
    Then Matrix backend policy resolution fails visibly

  Scenario: Unsafe policy fails instead of weakening coverage
    Given an unsafe Matrix backend policy
    When the Matrix backend policy is resolved
    Then Matrix backend policy resolution fails visibly
