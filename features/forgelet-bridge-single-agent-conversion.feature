Feature: Forgelet Bridge Single Agent Conversion

  # The conversion keeps the bridge's Go implementation and replaces the
  # generated Maven scaffold with the project's real gates. The stage list is
  # the executable contract, and each gate leaves evidence from this checkout.

  Background:
    Given the forgelet-bridge project is a single-agent Go project

  Scenario Outline: Each project gate runs its native Go toolchain and preserves evidence
    Then stage <stage> runs <command> and writes <log>

    Examples:
      | stage      | command                         | log                          |
      | code       | scripts/test.sh                 | build/stages/code.log        |
      | clean      | scripts/check-clean.sh          | build/stages/clean.log       |
      | architect  | scripts/check-architecture.sh   | build/stages/architect.log   |
      | harden     | scripts/check-mutation.sh       | build/stages/harden.log      |
      | clean-final| scripts/check-clean.sh          | build/stages/clean-final.log |
      | qa         | scripts/property.sh             | build/stages/qa.log          |
      | qa         | scripts/acceptance.sh           | build/stages/qa.log          |
      | docs       | scripts/check-documentation.sh  | build/stages/docs.log        |
      | deploy     | scripts/build.sh                | build/stages/deploy.log      |
      | deploy     | scripts/matrix-bridge.sh        | build/stages/deploy.log      |
