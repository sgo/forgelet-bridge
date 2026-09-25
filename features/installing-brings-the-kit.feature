# mutation-stamp: sha256=fc2327abd30b7b3133fceecb3e0f025d83a839810076bd488bd3f86806af79d7
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-25T15:22:04.468605Z","feature_name":"Installing Brings The Kit","feature_path":"features/installing-brings-the-kit.feature","background_hash":"c3482943d65c6e74891e8db70a351607d417005d9f06d1e0503d20e76ef94221","implementation_hash":"sha256:b06eb840e702902aa11dc4320cbd52c393011f05f9942a67600131be6d4d6107","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Installing Brings The Kit

  # Installing the bridge for a forge brings the tools that make the forge
  # behave, the way it already brings the rules: the route gate, the idler check,
  # the stall watch and the doorbell, in the forge's own scripts, with the
  # watch's agent.
  # Each one is self-checked against the forge it was installed into - the pane,
  # the board row and the inbox it read - so a tool that reads nothing is a
  # failure on the page rather than a clean-looking silence, and the self-check
  # prints the command and the marker it looked for. The installer ships tools;
  # it never decides policy: the gate's strictness stays in the forge's own
  # lieutenant prompt, and the report says which policy it found and left alone.
  # A forge between cards is the other end of the same check: an empty board and
  # an empty inbox are things the tool *did* read, so a quiet forge installs, and
  # the self-check says what it read rather than calling the read unread. What
  # still fails an install is a tool that read nothing at all - no roles file, no
  # board, no inbox, which is a wrong path or a wrong forge rather than a quiet
  # one. A forge that is not running, or whose projects are stopped, is neither:
  # installing the tools before starting the forge is the order most people
  # choose, and such an install succeeds while saying what it could not exercise,
  # naming the projects it read and the pane path it could not prove, so a later
  # run can prove it.
  # The doorbell's self-check runs its pass, so installing the kit into a forge
  # with a request that never arrived rings it: the rings are right, because such
  # a request really was never delivered, and installing is a moment when somebody
  # is reading. That is a deliberate repair, not a surprise, so the report says
  # which requests it rang on the way in.
  #
  # A forge that has been composed and never started is the other end of the same
  # check: nothing has been started there, so there is no roles file, no board, no
  # inbox and no pane to read anywhere, and a tool that can prove nothing must say
  # so rather than failing the install. That is the shape a Forgelet forge arrives
  # in, where the kit is installed before anything is started, and it has to
  # install the way a forge between cards does. Reading nothing at all is still a
  # failure: a forge with no project to read is a wrong path or a wrong forge.

  Background:
    Given the fixture forge root forge-a carries its own lieutenant prompt
    And the fixture forge root forge-a holds the project forgelet-bridge

  # Installing Brings The Kit 1: the tools land, and each reads the forge it was installed into
  Scenario: Installing Brings The Kit 1: the tools land, and each reads the forge it was installed into
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    And the forge's board already holds the card refund-card in the project forgelet-bridge in the lane coder
    And the project forgelet-bridge of the forge root forge-a keeps a note waiting to be picked up
    When the adapter for the forge root forge-a installs the kit
    Then the installer's output says it installed the route gate, the idler check, the stall watch and the doorbell
    And the forge root forge-a carries the route gate, the idler check, the stall watch and the doorbell
    And the self-check of the idler check names the command it ran, the pane it read and the marker it looked for
    And the self-check of the idler check names the card and the mail it read
    And the installer succeeded

  # Installing Brings The Kit 2: a tool that reads nothing is a failure, not silence
  Scenario: Installing Brings The Kit 2: a tool that reads nothing is a failure, not silence
    Given the fixture forge root forge-a has no project structure
    When the adapter for the forge root forge-a installs the kit
    Then the self-check of the idler check says it read no roles, no board and no inbox
    And the installer's output says the self-check failed
    And the installer failed

  # Installing Brings The Kit 3: running it again is safe, and the policy is left alone
  Scenario: Installing Brings The Kit 3: running it again is safe, and the policy is left alone
    Given the adapter for the forge root forge-a installs the kit
    When the adapter for the forge root forge-a runs the kit install again
    Then the installer's output says the kit was already current
    And the installer changed nothing in the forge root forge-a
    And the installer's output says it left the gate policy in the lieutenant prompt alone
    And the installer succeeded

  # Installing Brings The Kit 4: a forge between cards installs, and its empty reads count as read
  Scenario: Installing Brings The Kit 4: a forge between cards installs, and its empty reads count as read
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the forge root forge-a gives the role coder a live session
    When the adapter for the forge root forge-a installs the kit
    Then the self-check of the idler check names the command it ran, the pane it read and the marker it looked for
    And the self-check of the idler check names the board and the inbox it read, and both were empty
    And the installer succeeded

  # Installing Brings The Kit 5: a stopped forge installs, and says what it could not exercise
  Scenario: Installing Brings The Kit 5: a stopped forge installs, and says what it could not exercise
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    When the adapter for the forge root forge-a installs the kit
    Then the installer's output names the projects it read and the pane it could not prove
    And the installer succeeded

  # Installing Brings The Kit 6: the doorbell's self-check rings a request that never arrived
  Scenario: Installing Brings The Kit 6: the doorbell's self-check rings a request that never arrived
    Given the project forgelet-bridge of the forge root forge-a records the role coder running codex
    And the project forgelet-bridge of the forge root forge-a is mastered by the role coder
    And the forge root forge-a gives the role coder a live session
    And the forge's dashboard already holds the chat request "is the build green?"
    When the adapter for the forge root forge-a installs the kit
    Then the master role's pane holds the chat request "is the build green?" the doorbell typed
    And the installer's output says it rang the chat request "is the build green?" that had never been delivered
    And the installer succeeded

  # Installing Brings The Kit 7: a forge that has not been started installs, and says what it could not prove
  Scenario: Installing Brings The Kit 7: a forge that has not been started installs, and says what it could not prove
    Given the fixture forge root forge-a has been composed but never started
    When the adapter for the forge root forge-a installs the kit
    Then the self-check of the route gate says the proposal store it could not read
    And the self-check of the idler check says it read the project, which has not been started, and the pane it could not prove
    And the self-check of the doorbell says the pane it could not read
    And the installer succeeded
