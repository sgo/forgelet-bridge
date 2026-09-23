# mutation-stamp: sha256=af2c503e60ce8a9eea3b5cdd57200d37a8f122b71be27935a277288270938b61
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-23T20:25:12.385427Z","feature_name":"Installing Brings The Kit","feature_path":"features/installing-brings-the-kit.feature","background_hash":"c3482943d65c6e74891e8db70a351607d417005d9f06d1e0503d20e76ef94221","implementation_hash":"sha256:b06eb840e702902aa11dc4320cbd52c393011f05f9942a67600131be6d4d6107","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Installing Brings The Kit

  # Installing the bridge for a forge brings the tools that make the forge
  # behave, the way it already brings the rules: the route gate, the idler check
  # and the stall watch, in the forge's own scripts, with the watch's agent.
  # Each one is self-checked against the forge it was installed into - one live
  # pane, one board row, one inbox - so a tool that reads nothing is a failure on
  # the page rather than a clean-looking silence, and the self-check prints the
  # command and the marker it looked for. The installer ships tools; it never
  # decides policy: the gate's strictness stays in the forge's own lieutenant
  # prompt, and the report says which policy it found and left alone.

  Background:
    Given the fixture forge root forge-a carries its own lieutenant prompt
    And the fixture forge root forge-a holds the project forgelet-bridge

  # Installing Brings The Kit 1: the tools land, and each reads the forge it was installed into
  Scenario: Installing Brings The Kit 1: the tools land, and each reads the forge it was installed into
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    And the project forgelet-bridge of the forge root forge-a keeps a note waiting to be picked up
    When the adapter for the forge root forge-a installs the kit
    Then the installer's output says it installed the route gate, the idler check and the stall watch
    And the forge root forge-a carries the route gate, the idler check and the stall watch
    And the self-check of the idler check names the command it ran, the pane it read and the marker it looked for
    And the self-check of the idler check names the card and the mail it read

  # Installing Brings The Kit 2: a tool that reads nothing is a failure, not silence
  Scenario: Installing Brings The Kit 2: a tool that reads nothing is a failure, not silence
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    When the adapter for the forge root forge-a installs the kit
    Then the self-check of the idler check reports the forge as not running
    And the installer's output says the self-check failed

  # Installing Brings The Kit 3: running it again is safe, and the policy is left alone
  Scenario: Installing Brings The Kit 3: running it again is safe, and the policy is left alone
    Given the adapter for the forge root forge-a installs the kit
    When the adapter for the forge root forge-a runs the kit install again
    Then the installer's output says the kit was already current
    And the installer changed nothing in the forge root forge-a
    And the installer's output says it left the gate policy in the lieutenant prompt alone
