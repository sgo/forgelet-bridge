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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T22:24:28+02:00","module_hash":"61d1430711327e06d02a7bd1c4db75c94a7e640589de7ff1bedb5ffcc03b7c06","functions":[{"id":"func/main","name":"main","line":27,"end_line":35,"hash":"1e3d16e87bb8509b62ee70ab718c929316493469b1f6f9948a7859a28569b959"},{"id":"func/run","name":"run","line":40,"end_line":51,"hash":"3d4bef8efc73e49bd5a81187b29d9223541a1f4267689403cabae6189ebac8d9"},{"id":"func/limitsFrom","name":"limitsFrom","line":54,"end_line":64,"hash":"2de5ffac4388bc120157a67a05793264d70ffa7455c01b1fa24ced381a64c667"},{"id":"func/countFrom","name":"countFrom","line":67,"end_line":77,"hash":"65865a3531d44982cf95bee875f45f9a1edd3c8dd08b81d6916cefb9f57f0350"}]}
// mutate4go-manifest-end
