// Package prune holds the tests that run the layer's pruner - the script the
// pass hands a forge's projects to - against a fixture forge root.
//
// What a run's own cleaner does at the end of a run is pinned in internal/runs.
// What is pinned here is the same rule meeting the one caller that runs while a
// run may still be going: the count keeps the newest few so a failure can still
// be looked at, and a directory of runs something is writing in right now is
// left alone rather than emptied around the run happening in it.
package prune

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The times a fixture leaves its runs at: a run nobody has touched for a month,
// a run a pass left half an hour ago - past the window a run in flight is read
// within, so the count is free to take it - and a run filling its scratch as the
// prune runs.
const (
	quietFor = 30 * 24 * time.Hour
	staleFor = 30 * time.Minute
	justNow  = time.Second
)

// projectRoot is the checkout the tool under test lives in.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("the test cannot say where it is")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// fixture is one forge root with the project the pruner walks, and the scratch
// a run of the project's own leaves under it.
type fixture struct {
	t       *testing.T
	root    string
	project string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, root: filepath.Join(t.TempDir(), "forge-a")}
	f.project = filepath.Join(f.root, "projects", "forgelet-bridge")
	if err := os.MkdirAll(f.project, 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

// write leaves one working file of a run at path.
func (f *fixture) write(path string) string {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("a run's own working file\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return path
}

// touch gives a path the time the fixture says it was last written to.
func (f *fixture) touch(path string, when time.Time) {
	f.t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		f.t.Fatal(err)
	}
}

// aged gives a whole tree the time the fixture says it was last written to: a
// run writes throughout its own scratch, so a run nobody has written in has
// nothing fresh anywhere inside it, its own directory included.
func (f *fixture) aged(root string, when time.Time) {
	f.t.Helper()
	err := filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, when, when)
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

// scenarioRun leaves one run where an acceptance pass puts it: a directory under
// build/acceptance/run, with the work the run did inside it.
func (f *fixture) scenarioRun(base, name string, when time.Time) string {
	f.t.Helper()
	dir := filepath.Join(base, "build", "acceptance", "run", name)
	f.write(filepath.Join(dir, "synapse", "homeserver.db"))
	f.aged(dir, when)
	return dir
}

// mutantRun leaves one run where a mutation pass puts it: a directory of its own
// under a directory of mutants, with the work the run did inside it.
func (f *fixture) mutantRun(mutations, name string, when time.Time) string {
	f.t.Helper()
	dir := filepath.Join(mutations, name)
	f.write(filepath.Join(dir, "feature.json"))
	f.write(filepath.Join(dir, "acceptance-run", "scenario-1", "homeserver.db"))
	f.write(filepath.Join(dir, "acceptance-run", "scenario-1", "bridge.log"))
	f.aged(dir, when)
	return dir
}

// acceptanceRun and mutationDir are the two debris trees a worktree of the
// project holds, where the passes root them.
func (f *fixture) acceptanceRun() string {
	return filepath.Join(f.project, "build", "acceptance", "run")
}

func (f *fixture) mutationDir(where ...string) string {
	path := append([]string{f.project, "build", "acceptance-mutation"}, where...)
	return filepath.Join(append(path, "mutations")...)
}

// ran prunes the fixture forge the way the pass hands it to the pruner.
func (f *fixture) ran(args ...string) string {
	f.t.Helper()
	command := exec.Command(filepath.Join(projectRoot(f.t), "swarmforge", "scripts", "prune_build_debris.sh"),
		append([]string{f.root}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		f.t.Fatalf("the pruner failed: %v\n%s", err, out)
	}
	return string(out)
}

// gone is whether a path the prune was free to take is no longer there.
func (f *fixture) gone(path string) bool {
	f.t.Helper()
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// stillHere is every path of a set that survives, named rather than counted, so
// a failure says which run went.
func (f *fixture) stillHere(paths []string) []string {
	f.t.Helper()
	var left []string
	for _, path := range paths {
		if !f.gone(path) {
			left = append(left, path)
		}
	}
	return left
}

// lastLine is the one line a caller that keeps a single line hears: what the
// pass writes into the forge's own log.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return lines[len(lines)-1]
}

// TestAPruneLeavesTheRunsARunIsStillUsing is the case the count alone cannot
// see: a mutation pass puts a burst of runs under one directory, the rule keeps
// the newest of them, and the pass that hands the projects over is day-gated, so
// it lands in the middle of a burst sooner or later. The run started before its
// short neighbours is the one that would be emptied out from under: nothing in
// the directory goes while something there was written just now, and the prune
// names what said so.
func TestAPruneLeavesTheRunsARunIsStillUsing(t *testing.T) {
	f := newFixture(t)
	mutations := f.mutationDir("forge-schedule")
	now := time.Now()

	// The long run, started first and still filling its scratch.
	live := f.mutantRun(mutations, "m1", now.Add(-staleFor))
	fresh := f.write(filepath.Join(live, "acceptance-run", "scenario-1", "bridge.log"))
	f.touch(fresh, now.Add(-justNow))
	f.touch(filepath.Dir(fresh), now.Add(-justNow))
	// The burst of short runs beside it, newest last.
	burst := []string{
		f.mutantRun(mutations, "m2", now.Add(-staleFor)),
		f.mutantRun(mutations, "m3", now.Add(-staleFor+time.Minute)),
		f.mutantRun(mutations, "m4", now.Add(-staleFor+2*time.Minute)),
	}
	f.touch(mutations, now.Add(-staleFor))

	out := f.ran()

	want := append([]string{live}, burst...)
	if left := f.stillHere(want); len(left) != len(want) {
		t.Errorf("the prune took some of %v, and a run wrote in that directory just now", left)
	}
	if !strings.Contains(out, "left the runs under "+mutations+" alone") {
		t.Errorf("the prune does not say it left the runs alone:\n%s", out)
	}
	if line := lastLine(out); !strings.Contains(line, "left the runs under 1 directory alone") {
		t.Errorf("the one line a caller keeps does not name the directory it left alone: %q", line)
	}
}

// TestAPruneTakesTheRunsNobodyIsUsing is the other half of the same reading: a
// directory of runs nothing has written in for days is the debris the rule
// exists for, and the count is what decides which of them go.
func TestAPruneTakesTheRunsNobodyIsUsing(t *testing.T) {
	f := newFixture(t)
	mutations := f.mutationDir("forge-schedule")
	when := time.Now().Add(-quietFor)

	stale := []string{
		f.mutantRun(mutations, "m1", when.Add(time.Minute)),
		f.mutantRun(mutations, "m2", when.Add(2*time.Minute)),
		f.mutantRun(mutations, "m3", when.Add(3*time.Minute)),
	}
	kept := f.mutantRun(mutations, "m4", when.Add(4*time.Minute))
	f.touch(mutations, when)

	out := f.ran()

	for _, dir := range stale {
		if !f.gone(dir) {
			t.Errorf("the prune left %s, and nothing has written in that directory for a month", dir)
		}
	}
	if f.gone(kept) {
		t.Errorf("the prune took %s, which is the newest run the rule keeps", kept)
	}
	if !strings.Contains(out, "pruned 3 directories") {
		t.Errorf("the prune does not say what it removed:\n%s", out)
	}
}

// TestAPruneLeavesTheScenarioRunsAScenarioIsStillUsing is the same reading on
// the other debris tree the project leaves: the count keeps the newest twenty
// scenario runs, a suite of more than twenty puts the run in flight among them,
// and the directory is left alone for the pass rather than taken out from under
// the scenario the suite is in the middle of.
func TestAPruneLeavesTheScenarioRunsAScenarioIsStillUsing(t *testing.T) {
	f := newFixture(t)
	run := f.acceptanceRun()
	now := time.Now()

	var runs []string
	for i := 0; i < 21; i++ {
		runs = append(runs, f.scenarioRun(f.project, fmt.Sprintf("scenario-%02d", i), now.Add(-staleFor)))
	}
	live := f.scenarioRun(f.project, "scenario-live", now.Add(-staleFor))
	fresh := f.write(filepath.Join(live, "synapse", "homeserver.db.log"))
	f.touch(fresh, now.Add(-justNow))
	f.touch(filepath.Dir(fresh), now.Add(-justNow))
	runs = append(runs, live)
	f.touch(run, now.Add(-staleFor))

	out := f.ran()

	if left := f.stillHere(runs); len(left) != len(runs) {
		t.Errorf("the prune took some of the runs of a suite still in flight: %v", left)
	}
	if !strings.Contains(out, "left the runs under "+run+" alone") {
		t.Errorf("the prune does not say it left the runs alone:\n%s", out)
	}
}

// TestAPruneReadsTheRunsWhereverThePassRootedThem pins the reach: the mutation
// pass roots its work wherever it is run from, so the directory of mutants is
// the one named mutations anywhere below the mutation tree - beside the tree's
// own project, per feature, and in a worktree - read the way this project's own
// cleaner reads it.
func TestAPruneReadsTheRunsWhereverThePassRootedThem(t *testing.T) {
	f := newFixture(t)
	when := time.Now().Add(-quietFor)
	worktree := filepath.Join(f.project, ".worktrees", "coder")
	dirs := []string{
		f.mutationDir(),                 // build/acceptance-mutation/mutations
		f.mutationDir("forge-schedule"), // build/acceptance-mutation/<feature>/mutations
		filepath.Join(worktree, "build", "acceptance-mutation", "doorbell", "mutations"),
	}

	var stale, kept []string
	for _, dir := range dirs {
		stale = append(stale, f.mutantRun(dir, "m1", when))
		kept = append(kept, f.mutantRun(dir, "m2", when.Add(time.Minute)))
		f.touch(dir, when)
	}

	f.ran()

	for _, dir := range stale {
		if !f.gone(dir) {
			t.Errorf("the prune never found %s", dir)
		}
	}
	for _, dir := range kept {
		if f.gone(dir) {
			t.Errorf("the prune took %s, which is the newest run of its directory", dir)
		}
	}
}

// TestAPruneLeavesWhatTheRuleNeverCovers pins what the reach is not, now that
// it takes the directory of mutants wherever the pass rooted it: the binaries
// the suite runs from, and a directory of mutants anywhere but under the
// mutation tree this project's passes write.
func TestAPruneLeavesWhatTheRuleNeverCovers(t *testing.T) {
	f := newFixture(t)
	when := time.Now().Add(-quietFor)
	elsewhere := filepath.Join(f.project, "build", "target", "mutations")

	binary := f.write(filepath.Join(f.project, "build", "acceptance", "bin", "forgelet-bridge"))
	f.touch(binary, when)
	untouched := []string{
		f.mutantRun(elsewhere, "m1", when),
		f.mutantRun(elsewhere, "m2", when.Add(time.Minute)),
	}
	f.aged(filepath.Join(f.project, "build", "target"), when)

	f.ran()

	if f.gone(binary) {
		t.Errorf("the prune took the binary the suite runs from: %s", binary)
	}
	if left := f.stillHere(untouched); len(left) != len(untouched) {
		t.Errorf("the prune reached into a project's own build output: %v", left)
	}
}

// TestTheWindowIsWhatDecidesWhetherARunIsInFlight pins the reading itself: the
// same burst is taken when the caller says a run has to have written in the
// directory within no time at all.
func TestTheWindowIsWhatDecidesWhetherARunIsInFlight(t *testing.T) {
	f := newFixture(t)
	mutations := f.mutationDir("forge-schedule")
	now := time.Now()

	stale := []string{
		f.mutantRun(mutations, "m1", now.Add(-4*time.Minute)),
		f.mutantRun(mutations, "m2", now.Add(-3*time.Minute)),
		f.mutantRun(mutations, "m3", now.Add(-2*time.Minute)),
	}
	kept := f.mutantRun(mutations, "m4", now.Add(-time.Minute))
	f.touch(mutations, now.Add(-4*time.Minute))

	out := f.ran("--in-flight", "0")

	for _, dir := range stale {
		if !f.gone(dir) {
			t.Errorf("a burst minutes quiet was left alone with no window to leave it for: %s", dir)
		}
	}
	if f.gone(kept) {
		t.Errorf("the prune took %s, which is the newest run the rule keeps", kept)
	}
	if strings.Contains(out, "left the runs under") {
		t.Errorf("the prune says it left runs alone, and nothing was written in them just now:\n%s", out)
	}
}

// TestAPruneSaysNothingAboutATreeItKeepsWhole pins the counting the pass reads:
// a tree the rule keeps everything of is left without a word about being left
// alone, because nothing was in flight and nothing was taken.
func TestAPruneSaysNothingAboutATreeItKeepsWhole(t *testing.T) {
	f := newFixture(t)
	kept := f.scenarioRun(f.project, "scenario-1", time.Now().Add(-quietFor))

	out := f.ran()

	if f.gone(kept) {
		t.Errorf("the prune took the newest scenario run: %s", kept)
	}
	if strings.Contains(out, "left the runs under") {
		t.Errorf("the prune says it left a tree alone, and the rule keeps all of it anyway:\n%s", out)
	}
	if !strings.Contains(out, "pruned 0 directories") {
		t.Errorf("the prune does not say it took nothing:\n%s", out)
	}
}

// TestADryRunSaysWhatWouldGoAndTakesNothing pins the shape a person runs it in
// before trusting it on a forge: the runs past the count are named one by one,
// the summary counts them, and nothing is touched.
func TestADryRunSaysWhatWouldGoAndTakesNothing(t *testing.T) {
	f := newFixture(t)
	mutations := f.mutationDir("forge-schedule")
	when := time.Now().Add(-quietFor)

	runs := []string{
		f.mutantRun(mutations, "m1", when),
		f.mutantRun(mutations, "m2", when.Add(time.Minute)),
		f.mutantRun(mutations, "m3", when.Add(2*time.Minute)),
	}
	f.touch(mutations, when)

	out := f.ran("--dry-run")

	if left := f.stillHere(runs); len(left) != len(runs) {
		t.Errorf("a dry run took %v, and it is the run that says what would go", left)
	}
	for _, gone := range runs[:2] {
		if !strings.Contains(out, "would remove "+gone) {
			t.Errorf("the dry run does not name %s:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "dry run: 2 directories") {
		t.Errorf("the dry run does not count what would go:\n%s", out)
	}
}
