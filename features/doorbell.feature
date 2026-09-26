# mutation-stamp: sha256=2630e8bd7b6345d396b2c5a80e95f2a54480781c24e71c0b852f28edb9be8cd2
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-26T11:47:39.872251Z","feature_name":"Doorbell","feature_path":"features/doorbell.feature","background_hash":"70a6e3b997c3489e0e5ec864702cf74065b35a6c7129add56196fe28fa475b95","implementation_hash":"sha256:247c05d7507080ede31977b0e993c9d386b49ac843bf25bbb8d36dfe9cb685d9","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Doorbell

  # The operator's message to a forge can be written down and never delivered:
  # the dashboard takes it, writes its request, types it into the master role's
  # pane and answers 200 — and when the typing fails, only the dashboard's own
  # stderr says so. The bridge counts the work done, and nothing else looks at a
  # request that was never delivered, so the operator's message is lost in
  # silence. The doorbell is the repair, and it is visible rather than hoped for:
  # a pending request with no evidence it was ever delivered is rung again, and
  # the pass writes down what it saw and did.
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
  # Delivered is not answered. The pane's text proves that words arrived, not
  # that a session read them, and the ledger's memory is forever, so a request
  # nobody answered stopped being rung at all: the operator's phone showed a
  # question the lieutenant had never seen. A request the dashboard still holds
  # as pending is unanswered by definition, and the pass holds both facts - the
  # pending request and what it knows about its delivery - so it compares them
  # and rings it again. Ringing again takes a gap and a fill rather than a bare
  # loop: a request is not due until the gap has passed, so a session that is
  # away is not battered, and once it has been rung its fill a still-unanswered
  # request is reported as exactly that rather than rung forever. Each ring says
  # which ring it is, so a pane that has seen the same alert three times knows it
  # is not new.
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

  # A ring is not delivered by being typed. The doorbell types the whole request,
  # and a request runs to lines: the body's own, the answering command, and the
  # gate when it holds one. A terminal still taking that text swallows the Enter
  # that follows it, so the whole ring sits in the composer, unsent, and the pass
  # counts it as delivered - the composer is drawn on the screen, which is where
  # the proof of delivery reads, and the first ring's submitted copy is in the
  # scrollback behind it, so every later ring is left alone as already delivered.
  # A request is then counted as delivered when no session has seen it, which is
  # the failure this tool exists to prevent. So the ring goes in as one paste,
  # which a body of many lines cannot submit on its own newlines, and the pass
  # reads what it did: an empty composer is a ring that landed, and text still in
  # the composer is a ring that did not - rung again, or reported as owed rather
  # than written down as a delivery. Text found only in the composer is not
  # evidence that anything was delivered.

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

  # Doorbell 2: a request delivered a moment ago is left alone until the gap has passed
  Scenario: Doorbell 2: a request delivered a moment ago is left alone until the gap has passed
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
    And the ring says the answer is the operator's to give, and not to answer the clarification request unless the operator says to

  # Doorbell 8: a request that carries no gate says nothing of the kind
  Scenario: Doorbell 8: a request that carries no gate says nothing of the kind
    Given the forge root forge-a gives the role coder a live session
    When the doorbell runs for the forge root forge-a
    Then the master role's pane holds the chat request "is the build green?" the doorbell typed
    And the ring carries neither clause

  # Doorbell 9: a request nobody answered is rung again, and the ring says which ring it is
  Scenario: Doorbell 9: a request nobody answered is rung again, and the ring says which ring it is
    Given the forge root forge-a gives the role coder a live session
    And the dashboard typed the chat request "is the build green?" into the master role's pane
    And the doorbell has already rung the chat request "is the build green?" once, and nobody answered it
    And the chat request "is the build green?" has waited unanswered past the doorbell's gap
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" was never answered and was rung again
    And the ring says it is the doorbell's second ring of the chat request "is the build green?"
    And the doorbell's ledger says the chat request "is the build green?" was rung

  # Doorbell 10: a request that has been rung its fill is reported as still unanswered
  Scenario: Doorbell 10: a request that has been rung its fill is reported as still unanswered
    Given the forge root forge-a gives the role coder a live session
    And the dashboard typed the chat request "is the build green?" into the master role's pane
    And the chat request "is the build green?" has been rung its fill and is still unanswered
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" is still unanswered
    And the doorbell did not ring the chat request "is the build green?" again

  # Doorbell 11: a request that has been answered is left alone
  Scenario: Doorbell 11: a request that has been answered is left alone
    Given the forge root forge-a gives the role coder a live session
    And the dashboard answered the chat request "is the build green?"
    When the doorbell runs for the forge root forge-a
    Then the doorbell says nothing is pending

  # Doorbell 12: a request the dashboard delivered and nobody answered is rung once the gap has passed
  Scenario: Doorbell 12: a request the dashboard delivered and nobody answered is rung once the gap has passed
    Given the forge root forge-a gives the role coder a live session
    And the dashboard typed the chat request "is the build green?" into the master role's pane
    And the chat request "is the build green?" has waited unanswered past the doorbell's gap
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "is the build green?" was never answered and rung into the master role's pane
    And the master role's pane holds the chat request "is the build green?" the doorbell typed

  # Doorbell 13: a ring whose text runs to lines lands as one submitted turn, and the composer is empty
  Scenario: Doorbell 13: a ring whose text runs to lines lands as one submitted turn, and the composer is empty
    Given the forge root forge-a gives the role coder a live session
    And the forge's dashboard already holds the chat request the bridge wrote for the clarification of the project forgelet-bridge from the role coder
    When the doorbell runs for the forge root forge-a
    Then the master role's pane holds the chat request "Clarification for forgelet-bridge from coder" as one submitted turn
    And the master role's pane holds nothing in the composer

  # Doorbell 14: a ring whose Enter is lost leaves the request owed, and says so
  Scenario: Doorbell 14: a ring whose Enter is lost leaves the request owed, and says so
    Given the forge root forge-a gives the role coder a live session that loses the Enter the doorbell sends
    And the forge's dashboard already holds the chat request the bridge wrote for the clarification of the project forgelet-bridge from the role coder
    When the doorbell runs for the forge root forge-a
    Then the doorbell says the chat request "Clarification for forgelet-bridge from coder" was rung and the ring did not land
    When the doorbell runs for the forge root forge-a again
    Then the doorbell's ledger says the chat request "Clarification for forgelet-bridge from coder" is still owed
