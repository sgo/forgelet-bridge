# mutation-stamp: sha256=634760d90ebbaf6a358d3f47d5316279aa7ea146e7ad3f4448815faca1d615a9
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-26T11:47:40.322954Z","feature_name":"Fixture Git Isolation","feature_path":"features/fixture-git-isolation.feature","background_hash":"eab7aa60fba1e7d473a61d2dcd406e9dd2270c7c6fb84be07f7fc1f02b1af052","implementation_hash":"sha256:8bd0d2175babca61ac0091b78cbd9eec054952030f0a7e14f331dc35582a50da","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Fixture Git Isolation

  # Driving the forge's real dashboard against a fixture root has to be safe
  # from any lane: the git world of a fixture ends at the fixture. The suite
  # keeps exercising the production path — the bridge's send-back and the
  # dashboard's retry handling — and shows that the isolation holds, so a
  # fixture whose git commands walk out into the worktree the suite runs from
  # cannot pass unnoticed.

  Background:
    Given the acceptance suite remembers the worktree it runs from
    And the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started

  # Fixture Git Isolation 1: sending an approval back stays inside the fixture
  Scenario: Fixture Git Isolation 1: sending an approval back stays inside the fixture
    Given the forge's dashboard already holds the pending approval for the card phone-approvals with its handover roles
    And the bridge has caught up with the forge
    When the operator replies "the timesheet total is still wrong" in the approval message's thread for the card phone-approvals
    Then the forge's dashboard recorded the approval for the card phone-approvals as sent back with "the timesheet total is still wrong"
    And the fixture forge holds the snapshot it took of the card phone-approvals
    And the worktree the acceptance suite runs from still holds its head, its branches and its changes
