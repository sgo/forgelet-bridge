# mutation-stamp: sha256=b11d31aeaf7164a095fa84ed305731d940cb2859cd5c60aea3ed8ede6e987322
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-25T16:55:31.151030Z","feature_name":"Doorbell","feature_path":"features/doorbell.feature","background_hash":"70a6e3b997c3489e0e5ec864702cf74065b35a6c7129add56196fe28fa475b95","implementation_hash":"sha256:247c05d7507080ede31977b0e993c9d386b49ac843bf25bbb8d36dfe9cb685d9","scenarios":[]}
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
  #
  # The evidence is what the pane can still prove, not only what its visible
  # screen shows. A request delivered before the doorbell existed, or long enough
  # ago to have scrolled away, is still in the pane's scrollback, and reading the
  # screen alone would ring it a second time — the harm this rule exists to
  # prevent, on the first pass of every forge. So the pass reads back through the
  # scrollback for the id, and what it finds there is delivery; the ledger records
  # which requests were delivered and which were rung, so the next reader can tell
  # the two apart without re-reading the pane. The bound stays where the evidence
  # does: a request the pane can no longer prove is not remembered forever, and
  # ringing it is the repair working rather than a failure.
  #
  # The ledger remembers only what has been proved or rung. A request the pass
  # skipped because the role was mid-turn has been *looked at*, not delivered, and
  # it stays owed: recording a skip as seen would let a message the operator sent
  # and never arrived be written off silently, which is the failure this tool
  # exists to repair. So the three outcomes stay three — delivered, rung, and not
  # rung because the role was busy — and a later pass, once the role is free,
  # rings what the earlier one had to leave.
  #
  # The ring also carries the answering command's neighbour: the two
  # notifications the bridge writes carry a gate. An approval the operator
  # forwarded is the operator's decision, not the lieutenant's to take, and a
  # clarification an agent is blocked on is the operator's answer to give. The
  # dashboard's wake says so, and the doorbell keeps the same words in its ring
  # for the same reason it carries the answering command: a session that never
  # read its prompt, or read a stale copy in a pane that has been up for days,
  # meets the gate at the ring and nowhere else. A request that is neither says
  # nothing of the kind.

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
    Then the doorbell says the chat request "is the build green?" was already delivered from the screen and left alone
    And the doorbell's ledger says the chat request "is the build green?" was rung

  # Doorbell 2: a request that was delivered is left alone
  Scenario: Doorbell 2: a request that was delivered is left alone
    Given the forge root forge-a gives the role coder a live session
    And the dashboard typed the chat request "is the build green?" into the master role's pane
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" was already delivered from the screen and left alone

  # Doorbell 3: a role mid-turn is not rung into, and the pass says so
  Scenario: Doorbell 3: a role mid-turn is not rung into, and the pass says so
    Given the forge root forge-a gives the role coder a working session
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" was not rung because the role was busy

  # Doorbell 4: a request the screen has forgotten is still proved by the scrollback
  Scenario: Doorbell 4: a request the screen has forgotten is still proved by the scrollback
    Given the forge root forge-a gives the role coder a live session
    And the dashboard typed the chat request "is the build green?" into the master role's pane, and it has scrolled off the screen
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" was already delivered from the scrollback and left alone
    And the doorbell's ledger says the chat request "is the build green?" was delivered

  # Doorbell 5: a request skipped for a busy role is rung once the role is free
  Scenario: Doorbell 5: a request skipped for a busy role is rung once the role is free
    Given the forge root forge-a gives the role coder a working session
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" was not rung because the role was busy
    And the doorbell's ledger says the chat request "is the build green?" is still owed
    When the forge root forge-a gives the role coder a live session
    And the doorbell runs for the forge root forge-a again
    Then the doorbell says the chat request "is the build green?" was never delivered and rung
    And the master role's pane holds the chat request "is the build green?" the doorbell typed
    And the doorbell's ledger says the chat request "is the build green?" was rung

  # Doorbell 6: an approval the operator forwarded is rung with the gate clause
  Scenario: Doorbell 6: an approval the operator forwarded is rung with the gate clause
    Given the forge root forge-a gives the role coder a live session
    And the forge's dashboard already holds the chat request the bridge wrote for the approval of the card refund-card in the project forgelet-bridge
    When the doorbell runs for the forge root forge-a
    Then the master role's pane holds the chat request "Approval for refund-card in forgelet-bridge" the doorbell typed
    And the ring says the gate is the operator's, and not to approve unless the operator says to

  # Doorbell 7: a clarification is rung with the answer clause
  Scenario: Doorbell 7: a clarification is rung with the answer clause
    Given the forge root forge-a gives the role coder a live session
    And the forge's dashboard already holds the chat request the bridge wrote for the clarification of the project forgelet-bridge from the role coder
    When the doorbell runs for the forge root forge-a
    Then the master role's pane holds the chat request "Clarification for forgelet-bridge from coder" the doorbell typed
    And the ring says the answer is the operator's to give, and not to answer unless the operator says to

  # Doorbell 8: a request that carries no gate says nothing of the kind
  Scenario: Doorbell 8: a request that carries no gate says nothing of the kind
    Given the forge root forge-a gives the role coder a live session
    When the doorbell runs for the forge root forge-a
    Then the master role's pane holds the chat request "is the build green?" the doorbell typed
    And the ring says nothing about a gate
