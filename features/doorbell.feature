# mutation-stamp: sha256=bb5f43eb81442729e34c4a5d9be97261cd148413418189e45ca2e31062a9cb6b
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-24T16:30:03.881782Z","feature_name":"Doorbell","feature_path":"features/doorbell.feature","background_hash":"70a6e3b997c3489e0e5ec864702cf74065b35a6c7129add56196fe28fa475b95","implementation_hash":"sha256:247c05d7507080ede31977b0e993c9d386b49ac843bf25bbb8d36dfe9cb685d9","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Doorbell

  # The operator's message to a forge can be written down and never delivered:
  # the dashboard takes it, writes its request, types it into the master role's
  # pane and answers 200 — and when the typing fails, only the dashboard's own
  # stderr says so. The bridge counts the work done, and nothing else looks at a
  # request that was never delivered, so the operator's message is lost in
  # silence. The doorbell is the repair, and it is visible rather than hoped for:
  # a pending request with no evidence it was ever delivered is rung again, once,
  # and the pass writes down what it saw and did.
  #
  # Delivery evidence is the pane's own text — the dashboard types the request's
  # id in as [<id>] — and captures are bounded, so a ledger of what has already
  # been seen and already rung is what keeps a delivered request from being rung
  # twice. It never rings into a role mid-turn, which is the case that failed: it
  # waits for a window and says so when it cannot find one.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the project forgelet-bridge of the forge root forge-a is mastered by the role coder
    And the forge's dashboard already holds the chat request "is the build green?"

  # Doorbell 1: a request with no delivery evidence is rung once, and then left alone
  Scenario: Doorbell 1: a request with no delivery evidence is rung once, and then left alone
    Given the forge root forge-a gives the role coder a live session
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" was never delivered and rung
    And the master role's pane holds the chat request "is the build green?" the doorbell typed
    When the doorbell runs for the forge root forge-a again
    Then the doorbell says the chat request "is the build green?" was already delivered and left alone

  # Doorbell 2: a request that was delivered is left alone
  Scenario: Doorbell 2: a request that was delivered is left alone
    Given the forge root forge-a gives the role coder a live session
    And the dashboard typed the chat request "is the build green?" into the master role's pane
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" was already delivered and left alone

  # Doorbell 3: a role mid-turn is not rung into, and the pass says so
  Scenario: Doorbell 3: a role mid-turn is not rung into, and the pass says so
    Given the forge root forge-a gives the role coder a working session
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" was not rung because the role was busy
