Feature: Bridge Device Continuity

  # A restart has to leave the bridge on the same Matrix device, so the
  # operator never meets a new unverified device, and chat that happens after
  # the restart has to stay readable on the operator's phone.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started

  # Bridge Device Continuity 1: the restart reuses the bridge's Matrix device
  Scenario: Bridge Device Continuity 1: the restart reuses the bridge's Matrix device
    Given the bridge has published its Matrix device to the operator
    When the bridge is stopped and started again
    Then the operator sees the same bridge Matrix device

  # Bridge Device Continuity 2: a chat request delivered after the restart is readable
  Scenario: Bridge Device Continuity 2: a chat request delivered after the restart is readable
    Given the bridge has caught up with the forge
    When the bridge is stopped and started again
    And the forge's dashboard already holds the chat request "is the build green?"
    Then the operator decrypts the chat message "is the build green?"
    And the bridge sent the chat message "is the build green?" encrypted

  # Bridge Device Continuity 3: the reply and the answer still round-trip after the restart
  Scenario: Bridge Device Continuity 3: the reply and the answer still round-trip after the restart
    Given the bridge has caught up with the forge
    When the bridge is stopped and started again
    And the operator sends the message "is the build green?" into chat room Chat
    Then the forge holds a chat request reading "is the build green?"
    And the lieutenant is woken with the chat request reading "is the build green?"
    When the lieutenant answers the chat request "is the build green?" with "yes, the build is green"
    Then the operator decrypts the thread reply "yes, the build is green" to the chat message "is the build green?"
