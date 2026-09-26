package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// forgeScheduleCommand is this repository's schedule: the cadence that makes
// the watch's pass and the doorbell's, and the thing the installed agent runs.
const forgeScheduleCommand = "forge_schedule.sh"

// theForgeScheduleRuns runs this repository's schedule over the fixture forge
// roots, the way the machine's agent runs it. A pass that failed is named and
// the rest still runs, so the step keeps the words and lets the scenario read
// them rather than failing on the status.
func theForgeScheduleRuns(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	roots, err := w.rootsOf(captures[1])
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", forgeScheduleCommand),
		append([]string{"run"}, roots...)...)
	command.Dir = roots[0]
	out, _ := command.CombinedOutput()
	w.scheduleOutput = string(out)
	w.scheduleRoots = roots
	w.scheduleRuns = append(w.scheduleRuns, w.scheduleOutput)
	// The schedule's words carry every pass's, so a step about the doorbell
	// reads the same whether the doorbell ran alone or was run by the machine.
	w.doorbellOutput = w.scheduleOutput
	return nil
}

// theForgeScheduleRunsAgain runs the cadence a second time in one scenario: the
// pass is the thing that must not repeat a job it has already done today.
func theForgeScheduleRunsAgain(ctx context.Context, world any, captures []string) error {
	return theForgeScheduleRuns(ctx, world, captures)
}

// theScheduleLeavesEvidenceItRan checks the schedule's own heartbeat: a cadence
// that stopped is visible rather than assumed.
func theScheduleLeavesEvidenceItRan(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if len(w.scheduleRoots) == 0 {
		return fmt.Errorf("the schedule has not run")
	}
	heartbeat := filepath.Join(w.scheduleRoots[0], ".swarmforge", "forge-schedule.heartbeat")
	data, err := os.ReadFile(heartbeat)
	if err != nil {
		return fmt.Errorf("the schedule left no evidence it ran: %w", err)
	}
	line := strings.TrimSpace(string(data))
	if !strings.Contains(line, "roots=") {
		return fmt.Errorf("the schedule's heartbeat does not say what it ran: %q", line)
	}
	return nil
}

// theScheduleNamesTheRootItCouldNotServe checks a root the schedule could not
// serve is named rather than skipped in silence.
func theScheduleNamesTheRootItCouldNotServe(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if w.scheduleOutput == "" {
		return fmt.Errorf("the schedule has not run")
	}
	if !strings.Contains(w.scheduleOutput, "could not serve the forge root") {
		return fmt.Errorf("the schedule names no root it could not serve:\n%s", w.scheduleOutput)
	}
	return nil
}

// scheduleSays checks the pass last run said something, and named the path it
// acted on: a removal nobody can read about is the silent cleanup the schedule
// exists to avoid.
func scheduleSays(w *World, words, path string) error {
	if w.scheduleOutput == "" {
		return fmt.Errorf("the schedule has not run")
	}
	if !strings.Contains(w.scheduleOutput, words) {
		return fmt.Errorf("the schedule does not say %q:\n%s", words, w.scheduleOutput)
	}
	if path != "" && !strings.Contains(w.scheduleOutput, path) {
		return fmt.Errorf("the schedule does not name %q:\n%s", path, w.scheduleOutput)
	}
	return nil
}

// theScheduleSaysItRemovedTheScratchNobodyHasTouched checks the pass named the
// scratch it trimmed, which is the removal being visible rather than silent.
func theScheduleSaysItRemovedTheScratchNobodyHasTouched(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	return scheduleSays(w, "removed the scratch", w.staleScratch)
}

// theForgeTmpStillHoldsTheFreshScratch checks the sweep went by age and not by
// name: the scratch a run left just now is still there, under the name a compose
// gives its scratch.
func theForgeTmpStillHoldsTheFreshScratch(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	return directorySurvived(w.freshScratch, "the scratch a run left just now")
}

// theScheduleSaysItDroppedTheLogFor checks the pass named the day of its own log
// it dropped, rather than letting the file go quietly.
func theScheduleSaysItDroppedTheLogFor(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	return scheduleSays(w, "dropped the schedule's own log", w.scheduleLogs[captures[1]])
}

// theForgeStillHoldsTheLogFor checks the pass kept what it keeps: the days of
// its own log inside the bound are left where they are.
func theForgeStillHoldsTheLogFor(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	return fileSurvived(w.scheduleLogs[captures[2]], "the schedule's own log for "+captures[2])
}

// theLayerPrunerRanFor checks the pass handed the projects to the layer's own
// pruner, and said so.
func theLayerPrunerRanFor(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	return scheduleSays(w, "pruned the projects of "+root+" with the layer's pruner", "")
}

// theLayerPrunerRanOnceFor checks the cadence kept the walk to once a day: the
// second pass over the same forge names no pruner run.
func theLayerPrunerRanOnceFor(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	ran := 0
	for _, out := range w.scheduleRuns {
		if strings.Contains(out, "pruned the projects of "+root+" with the layer's pruner") {
			ran++
		}
	}
	if ran != 1 {
		return fmt.Errorf("the layer's pruner ran %d times for %s, and a walk over every worktree is once a day:\n%s",
			ran, root, strings.Join(w.scheduleRuns, "\n"))
	}
	return nil
}

// theScheduleSaysTheProjectsWereNotPruned checks a forge whose layer ships no
// pruner is named rather than passed over in silence, and that the pass still
// ran: the kit cannot carry the layer's script for it.
func theScheduleSaysTheProjectsWereNotPruned(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	return scheduleSays(w, "the projects of "+root+" were not pruned because the pruner is not there", w.prunerPath)
}

// theProjectStillHoldsItsTargetDirs checks the build output a Maven project
// keeps is still there, in its tree and in a worktree: however old it is, it is
// the project's own, and the pass does not reach it.
func theProjectStillHoldsItsTargetDirs(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if len(w.targetDirs) == 0 {
		return fmt.Errorf("the scenario left no target directories to check")
	}
	for _, dir := range w.targetDirs {
		if err := directorySurvived(dir, "a project's own target directory"); err != nil {
			return err
		}
	}
	return nil
}

// theProjectStillHoldsItsAcceptanceBinaries checks the binaries the acceptance
// suite runs from are still there: the pruner walks the runs beside them.
func theProjectStillHoldsItsAcceptanceBinaries(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if len(w.binaries) == 0 {
		return fmt.Errorf("the scenario left no binaries to check")
	}
	for _, path := range w.binaries {
		if err := fileSurvived(path, "a binary the acceptance suite runs from"); err != nil {
			return err
		}
	}
	return nil
}

// directorySurvived reports a directory the pass had to leave alone is still
// there.
func directorySurvived(path, what string) error {
	if path == "" {
		return fmt.Errorf("the scenario left no %s to check", what)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s is gone (%s): %w", what, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory any more: %s", what, path)
	}
	return nil
}

// fileSurvived reports a file the pass had to leave alone is still there.
func fileSurvived(path, what string) error {
	if path == "" {
		return fmt.Errorf("the scenario left no %s to check", what)
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%s is gone (%s): %w", what, path, err)
	}
	return nil
}
