package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunCleansTheTreeAndSaysWhatItRemoved(t *testing.T) {
	build := t.TempDir()
	dir := filepath.Join(build, "acceptance", "run")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-time.Hour)
	for index := 0; index < 3; index++ {
		path := filepath.Join(dir, "run-"+string(rune('a'+index)))
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		when := start.Add(time.Duration(index) * time.Minute)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	env := func(name string) string {
		if name == scenarioRunsEnv {
			return "1"
		}
		return ""
	}

	if err := run(build, env, &out); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "removed 2 scenario runs") {
		t.Errorf("report = %q, want the two scenario runs named", out.String())
	}
	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 {
		t.Errorf("%d scenario runs left, want the newest one", len(left))
	}
}

func TestRunRefusesALimitThatIsNotANumberOfRuns(t *testing.T) {
	env := func(name string) string {
		if name == mutantRunsEnv {
			return "plenty"
		}
		return ""
	}

	err := run(t.TempDir(), env, &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), mutantRunsEnv) {
		t.Errorf("err = %v, want it to name the limit it could not read", err)
	}
}

func TestLimitsFallBackToHowManyRunsAForgeKeeps(t *testing.T) {
	limits, err := limitsFrom(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if limits.Scenarios != defaultScenarioRuns || limits.Mutants != defaultMutantRuns {
		t.Errorf("limits = %+v, want the defaults %d and %d", limits, defaultScenarioRuns, defaultMutantRuns)
	}
}
