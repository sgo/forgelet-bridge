# mutation-stamp: sha256=82ee9f2030768df39fda0484c107bcc5a8d79c593929f85a9bf9e4d884ed727f
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-25T16:58:11.424063Z","feature_name":"Pushing The Bridge When A Card Lands","feature_path":"features/pushing-the-bridge-when-a-card-lands.feature","background_hash":"07b994a4ea48203c5fc7327b67631cc72ed486da87fb84833c66b0b898b451db","implementation_hash":"sha256:415f972838d910ef5ccbadc9c98811ab51f374efee43de5820e27665a1a8f14e","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Pushing The Bridge When A Card Lands

  # The composition builds a forge's bridge from GitHub's master, so a card
  # whose work lands somewhere else leaves the next forge built from yesterday's
  # bridge, and nothing in that forge says so. The tooling knows the one moment
  # a card's work has landed on master - the merge into the master worktree of a
  # handoff whose board row already says the card is done - and runs the
  # project's own finishing step there, echoing what it says into the pane and
  # printing HOOK_FAILED when it fails. This project's step is the push the
  # composition depends on: the code a forge downloads is the code the bridge
  # runs. A push that fails says so instead of passing quietly, a card whose work
  # is already pushed does not mind being pushed again, and a project with no
  # remote has nowhere to push and stands down rather than reporting a failure.
  # It never forces: a remote that has moved on is a failure to report, not an
  # overwrite.
  # Deploying a running bridge is not this step's business: the binary a bridge
  # is serving from still needs replacing when a card changed it, and the push
  # says nothing about that either way.

  Background:
    Given the fixture project is a checkout of the bridge

  # Pushing The Bridge When A Card Lands 1: a card's work reaches the remote when the card is done
  Scenario: Pushing The Bridge When A Card Lands 1: a card's work reaches the remote when the card is done
    Given the fixture project has a remote
    And the fixture project holds the finished card push-the-bridge-when-a-card-lands on its master
    When the finishing step runs for the card push-the-bridge-when-a-card-lands
    Then the fixture project's remote holds the card's work on its master
    And the finishing step says it pushed the card push-the-bridge-when-a-card-lands
    And the finishing step changed nothing in the fixture project's tree
    And the project ships its finishing step at swarmforge/hooks/card-complete.sh

  # Pushing The Bridge When A Card Lands 2: a push that fails says so rather than passing quietly
  Scenario: Pushing The Bridge When A Card Lands 2: a push that fails says so rather than passing quietly
    Given the fixture project has a remote
    And the fixture project holds the finished card push-the-bridge-when-a-card-lands on its master
    And the fixture project's remote has a commit of its own
    When the finishing step runs for the card push-the-bridge-when-a-card-lands
    Then the finishing step says the push failed
    And the finishing step failed
    And the fixture project's remote holds nothing of the card

  # Pushing The Bridge When A Card Lands 3: a card whose work is already pushed does not mind
  Scenario: Pushing The Bridge When A Card Lands 3: a card whose work is already pushed does not mind
    Given the fixture project has a remote
    And the fixture project holds the finished card push-the-bridge-when-a-card-lands on its master
    And the card's work has already reached the remote
    When the finishing step runs for the card push-the-bridge-when-a-card-lands again
    Then the finishing step says there was nothing new to push
    And the finishing step succeeded
    And the fixture project's remote holds the card's work on its master

  # Pushing The Bridge When A Card Lands 4: a project with no remote has nowhere to push
  Scenario: Pushing The Bridge When A Card Lands 4: a project with no remote has nowhere to push
    Given the fixture project has no remote
    And the fixture project holds the finished card push-the-bridge-when-a-card-lands on its master
    When the finishing step runs for the card push-the-bridge-when-a-card-lands
    Then the finishing step says there is no remote to push to
    And the finishing step succeeded
