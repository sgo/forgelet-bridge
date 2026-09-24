# mutation-stamp: sha256=f08fa9fc6af1e8535290c279939f81633a369d512722a9aa311b65e4af81236d
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-24T12:23:46.614925Z","feature_name":"Phone Message Wake","feature_path":"features/phone-message-wake.feature","background_hash":"0957242f3378ed649219759eae4a7458319ffb474195e865247a544bbf0769a4","implementation_hash":"sha256:67b27f1a12e55a0fdb1b3f2bb2f46cb29cf5c1601459c8be57d948c0942edf5c","scenarios":[]}
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
    Then the forge holds the chat request "is the build green?" the dashboard took and typed into the lieutenant's pane

  # Phone Message Wake 2: a request the dashboard takes itself is woken the same way
  Scenario: Phone Message Wake 2: a request the dashboard takes itself is woken the same way
    When the forge's dashboard takes the chat request "please look at the invoice card" the way its clients give it
    Then the forge holds the chat request "please look at the invoice card" the dashboard took and typed into the lieutenant's pane

  # Phone Message Wake 3: the answer to a message from the phone comes back to the room
  Scenario: Phone Message Wake 3: the answer to a message from the phone comes back to the room
    When the operator sends the message "is the build green?" into chat room Chat
    And the forge holds the chat request "is the build green?" the dashboard took and typed into the lieutenant's pane
    When the lieutenant answers the chat request "is the build green?" with "yes, the build is green"
    Then the operator decrypts the thread reply "yes, the build is green" to the chat message "is the build green?"
