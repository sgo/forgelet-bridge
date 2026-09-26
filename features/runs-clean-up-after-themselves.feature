# mutation-stamp: sha256=68d673dfe6bfb56c2835ca52bd8308eede5c57f91ddc956204288e2572d5f045
# acceptance-mutation-manifest-begin
# {"version":1,"tested_at":"2026-09-26T11:50:22.687165Z","feature_name":"Runs Clean Up After Themselves","feature_path":"features/runs-clean-up-after-themselves.feature","background_hash":"e375e5a5694e3901f3ab014b2494a56b1a4dee3268bf71a717fd0456c5a9e9ed","implementation_hash":"sha256:96143236ccd575257bcedf8028a748cd249249308e212d6fbae2ea4f012f0f45","scenarios":[{"index":0,"name":"Runs Clean Up After Themselves 1: a finished run keeps the newest of each kind and removes the rest","scenario_hash":"1f108aae260728b0beb899c20011ce7ec403b3850acd13970da32abaff8840bd","mutation_count":12,"result":{"Total":12,"Killed":12,"Survived":0,"Errors":0},"tested_at":"2026-09-23T20:28:13.754383Z"}]}
# acceptance-mutation-manifest-end

Feature: Runs Clean Up After Themselves

  # Every acceptance scenario starts its own homeserver and clients under
  # <worktree>/build/acceptance/run/, and every mutant of a mutation run leaves a
  # whole run under <worktree>/build/acceptance-mutation/.../, and neither was
  # ever removed: this project's throwaway output grew into gigabytes against
  # kilobytes of code, the way nothing under a build tool's directory would. A
  # run now cleans up after itself when it finishes, keeping the newest few of
  # each kind — a scenario run is megabytes and a mutant run is hundreds, so they
  # are kept to different limits — and saying what it removed, so a failure can
  # still be looked at while the directory stops growing without limit. The
  # newest of each kind is exactly what a run still in flight is using, and is
  # never touched. And `clean` does by hand what the runs do anyway, without
  # taking the bridge binary that sits beside the debris and is running.

  Background:
    Given the fixture project has a build tree

  # Runs Clean Up After Themselves 1: a finished run keeps the newest of each kind and removes the rest
  Scenario Outline: Runs Clean Up After Themselves 1: a finished run keeps the newest of each kind and removes the rest
    Given the runs keep the newest <keep_scenarios> scenario runs and the newest <keep_mutants> mutant runs
    And the build tree carries <scenarios> scenario runs and <mutants> mutant runs
    When the runs clean up after themselves
    Then the build tree holds the newest <keep_scenarios> scenario runs and the newest <keep_mutants> mutant runs
    And the build tree still holds the scenario run that was in flight
    And the clean up says it removed <removed_scenarios> scenario runs and <removed_mutants> mutant runs

    Examples:
      | keep_scenarios | keep_mutants | scenarios | mutants | removed_scenarios | removed_mutants |
      | 4              | 3            | 7         | 6       | 3                 | 3               |
      | 5              | 1            | 8         | 4       | 3                 | 3               |

  # Runs Clean Up After Themselves 2: a run with nothing to remove says so
  Scenario: Runs Clean Up After Themselves 2: a run with nothing to remove says so
    Given the build tree holds fewer runs than it keeps
    When the runs clean up after themselves
    Then the build tree holds every run it had
    And the clean up says there was nothing to remove

  # Runs Clean Up After Themselves 3: clean does by hand what the runs do, and keeps the binary
  Scenario: Runs Clean Up After Themselves 3: clean does by hand what the runs do, and keeps the binary
    Given the build tree holds run debris beside the bridge binary
    When the project is cleaned
    Then the build tree still holds the bridge binary
    And the clean up says it removed the run debris it did not keep
    And the project's build targets include clean
