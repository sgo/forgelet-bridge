package runs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureRuns writes count run directories into dir, oldest first, and returns
// their names in that order.
func fixtureRuns(t *testing.T, dir string, count int) []string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, count)
	start := time.Now().Add(-time.Duration(count) * time.Hour)
	for index := 0; index < count; index++ {
		name := "run-" + string(rune('a'+index))
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		when := start.Add(time.Duration(index) * time.Hour)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

// survivors lists what is left in dir, in name order.
func survivors(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestCleanKeepsTheNewestOfEachKindAndSaysWhatWent(t *testing.T) {
	build := t.TempDir()
	scenarios := fixtureRuns(t, filepath.Join(build, "acceptance", "run"), 7)
	mutants := fixtureRuns(t, filepath.Join(build, "acceptance-mutation", "chat-channel-relay", "mutations"), 6)

	report, err := Clean(build, Limits{Scenarios: 4, Mutants: 3})
	if err != nil {
		t.Fatal(err)
	}

	if report.Removed() != 6 || !strings.Contains(report.String(), "removed 3 scenario runs and 3 mutant runs") {
		t.Errorf("report = %q, want the six runs it removed named", report)
	}
	keptScenarios := survivors(t, filepath.Join(build, "acceptance", "run"))
	if len(keptScenarios) != 4 {
		t.Fatalf("scenario runs left = %v, want the newest four", keptScenarios)
	}
	// The newest of each kind is what a run in flight is using, so it has to
	// be exactly the ones that stayed.
	for _, wanted := range scenarios[3:] {
		if !contains(keptScenarios, wanted) {
			t.Errorf("%s was removed, and it is one of the newest scenario runs", wanted)
		}
	}
	keptMutants := survivors(t, filepath.Join(build, "acceptance-mutation", "chat-channel-relay", "mutations"))
	if len(keptMutants) != 3 {
		t.Fatalf("mutant runs left = %v, want the newest three", keptMutants)
	}
	for _, wanted := range mutants[3:] {
		if !contains(keptMutants, wanted) {
			t.Errorf("%s was removed, and it is one of the newest mutant runs", wanted)
		}
	}
}

func TestCleanSaysWhenThereWasNothingToRemove(t *testing.T) {
	build := t.TempDir()
	fixtureRuns(t, filepath.Join(build, "acceptance", "run"), 2)

	report, err := Clean(build, Limits{Scenarios: 4, Mutants: 3})
	if err != nil {
		t.Fatal(err)
	}

	if report.Removed() != 0 || report.String() != "there was nothing to remove" {
		t.Errorf("report = %q, want it to say there was nothing to remove", report)
	}
	if left := survivors(t, filepath.Join(build, "acceptance", "run")); len(left) != 2 {
		t.Errorf("scenario runs left = %v, want the two it had", left)
	}
}

func TestCleanNeverTouchesTheBinaryBesideTheRuns(t *testing.T) {
	build := t.TempDir()
	fixtureRuns(t, filepath.Join(build, "acceptance", "run"), 6)
	bin := filepath.Join(build, "acceptance", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	bridge := filepath.Join(bin, "forgelet-bridge")
	if err := os.WriteFile(bridge, []byte("the bridge the forge runs from\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Clean(build, Limits{Scenarios: 2, Mutants: 2}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(bridge)
	if err != nil {
		t.Fatalf("the bridge binary beside the runs went with them: %v", err)
	}
	if string(data) != "the bridge the forge runs from\n" {
		t.Errorf("the bridge binary was rewritten: %q", data)
	}
}

func TestCleanFindsMutantRunsWhereverThePassRootedThem(t *testing.T) {
	build := t.TempDir()
	fixtureRuns(t, filepath.Join(build, "acceptance-mutation", "mutations"), 5)

	report, err := Clean(build, Limits{Scenarios: 2, Mutants: 1})
	if err != nil {
		t.Fatal(err)
	}

	if report.Mutants != 4 {
		t.Errorf("removed %d mutant runs, want 4", report.Mutants)
	}
	if left := survivors(t, filepath.Join(build, "acceptance-mutation", "mutations")); len(left) != 1 {
		t.Errorf("mutant runs left = %v, want the newest one", left)
	}
}

func TestCleanKeepsTheNewestRunEvenWhenItIsAskedToKeepNone(t *testing.T) {
	build := t.TempDir()
	scenarios := fixtureRuns(t, filepath.Join(build, "acceptance", "run"), 3)

	if _, err := Clean(build, Limits{Scenarios: 0, Mutants: 0}); err != nil {
		t.Fatal(err)
	}

	left := survivors(t, filepath.Join(build, "acceptance", "run"))
	if len(left) != 1 || left[0] != scenarios[2] {
		t.Errorf("scenario runs left = %v, want the newest one (%s): it is what a run in flight is using", left, scenarios[2])
	}
}

func TestCleanKeepsTheNewestMutantRunEvenWhenItIsAskedToKeepNone(t *testing.T) {
	build := t.TempDir()
	dir := filepath.Join(build, "acceptance-mutation", "chat-channel-relay", "mutations")
	mutants := fixtureRuns(t, dir, 3)

	if _, err := Clean(build, Limits{Scenarios: 0, Mutants: 0}); err != nil {
		t.Fatal(err)
	}

	left := survivors(t, dir)
	if len(left) != 1 || left[0] != mutants[2] {
		t.Errorf("mutant runs left = %v, want the newest one (%s): it is what a run in flight is using", left, mutants[2])
	}
}

func TestCleanNamesOneRunAsOneRun(t *testing.T) {
	build := t.TempDir()
	fixtureRuns(t, filepath.Join(build, "acceptance", "run"), 2)

	report, err := Clean(build, Limits{Scenarios: 1, Mutants: 1})
	if err != nil {
		t.Fatal(err)
	}

	if report.String() != "removed 1 scenario run and 0 mutant runs" {
		t.Errorf("report = %q, want one run named as one run, and none named as none", report)
	}
}

func TestCleanBreaksATieByNameSoTheSameRunsSurvive(t *testing.T) {
	build := t.TempDir()
	dir := filepath.Join(build, "acceptance", "run")
	names := fixtureRuns(t, dir, 4)
	// The same moment for every run: with nothing to tell them apart by time,
	// the order they are kept in still has to be the same one every time, or
	// which runs survive a clean up is anyone's guess.
	when := time.Now().Add(-time.Hour)
	for _, name := range names {
		if err := os.Chtimes(filepath.Join(dir, name), when, when); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Clean(build, Limits{Scenarios: 2, Mutants: 1}); err != nil {
		t.Fatal(err)
	}

	left := survivors(t, dir)
	if len(left) != 2 || left[0] != names[2] || left[1] != names[3] {
		t.Errorf("scenario runs left = %v, want the last two by name (%v)", left, names[2:])
	}
}

func TestCleanLeavesABuildTreeThatHasNoRunsAlone(t *testing.T) {
	report, err := Clean(t.TempDir(), Limits{Scenarios: 3, Mutants: 2})
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed() != 0 {
		t.Errorf("report = %q, want nothing removed from a tree with no runs", report)
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
