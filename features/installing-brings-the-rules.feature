# mutation-stamp: sha256=8d789eecd199b24964547100623d4cee2dbf748f96d3ba6c216b3f071b7ef3ed
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-24T16:43:17.759412Z","feature_name":"Installing Brings The Rules","feature_path":"features/installing-brings-the-rules.feature","background_hash":"2c73d51e495b0977cfc742d79dc4cca67e47550e2d128e3a2fb2b0d279a8bcfa","implementation_hash":"sha256:61d4290fafa1ba2abe2dd1667b40a801659e301ef464a7d15bd26cdf43942ecc","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Installing Brings The Rules

  # The rooms the bridge carries only work because the forge's roles follow the
  # rules behind them: a role that stops while holding a card raises a
  # clarification - naming the card, what stops it, what it tried and what it
  # needs - so the operator can move it back into work, and a role with nothing
  # assigned reports NO_TASK instead of going quiet. That text has been copied
  # by hand into the forge's constitution, each project's tracked copy and the
  # pack new projects come from, which is how the copies drift. Installing the
  # bridge for a forge installs the rules instead: an idempotent step that owns
  # one marked block per subject, refreshes a block that drifted, keeps every
  # word it did not write, reports what it changed, what was already current and
  # what it left alone, and never reaches into a live project's tracked tree - a
  # project picks a new rule up through its specifier, like any other change.

  Background:
    Given the fixture forge root forge-a carries a constitution the forge wrote itself
    And the fixture forge root forge-a scaffolds new projects from the pack four-pack
    And the project forgelet-bridge of the fixture forge root forge-a is mid-card
    And the acceptance suite remembers the worktree it runs from

  # Installing Brings The Rules 1: the rules land in the constitution and the pack, and nothing else moves
  Scenario: Installing Brings The Rules 1: the rules land in the constitution and the pack, and nothing else moves
    When the adapter for the forge root forge-a installs the rules
    Then the installer's output says it changed the stopping-without-finishing rule
    And the installer's output says it left the projects of the forge root forge-a alone
    And the constitution of the forge root forge-a carries the stopping-without-finishing rule
    And the packs of the forge root forge-a carry the stopping-without-finishing rule
    And the forge root forge-a keeps the wording it wrote itself
    And the project forgelet-bridge of the forge root forge-a is the tree it was
    And the worktree the acceptance suite runs from still holds its head, its branches and its changes

  # Installing Brings The Rules 2: running it again is safe and says so
  Scenario: Installing Brings The Rules 2: running it again is safe and says so
    Given the adapter for the forge root forge-a installs the rules
    When the adapter for the forge root forge-a installs the rules again
    Then the installer's output says the stopping-without-finishing rule was already current
    And the installer changed nothing in the forge root forge-a
    And the constitution of the forge root forge-a carries the stopping-without-finishing rule

  # Installing Brings The Rules 3: a rule that drifted is refreshed
  Scenario: Installing Brings The Rules 3: a rule that drifted is refreshed
    Given the adapter for the forge root forge-a installs the rules
    And the stopping-without-finishing rule in the constitution of the forge root forge-a has gone stale
    When the adapter for the forge root forge-a installs the rules again
    Then the installer's output says it changed the stopping-without-finishing rule
    And the constitution of the forge root forge-a carries the stopping-without-finishing rule
