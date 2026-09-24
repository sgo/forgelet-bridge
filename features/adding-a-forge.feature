# mutation-stamp: sha256=4bd6f0661122cce233bcfaf355ad2fe7f94284f57125c0244ed58533ba867055
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-24T17:21:03.599735Z","feature_name":"Adding A Forge","feature_path":"features/adding-a-forge.feature","background_hash":"d31ef266305b23e885c050863c022be588db58e2aa107cc3a31129869eff6ae1","implementation_hash":"sha256:5706c7861405e016f277433f9f5f95f1d78e35d51b26b0be653b999f91e0e109","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Adding A Forge

  # Bringing a forge into the space is one entry in the bridge's configuration
  # and a restart, and the adapter does it so nothing is left to memory: it
  # validates the configuration before it writes, keeps a copy of what it
  # replaced - which is also the rollback - restarts the bridge, and names the
  # new forge in what it echoes. The adapter is invoked for the forge root it
  # serves rather than from this repository's own location. An entry that would
  # make the configuration invalid is refused with the good configuration left
  # where it was, and the forge the command names is the forge the bridge then
  # reaches.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the bridge configuration of the forge root forge-a names the forge root forge-a
    And the adapter for the forge root forge-a has started the bridge

  # Adding A Forge 1: the command adds the forge, keeps what it replaced, and restarts into it
  Scenario: Adding A Forge 1: the command adds the forge, keeps what it replaced, and restarts into it
    When the adapter for the forge root forge-a adds the forge root forge-b as Saibill
    Then the adapter's output names the forge Saibill
    And the adapter's output names the copy of the bridge configuration it kept
    And the bridge configuration of the forge root forge-a names the forge root forge-b as Saibill
    And the adapter kept a copy of the bridge configuration it replaced
    And the bridge's status names every configured forge as reached
    And the operator sees the forge space Saibill

  # Adding A Forge 2: an entry that would make the configuration invalid is refused
  Scenario: Adding A Forge 2: an entry that would make the configuration invalid is refused
    When the adapter for the forge root forge-a is asked to add the forge root forge-a as Duplicate
    Then the adapter refused the new forge
    And the bridge configuration of the forge root forge-a holds only the forge root forge-a
    And the bridge's status names every configured forge as reached
