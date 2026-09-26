# mutation-stamp: sha256=cae36ef2836d735b3e19e5205683d115b72f628d4c07d8d5919e9c3762c5ed5a
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-26T11:47:40.940560Z","feature_name":"Forge Schedule","feature_path":"features/forge-schedule.feature","background_hash":"f573ec4511b3c8963c49e47b4e5827332d9fcaf8fc94692a6ea2e72da2b26e47","implementation_hash":"sha256:946376a88f4a966ecbe8d5f4457b4f451e21f9abfa171f39e811c3eca35fbaff","scenarios":[]}
# acceptance-mutation-manifest-end

Feature: Forge Schedule

  # The doorbell is installed in both forges and nothing runs it: the machine's
  # only cadence is the stall watch's agent, which runs the watch's pass alone, so
  # a request lost while every role is healthy stays lost. One schedule per
  # machine now runs both passes over every forge root it serves — the watch's
  # pass over the roles, then a doorbell pass for each root. The tools stay
  # separate, because they answer different questions and their rules differ, and
  # there is no second agent: a watcher that dies quietly is exactly what the
  # watch's heartbeat exists to catch.
  #
  # The runner's only job is the cadence. It runs both passes even when one
  # fails, so a broken pass cannot take the other down; it leaves evidence that
  # it ran, so a schedule that stopped is visible rather than assumed; it names a
  # root it could not serve rather than skipping it in silence; and it runs the
  # doorbell's pass as the pass it is, so a role that is busy is left alone and
  # the pass says so.
  #
  # The cadence is also the only thing that runs whether or not a session is up,
  # so it is what cleans up after the forge. A compose or an update exercise that
  # fails leaves its scratch in the forge's tmp on purpose, "so a person can look
  # at it", and nobody ever comes back for it; the pass sweeps the forge's own tmp
  # by age - untouched since, not a name pattern, because the honest question
  # about a failed run's scratch is whether anyone is still looking at it - and
  # says what it removed, so the removal is visible rather than silent.
  #
  # The schedule's own log is the one thing age cannot judge: it is written every
  # minute, so its mtime is always now. It is kept by file instead, a day to a
  # file, with the pass dropping its own older than a week.
  #
  # The projects take the second half, through the layer's own pruner, whose rule
  # is already decided - the newest twenty scenario runs and the newest mutant run
  # per directory, "so a failure can still be looked at" - so the pass brings a
  # cadence and a home rather than a policy: once a day, gated on the date,
  # because a walk over every worktree is not a thing to do every minute. The
  # pruner is the layer's script and the pass is the kit's, so the pass calls it
  # only when it is there and names a forge it could not prune when it is not. It
  # stays out of a project's own build output: never a Maven target/ directory,
  # and never the binaries the acceptance suite runs from.

  Background:
    Given the fixture forge roots forge-a and forge-b have their dashboards running
    And the fixture forge roots forge-a and forge-b hold the project forgelet-bridge

  # Forge Schedule 1: the machine rings a lost request with nobody asking
  Scenario: Forge Schedule 1: the machine rings a lost request with nobody asking
    Given the project forgelet-bridge of the forge root forge-a is mastered by the role coder
    And the forge root forge-a gives the role coder a live session
    And the forge forge-a's dashboard already holds the chat request "is the build green?"
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the doorbell says the chat request "is the build green?" was never delivered and rung
    And the master role's pane holds the chat request "is the build green?" the doorbell typed
    And the schedule leaves evidence that it ran

  # Forge Schedule 2: a busy role is not forced, and the pass says so
  Scenario: Forge Schedule 2: a busy role is not forced, and the pass says so
    Given the project forgelet-bridge of the forge root forge-a is mastered by the role coder
    And the forge root forge-a gives the role coder a working session
    And the forge forge-a's dashboard already holds the chat request "is the build green?"
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the doorbell says the chat request "is the build green?" was not rung because the role was busy
    And the schedule leaves evidence that it ran

  # Forge Schedule 3: a pass that fails does not take the other down
  Scenario: Forge Schedule 3: a pass that fails does not take the other down
    Given the fixture forge root forge-a has no project structure
    And the project forgelet-bridge of the forge root forge-b is mastered by the role coder
    And the forge root forge-b gives the role coder a live session
    And the forge forge-b's dashboard already holds the chat request "is the build green?"
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the schedule names the forge root it could not serve
    And the doorbell says the chat request "is the build green?" was never delivered and rung
    And the schedule leaves evidence that it ran

  # Forge Schedule 4: the pass sweeps the forge's own scratch by age, and says what it removed
  Scenario: Forge Schedule 4: the pass sweeps the forge's own scratch by age, and says what it removed
    Given the forge root forge-a's tmp holds a scratch directory nobody has touched for weeks
    And the forge root forge-a's tmp holds a scratch directory from a run just now
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the schedule says it removed the scratch directory nobody has touched for weeks
    And the forge root forge-a's tmp still holds the scratch directory from a run just now

  # Forge Schedule 5: the pass keeps a week of its own logs and drops the older ones
  Scenario: Forge Schedule 5: the pass keeps a week of its own logs and drops the older ones
    Given the forge root forge-a holds the schedule's own log for today
    And the forge root forge-a holds the schedule's own log for yesterday
    And the forge root forge-a holds the schedule's own log for a day three weeks ago
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the schedule says it dropped the schedule's own log for a day three weeks ago
    And the forge root forge-a still holds the schedule's own log for today
    And the forge root forge-a still holds the schedule's own log for yesterday

  # Forge Schedule 6: the projects are pruned once a day, not once a pass
  Scenario: Forge Schedule 6: the projects are pruned once a day, not once a pass
    Given the forge root forge-a carries the layer's pruner
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the layer's pruner ran for the forge root forge-a
    When the forge schedule runs for the forge roots forge-a, forge-b again
    Then the layer's pruner ran once for the forge root forge-a

  # Forge Schedule 7: a forge with no layer pruner is named, and the pass still runs
  Scenario: Forge Schedule 7: a forge with no layer pruner is named, and the pass still runs
    Given the forge root forge-a carries no pruner
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the schedule says the projects of the forge root forge-a were not pruned because the pruner is not there
    And the schedule leaves evidence that it ran

  # Forge Schedule 8: the pass leaves a project's own build output alone
  Scenario: Forge Schedule 8: the pass leaves a project's own build output alone
    Given the forge root forge-a carries the layer's pruner
    And the project forgelet-bridge of the forge root forge-a holds target directories from a build long past, in its tree and in a worktree
    And the project forgelet-bridge of the forge root forge-a holds the binaries the acceptance suite runs from
    When the forge schedule runs for the forge roots forge-a, forge-b
    Then the project forgelet-bridge of the forge root forge-a still holds its target directories, in its tree and in a worktree
    And the project forgelet-bridge of the forge root forge-a still holds the binaries the acceptance suite runs from
