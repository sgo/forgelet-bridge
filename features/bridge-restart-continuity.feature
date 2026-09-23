# mutation-stamp: sha256=7199e733c26e7e614959bd93dcc37d40361d92b2b34cc8e2080038f5b12cfd25
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-23T11:48:51.826399Z","feature_name":"Bridge Restart Continuity","feature_path":"features/bridge-restart-continuity.feature","background_hash":"0957242f3378ed649219759eae4a7458319ffb474195e865247a544bbf0769a4","implementation_hash":"sha256:44723d600f670f6293041c9e2291fc57c86bc1b1df8ac539fcf5d42e8fc4fd85","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Bridge Restart Continuity

  # A restart never duplicates or replays what the bridge already delivered in
  # either direction.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started

  # Bridge Restart Continuity 1: a chat request and its answer are not replayed on the next start
  Scenario: Bridge Restart Continuity 1: a chat request and its answer are not replayed on the next start
    Given the forge's dashboard already holds the chat request "is the build green?"
    And the lieutenant answers the chat request "is the build green?" with "yes, the build is green"
    And the bridge has caught up with the forge
    When the bridge is stopped and started again
    Then chat room Chat holds exactly one chat message reading "is the build green?"
    And chat room Chat holds exactly one thread reply reading "yes, the build is green"

  # Bridge Restart Continuity 2: an operator message is not relayed to the forge twice
  Scenario: Bridge Restart Continuity 2: an operator message is not relayed to the forge twice
    When the operator sends the message "is the build green?" into chat room Chat
    And the bridge has caught up with the forge
    When the bridge is stopped and started again
    Then the forge holds exactly one chat request reading "is the build green?"
    And chat room Chat holds exactly one chat message reading "is the build green?"
