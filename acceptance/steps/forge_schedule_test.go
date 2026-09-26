package steps

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// scheduleCase is the fixture the schedule's own scenarios work with: the two
// forge roots the feature's background declares, each carrying the project the
// feature starts with, in a work directory of its own.
func scheduleCase(t *testing.T) *World {
	t.Helper()
	w := newWorld()
	w.workDir = t.TempDir()
	if err := forgeRootsHoldProject(context.Background(), w, []string{"", "forge-a, forge-b", "forgelet-bridge"}); err != nil {
		t.Fatalf("declare the fixture forge roots: %v", err)
	}
	return w
}

// scheduleRoot is the root one fixture forge name stands for, which every test
// here reads a fixture path against.
func scheduleRoot(t *testing.T, w *World, name string) string {
	t.Helper()
	root, err := w.forgeRootOf(name)
	if err != nil {
		t.Fatalf("read the root of %s: %v", name, err)
	}
	return root
}

// TestTheScratchFixtureLeavesBothAges pins what the sweep's scenarios leave in
// a forge's own tmp: a scratch nobody has touched for weeks, under a name that
// says nothing about what left it, and one a run left just now, under the name a
// compose gives its scratch - the two things the age rule has to tell apart.
func TestTheScratchFixtureLeavesBothAges(t *testing.T) {
	w := scheduleCase(t)
	ctx := context.Background()
	root := scheduleRoot(t, w, "forge-a")

	if err := forgeTmpHoldsAScratchNobodyHasTouched(ctx, w, []string{"", "forge-a"}); err != nil {
		t.Fatalf("leave the scratch nobody has touched: %v", err)
	}
	if err := forgeTmpHoldsAFreshScratch(ctx, w, []string{"", "forge-a"}); err != nil {
		t.Fatalf("leave the scratch a run left just now: %v", err)
	}

	if want := filepath.Join(root, "tmp", staleScratch); w.staleScratch != want {
		t.Errorf("the scratch nobody has touched is at %s, want %s", w.staleScratch, want)
	}
	if want := filepath.Join(root, "tmp", freshScratch); w.freshScratch != want {
		t.Errorf("the scratch a run left just now is at %s, want %s", w.freshScratch, want)
	}
	for path, fresh := range map[string]bool{w.staleScratch: false, w.freshScratch: true} {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("the scratch %s is not a directory: %v", path, err)
		}
		if _, err := os.Stat(filepath.Join(path, "inner", "work.txt")); err != nil {
			t.Errorf("the scratch %s holds no work inside it: %v", path, err)
		}
		age := time.Since(info.ModTime())
		if fresh && age > time.Hour {
			t.Errorf("the scratch a run left just now reads as %s old: %s", age, path)
		}
		if !fresh && age < 7*24*time.Hour {
			t.Errorf("the scratch nobody has touched reads as only %s old: %s", age, path)
		}
	}
}

// TestTheScheduleLogFixtureLeavesEachDayTheScenarioNames pins the days the log's
// scenario leaves behind: one file per named day, at the name the pass looks for
// - named in UTC, the way the pass names its own days - and a day the step does
// not know is named rather than guessed at.
func TestTheScheduleLogFixtureLeavesEachDayTheScenarioNames(t *testing.T) {
	w := scheduleCase(t)
	ctx := context.Background()
	root := scheduleRoot(t, w, "forge-a")

	days := map[string]time.Duration{
		"today":                 0,
		"yesterday":             24 * time.Hour,
		"a day three weeks ago": 21 * 24 * time.Hour,
	}
	for named, ago := range days {
		if err := forgeHoldsTheScheduleLogFor(ctx, w, []string{"", "forge-a", named}); err != nil {
			t.Fatalf("leave the log for %s: %v", named, err)
		}
		if want := scheduleLogPath(root, ago); w.scheduleLogs[named] != want {
			t.Errorf("the log for %s is at %s, want %s", named, w.scheduleLogs[named], want)
		}
		if _, err := os.Stat(w.scheduleLogs[named]); err != nil {
			t.Errorf("the log for %s is not there: %v", named, err)
		}
	}
	if err := forgeHoldsTheScheduleLogFor(ctx, w, []string{"", "forge-a", "the day after tomorrow"}); err == nil {
		t.Error("the step guessed at a day it does not know, and it has to name it instead")
	}
}

// TestThePrunerFixturePutsTheShippedScriptWhereThePassLooks pins both halves of
// the layer's pruner: the forge that carries one gets the script the layer ships,
// executable, at the path the pass looks for rather than a stand-in; and the
// forge whose layer ships none has the path taken away.
func TestThePrunerFixturePutsTheShippedScriptWhereThePassLooks(t *testing.T) {
	w := scheduleCase(t)
	ctx := context.Background()
	root := scheduleRoot(t, w, "forge-a")

	if err := forgeCarriesTheLayerPruner(ctx, w, []string{"", "forge-a"}); err != nil {
		t.Fatalf("put the layer's pruner where the pass looks: %v", err)
	}
	if want := filepath.Join(root, "swarmforge", "scripts", prunerName); w.prunerPath != want {
		t.Errorf("the pruner is at %s, want %s", w.prunerPath, want)
	}
	info, err := os.Stat(w.prunerPath)
	if err != nil {
		t.Fatalf("the pruner is not where the pass looks: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("the pruner is not an executable file: %s", info.Mode())
	}
	shipped, err := os.ReadFile(filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", prunerName))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(w.prunerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(fixture) != string(shipped) {
		t.Error("the fixture put a stand-in where the pass looks, and the scenario has to run the layer's own script")
	}

	if err := forgeCarriesNoPruner(ctx, w, []string{"", "forge-b"}); err != nil {
		t.Fatalf("take the pruner away: %v", err)
	}
	if _, err := os.Stat(w.prunerPath); !os.IsNotExist(err) {
		t.Errorf("the pruner is still where the pass looks: %v", err)
	}
}

// TestTheBuildOutputFixtureLeavesWhatThePassMustNotTake pins the project's own
// build output a scenario leaves: a target directory in the project's tree and in
// one of its worktrees, built long ago and owned by the project however old it
// is, and the binaries the acceptance suite runs from.
func TestTheBuildOutputFixtureLeavesWhatThePassMustNotTake(t *testing.T) {
	w := scheduleCase(t)
	ctx := context.Background()
	root := scheduleRoot(t, w, "forge-a")
	project := filepath.Join(root, "projects", "forgelet-bridge")

	if err := projectHoldsTargetDirs(ctx, w, []string{"", "forgelet-bridge", "forge-a"}); err != nil {
		t.Fatalf("leave the target directories: %v", err)
	}
	want := []string{
		filepath.Join(project, "target"),
		filepath.Join(project, ".worktrees", "coder", "target"),
	}
	if !reflect.DeepEqual(w.targetDirs, want) {
		t.Errorf("the fixture kept %v, want %v", w.targetDirs, want)
	}
	for _, target := range want {
		info, err := os.Stat(target)
		if err != nil || !info.IsDir() {
			t.Fatalf("the target directory %s is not there: %v", target, err)
		}
		if _, err := os.Stat(filepath.Join(target, "classes", "Main.class")); err != nil {
			t.Errorf("the target directory %s holds no build output: %v", target, err)
		}
		if age := time.Since(info.ModTime()); age < 7*24*time.Hour {
			t.Errorf("the target directory %s reads as only %s old, and a build long past is what it stands for", target, age)
		}
	}

	if err := projectHoldsAcceptanceBinaries(ctx, w, []string{"", "forgelet-bridge", "forge-a"}); err != nil {
		t.Fatalf("leave the binaries the suite runs from: %v", err)
	}
	wantBinaries := []string{
		filepath.Join(project, "build", "acceptance", "bin", "forgelet-bridge"),
		filepath.Join(project, "build", "acceptance", "bin", "acceptance-entrypoint-generator"),
	}
	if !reflect.DeepEqual(w.binaries, wantBinaries) {
		t.Errorf("the fixture kept %v, want %v", w.binaries, wantBinaries)
	}
	for _, binary := range wantBinaries {
		if _, err := os.Stat(binary); err != nil {
			t.Errorf("the binary %s is not there: %v", binary, err)
		}
	}
}

// scheduleThenSteps is the handlers a scenario's "Then"s run, each one read
// against the fixture its scenario left and the words the pass said.
func scheduleThenSteps(ctx context.Context, w *World) map[string]func() error {
	return map[string]func() error{
		"the scratch it removed":    func() error { return theScheduleSaysItRemovedTheScratchNobodyHasTouched(ctx, w, nil) },
		"the scratch a run left":    func() error { return theForgeTmpStillHoldsTheFreshScratch(ctx, w, nil) },
		"the log it dropped":        func() error { return theScheduleSaysItDroppedTheLogFor(ctx, w, []string{"", "a day three weeks ago"}) },
		"the log it keeps":          func() error { return theForgeStillHoldsTheLogFor(ctx, w, []string{"", "forge-a", "today"}) },
		"the pruner's run":          func() error { return theLayerPrunerRanFor(ctx, w, []string{"", "forge-a"}) },
		"the pruner's once a day":   func() error { return theLayerPrunerRanOnceFor(ctx, w, []string{"", "forge-a"}) },
		"the forge it cannot prune": func() error { return theScheduleSaysTheProjectsWereNotPruned(ctx, w, []string{"", "forge-a"}) },
		"the project's target dirs": func() error { return theProjectStillHoldsItsTargetDirs(ctx, w, nil) },
		"the suite's own binaries":  func() error { return theProjectStillHoldsItsAcceptanceBinaries(ctx, w, nil) },
		"the root it could not serve": func() error {
			return theScheduleNamesTheRootItCouldNotServe(ctx, w, nil)
		},
		"the evidence it ran": func() error { return theScheduleLeavesEvidenceItRan(ctx, w, nil) },
	}
}

// TestTheScheduleThenStepsFindWhatThePassSaidAndLeaves pins the handlers the
// scenarios read the pass through: every one of them finds the removal, the kept
// thing, the pruner's run and the root it could not serve in the words the pass
// said and the fixture the scenario left, rather than in a second reading of the
// same facts.
func TestTheScheduleThenStepsFindWhatThePassSaidAndLeaves(t *testing.T) {
	w := scheduleCase(t)
	ctx := context.Background()
	root := scheduleRoot(t, w, "forge-a")

	if err := forgeTmpHoldsAScratchNobodyHasTouched(ctx, w, []string{"", "forge-a"}); err != nil {
		t.Fatal(err)
	}
	if err := forgeTmpHoldsAFreshScratch(ctx, w, []string{"", "forge-a"}); err != nil {
		t.Fatal(err)
	}
	for _, day := range []string{"today", "a day three weeks ago"} {
		if err := forgeHoldsTheScheduleLogFor(ctx, w, []string{"", "forge-a", day}); err != nil {
			t.Fatal(err)
		}
	}
	if err := forgeCarriesTheLayerPruner(ctx, w, []string{"", "forge-a"}); err != nil {
		t.Fatal(err)
	}
	if err := projectHoldsTargetDirs(ctx, w, []string{"", "forgelet-bridge", "forge-a"}); err != nil {
		t.Fatal(err)
	}
	if err := projectHoldsAcceptanceBinaries(ctx, w, []string{"", "forgelet-bridge", "forge-a"}); err != nil {
		t.Fatal(err)
	}

	w.scheduleOutput = strings.Join([]string{
		"removed the scratch " + w.staleScratch + ", untouched for 21 days",
		"dropped the schedule's own log " + w.scheduleLogs["a day three weeks ago"] + ", older than 7 days",
		"pruned the projects of " + root + " with the layer's pruner: pruned 0 directories, freed 0 MB",
		"the projects of " + root + " were not pruned because the pruner is not there (" + w.prunerPath + ")",
		"could not serve the forge root " + root + ": it holds no project with a roles file",
	}, "\n")
	// The pass ran over the root the scenario named, which is what the evidence it
	// left is read against.
	w.scheduleRoots = []string{root}
	// The once-a-day reading counts what every pass said, so the pass has to have
	// said it in a pass rather than only in the last reading of one.
	w.scheduleRuns = append(w.scheduleRuns, w.scheduleOutput)
	if err := writeFile(filepath.Join(root, ".swarmforge", "forge-schedule.heartbeat"),
		"2026-09-26T21:00:43Z roots=1 failed=0\n"); err != nil {
		t.Fatal(err)
	}

	for name, then := range scheduleThenSteps(ctx, w) {
		if err := then(); err != nil {
			t.Errorf("the step for %s reported %v, and the pass said what it reads", name, err)
		}
	}
}

// TestTheScheduleThenStepsRefuseSilenceAndAWrongPath pins the other half every
// one of those readings needs: a step that cannot find the words, or finds them
// and the path is gone, fails rather than passing on a fixture that says nothing.
func TestTheScheduleThenStepsRefuseSilenceAndAWrongPath(t *testing.T) {
	w := scheduleCase(t)
	ctx := context.Background()

	// Silence is not a pass that cut nothing away: every reading of the pass's own
	// words has to say so.
	for name, then := range scheduleThenSteps(ctx, w) {
		switch name {
		case "the pruner's once a day", "the root it could not serve", "the evidence it ran":
			continue
		}
		if err := then(); err == nil {
			t.Errorf("the step for %s passed on a pass that said nothing", name)
		}
	}

	// The words are there and the thing is not: a removal the pass named but did
	// not make, and a kept path that has gone, both fail.
	root := scheduleRoot(t, w, "forge-a")
	w.staleScratch = filepath.Join(root, "tmp", "a-scratch-nothing-removed")
	w.freshScratch = filepath.Join(root, "tmp", "a-scratch-that-went")
	w.scheduleLogs = map[string]string{
		"today":                 filepath.Join(root, ".swarmforge", "a-log-that-went.log"),
		"a day three weeks ago": filepath.Join(root, ".swarmforge", "a-log-the-pass-named.log"),
	}
	w.targetDirs = []string{filepath.Join(root, "projects", "forgelet-bridge", "target")}
	w.binaries = []string{filepath.Join(root, "projects", "forgelet-bridge", "build", "acceptance", "bin", "forgelet-bridge")}
	w.scheduleOutput = strings.Join([]string{
		"removed the scratch " + w.staleScratch,
		"dropped the schedule's own log " + w.scheduleLogs["a day three weeks ago"],
	}, "\n")

	for _, name := range []string{"the scratch a run left", "the log it keeps", "the project's target dirs", "the suite's own binaries"} {
		if err := scheduleThenSteps(ctx, w)[name](); err == nil {
			t.Errorf("the step for %s passed on a path that is gone", name)
		}
	}
}

// TestTheSurvivalHelpersReadAPathOnce pins the three readings those steps share:
// a directory and a file that are still there pass, each says what it read when
// the path is gone or is the other kind, and a scenario that left nothing to
// check is a gap rather than a pass.
func TestTheSurvivalHelpersReadAPathOnce(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a-file")
	if err := os.WriteFile(file, []byte("kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := directorySurvived(dir, "the directory"); err != nil {
		t.Errorf("directorySurvived = %v, want a directory that is still there to pass", err)
	}
	if err := fileSurvived(file, "the file"); err != nil {
		t.Errorf("fileSurvived = %v, want a file that is still there to pass", err)
	}
	if err := directorySurvived(file, "the file"); err == nil {
		t.Error("directorySurvived passed on a file")
	}
	if err := fileSurvived(filepath.Join(dir, "gone"), "the gone file"); err == nil {
		t.Error("fileSurvived passed on a path that is gone")
	}
	if err := everyPathSurvived(nil, directorySurvived, "a directory", "directories to check"); err == nil {
		t.Error("everyPathSurvived passed on a scenario that left nothing to check")
	}
	if err := everyPathSurvived([]string{dir}, directorySurvived, "a directory", "directories to check"); err != nil {
		t.Errorf("everyPathSurvived = %v, want the path it was given to be checked", err)
	}
}

// TestTheScheduleStepRunsTheMachinesOwnPass pins the step the scenarios run
// first: it runs this repository's schedule over the fixture roots, keeps what
// the pass said with what every earlier pass said, leaves the doorbell's steps
// reading those words, and hands the same roots to the run-again step. A root
// with no project behind it is named rather than skipping the run.
func TestTheScheduleStepRunsTheMachinesOwnPass(t *testing.T) {
	w := scheduleCase(t)
	ctx := context.Background()
	root := scheduleRoot(t, w, "forge-a")

	if err := theForgeScheduleRuns(ctx, w, []string{"", "forge-a"}); err != nil {
		t.Fatalf("run the schedule over the fixture root: %v", err)
	}
	if len(w.scheduleRoots) != 1 || w.scheduleRoots[0] != root {
		t.Errorf("the pass ran over %v, want the one root %s", w.scheduleRoots, root)
	}
	if !strings.Contains(w.scheduleOutput, "the forge schedule ran for 1 forges") {
		t.Errorf("the pass did not say it ran:\n%s", w.scheduleOutput)
	}
	if w.doorbellOutput != w.scheduleOutput {
		t.Error("the doorbell's steps do not read the words the pass said")
	}
	if len(w.scheduleRuns) != 1 {
		t.Errorf("the pass is remembered %d times, want once", len(w.scheduleRuns))
	}
	if err := theScheduleLeavesEvidenceItRan(ctx, w, nil); err != nil {
		t.Errorf("the pass left no evidence it ran: %v", err)
	}
	if err := theScheduleNamesTheRootItCouldNotServe(ctx, w, nil); err == nil {
		t.Error("the step named a root the pass could not serve, and it served this one")
	}

	if err := theForgeScheduleRunsAgain(ctx, w, []string{"", "forge-a"}); err != nil {
		t.Fatalf("run the schedule again: %v", err)
	}
	if len(w.scheduleRuns) != 2 {
		t.Errorf("the pass is remembered %d times over two runs, want twice", len(w.scheduleRuns))
	}

	// A root the pass cannot serve is a root with no project behind it, which is
	// what the scenario's own "no project structure" leaves.
	if err := theForgeHasNoProjectStructure(ctx, w, []string{"", "forge-b"}); err != nil {
		t.Fatalf("take the project structure away: %v", err)
	}
	if err := theForgeScheduleRuns(ctx, w, []string{"", "forge-b"}); err != nil {
		t.Fatalf("run the schedule over a root with no project behind it: %v", err)
	}
	if err := theScheduleNamesTheRootItCouldNotServe(ctx, w, nil); err != nil {
		t.Errorf("the pass named no root it could not serve: %v\n%s", err, w.scheduleOutput)
	}

	// The step is given roots, and a step that names none is the step's own gap
	// rather than a pass over nothing.
	if err := theForgeScheduleRuns(ctx, w, []string{"", ""}); err == nil {
		t.Error("the step ran over no roots at all")
	}
}
