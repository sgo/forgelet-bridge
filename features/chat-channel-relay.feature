Feature: Chat Channel Relay

  # The operator's chat channel in the forge is one conversation in the chat
  # room: what the forge holds appears in the room, what the operator sends
  # becomes a chat request for the lieutenant, and the lieutenant's answer
  # arrives as a reply in that message's thread.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started

  # Chat Channel Relay 1: an outstanding chat request appears in the chat room before any answer
  Scenario: Chat Channel Relay 1: an outstanding chat request appears in the chat room before any answer
    Given the forge's dashboard already holds the chat request "is the build green?"
    Then the operator decrypts the chat message "is the build green?"
    And the bridge sent the chat message "is the build green?" encrypted
    When the lieutenant answers the chat request "is the build green?" with "yes, the build is green"
    Then the operator decrypts the thread reply "yes, the build is green" to the chat message "is the build green?"

  # Chat Channel Relay 2: the operator's message reaches the lieutenant as a chat request
  Scenario: Chat Channel Relay 2: the operator's message reaches the lieutenant as a chat request
    When the operator sends the message "is the build green?" into chat room Chat
    Then the forge holds a chat request reading "is the build green?"
    And the lieutenant is woken with the chat request reading "is the build green?"

  # Chat Channel Relay 3: the lieutenant's answer arrives as a reply in the operator's thread
  Scenario: Chat Channel Relay 3: the lieutenant's answer arrives as a reply in the operator's thread
    When the operator sends the message "is the build green?" into chat room Chat
    And the lieutenant answers the chat request "is the build green?" with "yes, the build is green"
    Then the operator decrypts the thread reply "yes, the build is green" to the chat message "is the build green?"

  # Chat Channel Relay 4: each chat request carries its own thread
  Scenario: Chat Channel Relay 4: each chat request carries its own thread
    Given the forge's dashboard already holds the chat request "is the build green?"
    And the forge's dashboard already holds the chat request "please retry the invoice card"
    And the lieutenant answers the chat request "is the build green?" with "yes, the build is green"
    And the lieutenant answers the chat request "please retry the invoice card" with "retried, the card is queued"
    Then the operator decrypts the thread reply "yes, the build is green" to the chat message "is the build green?"
    And the operator decrypts the thread reply "retried, the card is queued" to the chat message "please retry the invoice card"
