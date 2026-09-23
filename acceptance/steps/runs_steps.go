package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// runsCleanUpAfterThemselves runs the project's own clean up against the
// fixture build tree, the way the acceptance and mutation passes run it when
// they finish.
func runsCleanUpAfterThemselves(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return w.cleanRuns(ctx, filepath.Join(w.runsProject(), "scripts", "clean.sh"), w.runsBuild())
}

// theProjectIsCleaned runs the project's clean target by hand, the way a person
// does, against the fixture's build tree.
func theProjectIsCleaned(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return w.cleanRuns(ctx, "make", "clean", "BUILD_ROOT="+w.runsBuild())
}

// cleanRuns runs one clean up in the fixture project and keeps what it said.
func (w *World) cleanRuns(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = w.runsProject()
	command.Env = append(os.Environ(), w.keepEnv()...)
	out, err := command.CombinedOutput()
	w.runsOutput = string(out)
	if err != nil {
		return fmt.Errorf("the clean up did not finish: %v\n%s", err, w.runsOutput)
	}
	return nil
}

// theBuildTreeHoldsTheNewestRuns checks the survivors are exactly the newest of
// each kind, which is what a pass in flight would be using.
func theBuildTreeHoldsTheNewestRuns(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	scenarios, err := countOfCaptures(captures, 1)
	if err != nil {
		return err
	}
	mutants, err := countOfCaptures(captures, 2)
	if err != nil {
		return err
	}
	if err := w.runsAre(w.scenarioRunsDir(), w.scenarioRuns, scenarios, "scenario runs"); err != nil {
		return err
	}
	return w.runsAre(w.mutantRunsDir(), w.mutantRuns, mutants, "mutant runs")
}

// runsAre checks one kind of run survived to the newest keep of them.
func (w *World) runsAre(dir string, made []string, keep int, kind string) error {
	left, err := dirNames(dir)
	if err != nil {
		return err
	}
	want := made
	if keep < len(made) {
		want = made[len(made)-keep:]
	}
	if strings.Join(left, ",") != strings.Join(want, ",") {
		return fmt.Errorf("the build tree holds %v, want the newest %d %s (%v)", left, keep, kind, want)
	}
	return nil
}

// theBuildTreeStillHoldsTheRunInFlight checks the newest scenario run is still
// there: a clean up never touches a pass that is using one.
func theBuildTreeStillHoldsTheRunInFlight(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if len(w.scenarioRuns) == 0 {
		return fmt.Errorf("the scenario made no scenario run to keep in flight")
	}
	inFlight := w.scenarioRuns[len(w.scenarioRuns)-1]
	if _, err := os.Stat(filepath.Join(w.scenarioRunsDir(), inFlight)); err != nil {
		return fmt.Errorf("the clean up took the scenario run in flight (%s): %w", inFlight, err)
	}
	return nil
}

// theBuildTreeHoldsEveryRunItHad checks a clean up with nothing to do left the
// runs where they were.
func theBuildTreeHoldsEveryRunItHad(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if err := w.runsAre(w.scenarioRunsDir(), w.scenarioRuns, len(w.scenarioRuns), "scenario runs"); err != nil {
		return err
	}
	return w.runsAre(w.mutantRunsDir(), w.mutantRuns, len(w.mutantRuns), "mutant runs")
}

// theCleanUpSaysItRemoved checks the clean up said how many of each kind went.
func theCleanUpSaysItRemoved(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	scenarios, err := countOfCaptures(captures, 1)
	if err != nil {
		return err
	}
	mutants, err := countOfCaptures(captures, 2)
	if err != nil {
		return err
	}
	want := fmt.Sprintf("removed %d scenario runs and %d mutant runs", scenarios, mutants)
	if !strings.Contains(w.runsOutput, want) {
		return fmt.Errorf("the clean up does not say %q:\n%s", want, w.runsOutput)
	}
	return nil
}

// theCleanUpSaysThereWasNothingToRemove checks a clean up with nothing to do
// says so rather than going quiet.
func theCleanUpSaysThereWasNothingToRemove(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if !strings.Contains(w.runsOutput, "there was nothing to remove") {
		return fmt.Errorf("the clean up does not say there was nothing to remove:\n%s", w.runsOutput)
	}
	return nil
}

// theBuildTreeStillHoldsTheBridgeBinary checks a clean target leaves the binary
// beside the debris where it was, whether it is running or not.
func theBuildTreeStillHoldsTheBridgeBinary(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	path := filepath.Join(w.runsBuild(), "acceptance", "bin", "forgelet-bridge")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("the clean target took the bridge binary beside the debris: %w", err)
	}
	if string(data) != bridgeBinaryWording {
		return fmt.Errorf("the clean target rewrote the bridge binary: %q", data)
	}
	return nil
}

// theCleanUpSaysItRemovedRunDebris checks the clean target reported what it took
// away rather than doing it silently.
func theCleanUpSaysItRemovedRunDebris(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if !strings.Contains(w.runsOutput, "removed ") || !strings.Contains(w.runsOutput, "run") {
		return fmt.Errorf("the clean up does not say what run debris it removed:\n%s", w.runsOutput)
	}
	return nil
}

// theProjectsBuildTargetsIncludeClean checks the project's own build targets
// offer a clean, so a person can do by hand what the passes do anyway.
func theProjectsBuildTargetsIncludeClean(_ context.Context, _ any, _ []string) error {
	data, err := os.ReadFile(filepath.Join(fixtures.ProjectRoot(), "Makefile"))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "clean:") {
			return nil
		}
	}
	return fmt.Errorf("the project's build targets have no clean:\n%s", data)
}

// dirNames lists the directories in one place, in name order.
func dirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
