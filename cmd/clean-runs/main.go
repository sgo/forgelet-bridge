// Command clean-runs removes the throwaway runs a build tree no longer needs,
// keeping the newest of each kind, and says what it removed.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/runs"
)

// The limits are the two kinds of run a build leaves: a scenario run is a few
// megabytes and a mutant run is hundreds, so they are kept to different
// numbers.
const (
	scenarioRunsEnv = "FORGELET_KEEP_SCENARIO_RUNS"
	mutantRunsEnv   = "FORGELET_KEEP_MUTANT_RUNS"

	defaultScenarioRuns = 5
	defaultMutantRuns   = 2
)

func main() {
	buildDir := flag.String("build", "build", "the build tree whose runs to clean up")
	flag.Parse()

	if err := run(*buildDir, os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "clean-runs:", err)
		os.Exit(1)
	}
}

// run cleans one build tree and writes what it removed to out: the report is
// the tool's contract with whoever runs it, so it is a value the caller hands
// in rather than the process's own output.
func run(buildDir string, env func(string) string, out io.Writer) error {
	limits, err := limitsFrom(env)
	if err != nil {
		return err
	}
	report, err := runs.Clean(buildDir, limits)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, report)
	return nil
}

// limitsFrom reads how many runs of each kind to keep.
func limitsFrom(env func(string) string) (runs.Limits, error) {
	scenarios, err := countFrom(env, scenarioRunsEnv, defaultScenarioRuns)
	if err != nil {
		return runs.Limits{}, err
	}
	mutants, err := countFrom(env, mutantRunsEnv, defaultMutantRuns)
	if err != nil {
		return runs.Limits{}, err
	}
	return runs.Limits{Scenarios: scenarios, Mutants: mutants}, nil
}

// countFrom reads one limit, which has to be a number of runs to keep.
func countFrom(env func(string) string, name string, fallback int) (int, error) {
	value := strings.TrimSpace(env(name))
	if value == "" {
		return fallback, nil
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 {
		return 0, fmt.Errorf("%s has to be a number of runs to keep, at least 1: %q", name, value)
	}
	return count, nil
}
