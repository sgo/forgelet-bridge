package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// defaultScenarioRuns and defaultMutantRuns are what a clean up keeps of each
// kind unless it is told otherwise: a scenario run is megabytes and a mutant
// run is hundreds, so they are kept to different numbers.
const (
	defaultScenarioRuns = 5
	defaultMutantRuns   = 2
)

// runsProject is the fixture project the runs scenarios work with: a project of
// its own with a build tree, so cleaning up runs never touches the tree the
// suite is itself running from.
func (w *World) runsProject() string { return filepath.Join(w.workDir, "runs-project") }

// runsBuild is that project's build tree, where its runs live.
func (w *World) runsBuild() string { return filepath.Join(w.runsProject(), "build") }

// scenarioRunsDir is where an acceptance pass starts its scenario runs.
func (w *World) scenarioRunsDir() string {
	return filepath.Join(w.runsBuild(), "acceptance", "run")
}

// mutantRunsDir is where a mutation pass leaves its mutant runs.
func (w *World) mutantRunsDir() string {
	return filepath.Join(w.runsBuild(), "acceptance-mutation", "chat-channel-relay", "mutations")
}

// keepEnv is how many runs of each kind the fixture's clean up keeps.
func (w *World) keepEnv() []string {
	return []string{
		fmt.Sprintf("FORGELET_KEEP_SCENARIO_RUNS=%d", w.keepScenarios),
		fmt.Sprintf("FORGELET_KEEP_MUTANT_RUNS=%d", w.keepMutants),
	}
}

// theFixtureProjectHasABuildTree gives the fixture project the build tree and
// the tools a clean up runs from, the way the project's own tree has them. The
// cleaner is put where the project's script looks for it, so the script that
// runs is the one the project ships.
func theFixtureProjectHasABuildTree(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	w.keepScenarios = defaultScenarioRuns
	w.keepMutants = defaultMutantRuns
	for _, dir := range []string{w.scenarioRunsDir(), w.mutantRunsDir(), filepath.Join(w.runsBuild(), "acceptance", "bin")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	for _, file := range []string{"Makefile", filepath.Join("scripts", "clean.sh")} {
		source := filepath.Join(fixtures.ProjectRoot(), file)
		content, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("the project's %s is missing: %w", file, err)
		}
		mode := os.FileMode(0o644)
		if filepath.Ext(file) == ".sh" {
			mode = 0o755
		}
		if err := writeBinary(filepath.Join(w.runsProject(), file), content, mode); err != nil {
			return err
		}
	}
	cleaner, err := buildHelper("clean-runs", "./cmd/clean-runs")
	if err != nil {
		return err
	}
	content, err := os.ReadFile(cleaner)
	if err != nil {
		return err
	}
	return writeBinary(filepath.Join(w.runsBuild(), "acceptance", "bin", "clean-runs"), content, 0o755)
}

// writeBinary writes a file a fixture needs, making its directory.
func writeBinary(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, mode)
}

// theRunsKeepTheNewestOfEachKind sets the two limits the clean up works to.
func theRunsKeepTheNewestOfEachKind(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	scenarios, err := countOfCaptures(captures, 1)
	if err != nil {
		return err
	}
	mutants, err := countOfCaptures(captures, 2)
	if err != nil {
		return err
	}
	w.keepScenarios, w.keepMutants = scenarios, mutants
	return nil
}

// theBuildTreeCarriesRuns fills the build tree with the runs it says it has,
// oldest first, so the newest of each kind is the run a pass would be using.
func theBuildTreeCarriesRuns(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	scenarios, err := countOfCaptures(captures, 1)
	if err != nil {
		return err
	}
	mutants, err := countOfCaptures(captures, 2)
	if err != nil {
		return err
	}
	w.scenarioRuns, err = makeRuns(w.scenarioRunsDir(), "scenario", scenarios)
	if err != nil {
		return err
	}
	w.mutantRuns, err = makeRuns(w.mutantRunsDir(), "m", mutants)
	return err
}

// makeRuns writes count run directories, each newer than the last, and names
// them in the order they were made.
func makeRuns(dir, prefix string, count int) ([]string, error) {
	names := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		name := fmt.Sprintf("%s-%d", prefix, index)
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return names, err
		}
		// The run made last is the newest, which is the one a pass in flight
		// would be using.
		when := time.Now().Add(-time.Duration(count-index) * time.Hour)
		if err := os.Chtimes(path, when, when); err != nil {
			return names, err
		}
		names = append(names, name)
	}
	return names, nil
}

// theBuildTreeHoldsFewerRunsThanItKeeps fills the build tree below both limits.
func theBuildTreeHoldsFewerRunsThanItKeeps(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	var err error
	w.scenarioRuns, err = makeRuns(w.scenarioRunsDir(), "scenario", w.keepScenarios-1)
	if err != nil {
		return err
	}
	w.mutantRuns, err = makeRuns(w.mutantRunsDir(), "m", w.keepMutants-1)
	return err
}

// theBuildTreeHoldsRunDebrisBesideTheBridgeBinary fills the build tree past
// both limits and puts the binary the bridge runs from beside the debris.
func theBuildTreeHoldsRunDebrisBesideTheBridgeBinary(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	var err error
	w.scenarioRuns, err = makeRuns(w.scenarioRunsDir(), "scenario", w.keepScenarios+2)
	if err != nil {
		return err
	}
	w.mutantRuns, err = makeRuns(w.mutantRunsDir(), "m", w.keepMutants+2)
	if err != nil {
		return err
	}
	return writeBinary(filepath.Join(w.runsBuild(), "acceptance", "bin", "forgelet-bridge"), []byte(bridgeBinaryWording), 0o755)
}

// bridgeBinaryWording is what the fixture's stand-in for the bridge binary
// holds, so that a clean up that takes it is a thing the scenario can see.
const bridgeBinaryWording = "the bridge the forge is running from\n"

// countOfCaptures reads one count out of a scenario's step.
func countOfCaptures(captures []string, index int) (int, error) {
	value, err := captured(captures, index)
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("the step's %q is not a number of runs", value)
	}
	return count, nil
}
