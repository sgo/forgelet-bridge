# mutation-stamp: sha256=afac64697b27fdedfb10c603d722e5acc51a8e853a2f2ccdd2286633ce14aa12
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-25T15:22:05.490052Z","feature_name":"Onboarding A Forge","feature_path":"features/onboarding-a-forge.feature","background_hash":"439544b01217c52119ca255eceefb109bc030e063df0a961696694ed5f5b692e","implementation_hash":"sha256:d6fc5c009ceda6825656b7df8124eaa29fe813afb87ac1c23558657455f30967","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Onboarding A Forge

  # A forge joining the space is a first appearance, and it must not read like a
  # hundred things happening at once. The feed exists so the operator can see
  # what is moving now, so a forge that arrives with a long history of finished
  # cards says what is in flight, or says nothing, and reports from the moment it
  # joined. The property to hold is that a first appearance produces no burst
  # proportional to the board it arrives with - the cards that finished days ago
  # are not news, and each of them buzzing a phone is the opposite of the quiet
  # that makes the feed worth reading.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the forge's board already holds 40 cards that finished

  # Onboarding A Forge 1: a forge joining with a history is not replayed
  Scenario: Onboarding A Forge 1: a forge joining with a history is not replayed
    When the bridge is started
    Then the activity room holds exactly 0 card updates
    And the bridge's status names the forge forge-a as one it reached

  # Onboarding A Forge 2: what moves after the forge joined is reported
  Scenario: Onboarding A Forge 2: what moves after the forge joined is reported
    Given the bridge is started
    And a quiet stretch passes with nothing changing
    When the forge's board already holds the card moving-card in the project forgelet-bridge in the lane coder
    Then the operator decrypts a card update saying the card moving-card appeared in the project forgelet-bridge in the lane coder
    And the activity room holds exactly 1 card update
