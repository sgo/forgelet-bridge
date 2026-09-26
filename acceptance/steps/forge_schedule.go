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
	return everyPathSurvived(world.(*World).targetDirs, directorySurvived,
		"a project's own target directory", "target directories to check")
}

// theProjectStillHoldsItsAcceptanceBinaries checks the binaries the acceptance
// suite runs from are still there: the pruner walks the runs beside them.
func theProjectStillHoldsItsAcceptanceBinaries(_ context.Context, world any, _ []string) error {
	return everyPathSurvived(world.(*World).binaries, fileSurvived,
		"a binary the acceptance suite runs from", "binaries to check")
}

// everyPathSurvived checks every path a scenario left is still there, each read
// the way that path is owed, and names the scenario's own gap when it left
// nothing to check.
func everyPathSurvived(paths []string, survived func(path, what string) error, what, nothing string) error {
	if len(paths) == 0 {
		return fmt.Errorf("the scenario left no %s", nothing)
	}
	for _, path := range paths {
		if err := survived(path, what); err != nil {
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-26T23:38:41+02:00","module_hash":"0ccb31306c3bb2bebbf2aae6ceed375617f8ca4b27b793393bdffeadf8328b1d","functions":[{"id":"func/theForgeScheduleRuns","name":"theForgeScheduleRuns","line":22,"end_line":41,"hash":"94baa491cb110db774148c42d2b44a7169b89402fa709835005273c439c7de8b"},{"id":"func/theForgeScheduleRunsAgain","name":"theForgeScheduleRunsAgain","line":45,"end_line":47,"hash":"fcd71a6b947ee13981ed0dcfa2260217cf16582d1593a3a37d595bb560b4a992"},{"id":"func/theScheduleLeavesEvidenceItRan","name":"theScheduleLeavesEvidenceItRan","line":51,"end_line":66,"hash":"fedf2184fb24856afc17b633450599bb179a4db1f816f453f89e41e4537feb85"},{"id":"func/theScheduleNamesTheRootItCouldNotServe","name":"theScheduleNamesTheRootItCouldNotServe","line":70,"end_line":79,"hash":"d57e1bb29d1621945567f1936a38d72bd22b14e263629e5f06e312ba5367a2cf"},{"id":"func/scheduleSays","name":"scheduleSays","line":84,"end_line":95,"hash":"90504e48083fcd67a0e31015f98e3308b5375bdc6d1cbb791645aeb9b97abcc7"},{"id":"func/theScheduleSaysItRemovedTheScratchNobodyHasTouched","name":"theScheduleSaysItRemovedTheScratchNobodyHasTouched","line":99,"end_line":102,"hash":"4d25666c86d6dda2807651271d8712a1b15b963c4967444e9eec5ebab7969054"},{"id":"func/theForgeTmpStillHoldsTheFreshScratch","name":"theForgeTmpStillHoldsTheFreshScratch","line":107,"end_line":110,"hash":"fde8cc610f2b5953348318ff4a9714f4a11f008a6024fd63eda32ed5191b63bd"},{"id":"func/theScheduleSaysItDroppedTheLogFor","name":"theScheduleSaysItDroppedTheLogFor","line":114,"end_line":117,"hash":"6c9c732548306729de9d86803e97a22f7622deb75ab803e4674b791a34257174"},{"id":"func/theForgeStillHoldsTheLogFor","name":"theForgeStillHoldsTheLogFor","line":121,"end_line":124,"hash":"c87da23a094bf36b1084ccdb79deabdc6b4a7bdc95b724641f1e2c93e9e30217"},{"id":"func/theLayerPrunerRanFor","name":"theLayerPrunerRanFor","line":128,"end_line":135,"hash":"9d59cc6cb20c5bf20da22ce5618b1c8faadf46b640f4ed40bbb194ceeeca61ba"},{"id":"func/theLayerPrunerRanOnceFor","name":"theLayerPrunerRanOnceFor","line":139,"end_line":156,"hash":"338a39bd22ed1b0251cd5a60ef1b72b27f698d8784c3ba052a1f47b450a311bb"},{"id":"func/theScheduleSaysTheProjectsWereNotPruned","name":"theScheduleSaysTheProjectsWereNotPruned","line":161,"end_line":168,"hash":"f24f246092f3e7ae9b7a8ccf3f1d72d6c58d427a262043323474d6188a7c900d"},{"id":"func/theProjectStillHoldsItsTargetDirs","name":"theProjectStillHoldsItsTargetDirs","line":173,"end_line":176,"hash":"09a7e5707ef00911a78af51670105bfb324a2cd5258a2649e10e56247f0b72d8"},{"id":"func/theProjectStillHoldsItsAcceptanceBinaries","name":"theProjectStillHoldsItsAcceptanceBinaries","line":180,"end_line":183,"hash":"30434495fd9956ee65cc20e6c1b46b23340db8c200ed08a0caae14594eed6c55"},{"id":"func/everyPathSurvived","name":"everyPathSurvived","line":188,"end_line":198,"hash":"bfeae94da637ee121fe5aba19d24ceda35ee766d133551f7fde7fbf9b4d0ab32"},{"id":"func/directorySurvived","name":"directorySurvived","line":202,"end_line":214,"hash":"0cbc2d954bd5c2bda762398bd613de6cdd5adb1fffad7a5fe40fa8a839ba2e46"},{"id":"func/fileSurvived","name":"fileSurvived","line":217,"end_line":225,"hash":"0dc30fef0f565ebdacf0dcb669409e98eb776e214da155de68a6dc2c0d830f52"}]}
// mutate4go-manifest-end
