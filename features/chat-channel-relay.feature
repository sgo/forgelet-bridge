# mutation-stamp: sha256=8b78941b6c91f5176a7db1b6f3aa9843147e533b9a2de10e57d055696954d7e0
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-25T20:44:52.512799Z","feature_name":"Chat Channel Relay","feature_path":"features/chat-channel-relay.feature","background_hash":"0957242f3378ed649219759eae4a7458319ffb474195e865247a544bbf0769a4","implementation_hash":"sha256:c48a064ec589656a4f26833b1484e1d04821b72eb5e41c6ea684b41ca4a52311","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Chat Channel Relay

  # The operator's chat channel in the forge is one conversation in the chat
  # room: what the forge holds appears in the room, what the operator sends
  # becomes a chat request for the lieutenant, and the lieutenant's answer
  # arrives as a reply in that message's thread. A reply the phone makes by
  # quoting a message sends the operator's own words, with the quote the phone
  # writes into the body left out of the request.
  # A message the operator writes inside a thread already carries a relation, and
  # a thread cannot begin at an event that carries one, so the answer to such a
  # message joins the thread the operator wrote in, anchored at its root: it
  # arrives there, not at the top of the room and not as a failure the bridge
  # retries forever.

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

  # Chat Channel Relay 5: a reply that quotes a message reaches the lieutenant as the operator's own words
  Scenario: Chat Channel Relay 5: a reply that quotes a message reaches the lieutenant as the operator's own words
    Given the forge's dashboard already holds the chat request "is the build green?"
    When the operator swipes a reply "yes, the build is green" to the chat message "is the build green?"
    Then the forge holds the chat request "yes, the build is green" the dashboard took and typed into the lieutenant's pane

  # Chat Channel Relay 6: the answer to a message written in a thread joins that thread
  Scenario: Chat Channel Relay 6: the answer to a message written in a thread joins that thread
    Given the forge's dashboard already holds the chat request "is the build green?"
    And the lieutenant answers the chat request "is the build green?" with "yes, the build is green"
    And the operator replies "one more thing" in the thread of the chat message "is the build green?"
    And the lieutenant answers the chat request "one more thing" with "ask away"
    Then the operator decrypts the thread reply "ask away" to the chat message "is the build green?"
    And the chat room holds exactly one copy of "ask away" and it is a thread reply
