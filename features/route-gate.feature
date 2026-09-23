Feature: Route Gate

  # A card is the forge's unit of work, and the gate is what stands between a
  # proposal and one: the proposal is recorded, the card is created only when
  # the operator's own words say so, and one approval is single-use, so a second
  # card cannot ride on one word. The tool carries no policy of its own — which
  # proposals need asking is the forge's own lieutenant prompt — so it stays the
  # same tool in a forge whose gate is stricter than another's.

  Background:
    Given the fixture forge root forge-a holds the project forgelet-bridge

  # Route Gate 1: a proposal creates no card until the operator says so
  Scenario: Route Gate 1: a proposal creates no card until the operator says so
    Given the route gate has a proposal for the card refund-card in the project forgelet-bridge
    Then the forge's board has no card refund-card for the project forgelet-bridge yet
    And the gate still has the proposal for the card refund-card waiting

  # Route Gate 2: the operator's own words create the card, once
  Scenario: Route Gate 2: the operator's own words create the card, once
    Given the route gate has a proposal for the card refund-card in the project forgelet-bridge
    When the operator answers the proposal for the card refund-card with "approve"
    Then the forge's board now holds the card refund-card for the project forgelet-bridge
    When the operator answers the proposal for the card refund-card with the same words again
    Then the gate refused the second card
    And the forge's board holds exactly one card refund-card
