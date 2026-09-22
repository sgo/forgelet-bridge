# mutation-stamp: sha256=267b4d6a81d1cb73a13afdba26d9af76376f3fbacc91a0aa872a1880087f6c65
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-22T19:37:56.402448Z","feature_name":"Phone Message Wake","feature_path":"features/phone-message-wake.feature","background_hash":"0957242f3378ed649219759eae4a7458319ffb474195e865247a544bbf0769a4","implementation_hash":"sha256:67b27f1a12e55a0fdb1b3f2bb2f46cb29cf5c1601459c8be57d948c0942edf5c","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Phone Message Wake

  # A message the operator sends from the phone has to reach the lieutenant the
  # way a message typed into the dashboard does: the forge's own dashboard takes
  # it and wakes the lieutenant by typing the request into the lieutenant's
  # pane. A chat request that appears without that wake is a message that went
  # nowhere, and only the forge's dashboard can produce the wake. The suite has
  # to run that dashboard rather than a stand-in, or a wake can be invented for
  # a request the dashboard never took.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started

  # Phone Message Wake 1: a message from the phone reaches the lieutenant with its wake
  Scenario: Phone Message Wake 1: a message from the phone reaches the lieutenant with its wake
    When the operator sends the message "is the build green?" into chat room Chat
    Then the forge holds a chat request reading "is the build green?"
    And the forge's dashboard typed the chat request reading "is the build green?" into the lieutenant's pane

  # Phone Message Wake 2: a request the dashboard takes itself is woken the same way
  Scenario: Phone Message Wake 2: a request the dashboard takes itself is woken the same way
    When the forge's dashboard takes the chat request "please look at the invoice card" the way its clients give it
    Then the forge's dashboard typed the chat request reading "please look at the invoice card" into the lieutenant's pane

  # Phone Message Wake 3: the answer to a message from the phone comes back to the room
  Scenario: Phone Message Wake 3: the answer to a message from the phone comes back to the room
    When the operator sends the message "is the build green?" into chat room Chat
    And the forge's dashboard typed the chat request reading "is the build green?" into the lieutenant's pane
    When the lieutenant answers the chat request "is the build green?" with "yes, the build is green"
    Then the operator decrypts the thread reply "yes, the build is green" to the chat message "is the build green?"
