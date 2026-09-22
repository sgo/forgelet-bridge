# mutation-stamp: sha256=b6bf8563394603a7e74642cb306af853cd2c96bd10a03faae0fb64ac8651b55d
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-22T13:03:31.337477Z","feature_name":"Operator Allowlist","feature_path":"features/operator-allowlist.feature","background_hash":"0957242f3378ed649219759eae4a7458319ffb474195e865247a544bbf0769a4","implementation_hash":"sha256:9d95c809c76568696066f7f3acd064961253dca5ca5e88d48d25a4512bf9442e","scenarios":[{"index":0,"name":"Operator Allowlist 1: only the configured operator's messages reach the forge","scenario_hash":"a3c37aa6087a06cb0b80e6046520f1bb2dcb97dbdfddb3c9fd71162002ed3300","mutation_count":4,"result":{"Total":4,"Killed":4,"Survived":0,"Errors":0},"tested_at":"2026-09-21T21:38:19.250962Z"}]}
# acceptance-mutation-manifest-end

Feature: Operator Allowlist

  # The bridge relays the chat channel of one operator. Messages from any other
  # Matrix user in the room never reach the forge.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started

  # Operator Allowlist 1: only the configured operator's messages reach the forge
  Scenario Outline: Operator Allowlist 1: only the configured operator's messages reach the forge
    Given the matrix client <sender> has joined chat room Chat
    When <sender> sends the message "is the build green?" into chat room Chat
    And the bridge has caught up with the forge
    Then the forge holds <chat_requests> chat requests

    Examples:
      | sender                | chat_requests |
      | @operator:example.org | 1             |
      | @stranger:example.org | 0             |
