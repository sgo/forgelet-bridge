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
