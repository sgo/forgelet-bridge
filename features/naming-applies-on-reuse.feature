# mutation-stamp: sha256=7836164a58e9329d44ee4f737246abe8ea970bcfaff82e66c94967682bdfb945
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-24T14:35:08.568565Z","feature_name":"Naming Applies On Reuse","feature_path":"features/naming-applies-on-reuse.feature","background_hash":"597b30070087a4868ae2cda8070f595fe8c4b3a9ce7a8e050b619863cfc798f4","implementation_hash":"sha256:31e6f074c9849ebf26319c4f20fe7c21074b546c0c653e8d0ade5dabe761e2e6","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Naming Applies On Reuse

  # The name a forge is configured with is a fact about that forge on every
  # start, not a one-time act of creating its rooms: a forge the bridge already
  # knows takes the name its configuration gives it, the way a forge whose rooms
  # are created now does, and nothing is duplicated by doing so.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the operator @operator:example.org

  # Naming Applies On Reuse 1: an existing forge takes the name its configuration gives it
  Scenario: Naming Applies On Reuse 1: an existing forge takes the name its configuration gives it
    Given the bridge is configured with the forge root forge-a
    And the bridge is started
    And the forge's dashboard already holds the chat request "is the build green?"
    When the bridge is configured with the forge root forge-a named Forgelet
    And the bridge is stopped and started again
    Then the operator sees exactly one forge space named Forgelet
    And the operator sees no forge space named forge-a
    And the forge space Forgelet holds exactly one chat room Chat
    And the operator decrypts the chat message "is the build green?" in the forge Forgelet's chat room sent under the name Forgelet

  # Naming Applies On Reuse 2: a forge created with its name keeps it on the next start
  Scenario: Naming Applies On Reuse 2: a forge created with its name keeps it on the next start
    Given the bridge is configured with the forge root forge-a named Forgelet
    And the bridge is started
    When the bridge is stopped and started again
    Then the operator sees exactly one forge space named Forgelet
    And the operator sees no forge space named forge-a
    And the forge space Forgelet holds exactly one chat room Chat

  # Naming Applies On Reuse 3: a restart leaves every fact about the forge as its configuration says
  Scenario: Naming Applies On Reuse 3: a restart leaves every fact about the forge as its configuration says
    Given the bridge is configured with the forge root forge-a named Forgelet
    And the bridge is started
    When the bridge is stopped and started again
    Then the operator is invited to the forge space Forgelet
    And the forge space Forgelet holds exactly one chat room Chat
    And chat room Chat is encrypted
    And the forge space Forgelet holds the approvals room Approvals
    And the approvals room Approvals is encrypted
    And the forge space Forgelet holds the activity room Activity
    And the activity room Activity is encrypted
