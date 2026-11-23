Feature: Restore the doorbell clarification clause

  # The bridge's canonical doorbell is installed over every forge's copy. Its
  # ring must therefore carry the same operator boundary as the dashboard's wake:
  # an approval is the operator's decision, and a clarification is the operator's
  # answer to give. The installer's self-check stays strict for both clauses, so
  # a future runtime sync cannot silently ship a doorbell that drops either one.

  Background:
    Given the fixture forge root forge-a carries its own lieutenant prompt
    And the fixture forge root forge-a holds the project forgelet-bridge

  Scenario Outline: each bridge-written request carries its operator clause
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's dashboard already holds the bridge-written <request_kind> request
    When the doorbell runs for the forge root forge-a
    Then the ring carries the <request_kind> operator clause

    Examples:
      | request_kind  |
      | approval      |
      | clarification |

  Scenario Outline: a doorbell missing either clause fails installation
    Given the kit's doorbell has lost its <clause> clause
    When the adapter for the forge root forge-a installs the kit
    Then the self-check of the doorbell says it could not find the <clause> clause
    And the installer's output says the self-check failed
    And the installer failed

    Examples:
      | clause |
      | gate   |
      | answer |
