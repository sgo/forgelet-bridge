// Package schedule holds the tests that run the forge schedule - the cadence the
// kit ships - against a fixture forge root, so what a pass does to the forge it
// serves is pinned without a machine's launch agent behind it.
//
// The watch's pass and the doorbell's pass have their own tests; what is pinned
// here is the trimming the cadence does because it is the only thing that runs
// whether or not a session is up: the forge's own scratch by age, its own log by
// the day, and the layer's pruner once a day.
package schedule

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

// The things a pass has to tell apart: scratch nobody has touched, under a name
// that says nothing, beside the scratch a run left just now under the name a
// compose gives it; and the days of the schedule's own log, inside the bound and
// outside it.
const (
	staleScratch = "notes-from-a-run-2f91"
	freshScratch = "swarmforge-compose-9c17"
	weeks        = 21 * 24 * time.Hour
)

// fixture is one forge root the schedule serves, with the scratch, the log and
// the layer's pruner a scenario gives it.
type fixture struct {
	t     *testing.T
	root  string
	calls string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "forge-a")
	f := &fixture{t: t, root: root, calls: filepath.Join(root, "pruner-calls")}
	project := filepath.Join(root, "projects", "forgelet-bridge")
	for _, dir := range []string{
		filepath.Join(root, ".swarmforge"),
		filepath.Join(root, "swarmforge", "scripts"),
		filepath.Join(project, ".swarmforge"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	roles := "coder\tmaster\t" + root + "\tfixture-pane\tCoder\tcodex\ttask\tforward-only\n"
	f.write(filepath.Join(root, ".swarmforge", "roles.tsv"), roles)
	f.write(filepath.Join(project, ".swarmforge", "roles.tsv"), roles)
	f.write(filepath.Join(root, "swarmforge", "scripts", "prune_build_debris.sh"),
		"#!/bin/sh\necho \"$@\" >> "+f.calls+"\necho 'pruned 0 directories, freed 0 MB'\n")
	if err := os.Chmod(filepath.Join(root, "swarmforge", "scripts", "prune_build_debris.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) write(path, body string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// scratch leaves one directory in the forge's own tmp, untouched since the time
// given.
func (f *fixture) scratch(name string, since time.Time) string {
	f.t.Helper()
	path := filepath.Join(f.root, "tmp", name)
	if err := os.MkdirAll(filepath.Join(path, "inner"), 0o755); err != nil {
		f.t.Fatal(err)
	}
	inner := filepath.Join(path, "inner", "work.txt")
	f.write(inner, "a run's scratch\n")
	for _, at := range []string{inner, filepath.Join(path, "inner"), path} {
		if err := os.Chtimes(at, since, since); err != nil {
			f.t.Fatal(err)
		}
	}
	return path
}

// logPath is the file the schedule keeps one day's own log in.
func (f *fixture) logPath(ago time.Duration) string {
	day := time.Now().UTC().Add(-ago).Format("2006-01-02")
	return filepath.Join(f.root, ".swarmforge", "stall-watch-"+day+".log")
}

// log leaves one day of the schedule's own log behind.
func (f *fixture) log(ago time.Duration) string {
	f.t.Helper()
	path := f.logPath(ago)
	f.write(path, "checked 3 projects across 2 forges\n")
	return path
}

// run runs the pass the way the machine's agent runs it.
func (f *fixture) run() string {
	f.t.Helper()
	command := exec.Command(filepath.Join(projectRoot(f.t), "swarmforge", "scripts", "forge_schedule.sh"), "run", f.root)
	command.Dir = f.root
	out, _ := command.CombinedOutput()
	return string(out)
}

// prunerRuns is how many times the layer's pruner was asked to run.
func (f *fixture) prunerRuns() int {
	f.t.Helper()
	data, err := os.ReadFile(f.calls)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		f.t.Fatal(err)
	}
	runs := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			runs++
		}
	}
	return runs
}

func (f *fixture) gone(path string) bool {
	f.t.Helper()
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// TestThePassSweepsTheForgesOwnScratchByAge pins the rule the card is loudest
// about: what a failed run left is removed because nobody has touched it, not
// because of its name - and the removal is named in the pass's words, so it is
// visible rather than silent.
func TestThePassSweepsTheForgesOwnScratchByAge(t *testing.T) {
	f := newFixture(t)
	stale := f.scratch(staleScratch, time.Now().UTC().Add(-weeks))
	fresh := f.scratch(freshScratch, time.Now().UTC())

	out := f.run()

	if !f.gone(stale) {
		t.Errorf("the pass left the scratch nobody has touched for weeks: %s", stale)
	}
	if !strings.Contains(out, "removed the scratch") || !strings.Contains(out, stale) {
		t.Errorf("the pass does not say it removed %s:\n%s", stale, out)
	}
	if f.gone(fresh) {
		t.Errorf("the pass removed the scratch a run left just now, under the name a compose gives it: %s", fresh)
	}
}

// TestThePassKeepsAWeekOfItsOwnLog pins the log's own rule: a day to a file,
// with the days outside the bound dropped and named, and the ones inside it left
// alone - age cannot judge a file the pass writes every minute.
func TestThePassKeepsAWeekOfItsOwnLog(t *testing.T) {
	f := newFixture(t)
	today := f.log(0)
	yesterday := f.log(24 * time.Hour)
	old := f.log(weeks)

	out := f.run()

	if !f.gone(old) {
		t.Errorf("the pass kept its own log for a day three weeks ago: %s", old)
	}
	if !strings.Contains(out, "dropped the schedule's own log") || !strings.Contains(out, old) {
		t.Errorf("the pass does not say it dropped %s:\n%s", old, out)
	}
	for _, kept := range []string{today, yesterday} {
		if f.gone(kept) {
			t.Errorf("the pass dropped %s, which is inside the week it keeps", kept)
		}
	}
}

// TestThePassLeavesAScratchARunIsStillFilling pins the other side of the age
// rule: what a failed run left is swept because nobody is looking at it, so a
// scratch somebody wrote in a moment ago is left alone however old the directory
// itself is.
func TestThePassLeavesAScratchARunIsStillFilling(t *testing.T) {
	f := newFixture(t)
	path := f.scratch("a-scratch-a-run-is-still-filling", time.Now().UTC().Add(-weeks))
	inner := filepath.Join(path, "inner", "work.txt")
	now := time.Now().UTC()
	if err := os.Chtimes(inner, now, now); err != nil {
		t.Fatal(err)
	}

	f.run()

	if f.gone(path) {
		t.Errorf("the pass removed a scratch a run wrote in a moment ago: %s", path)
	}
}

// TestThePassPrunesTheProjectsOnceADay pins the cadence the projects take: the
// walk over every worktree is not a thing to do every minute, so a pass that has
// already pruned today leaves it to the next day - and a forge whose layer ships
// no pruner is named rather than passed over.
func TestThePassPrunesTheProjectsOnceADay(t *testing.T) {
	f := newFixture(t)

	first := f.run()
	second := f.run()

	if runs := f.prunerRuns(); runs != 1 {
		t.Errorf("the layer's pruner ran %d times over two passes, and the walk is once a day:\n%s\n%s", runs, first, second)
	}
	if !strings.Contains(first, "pruned the projects of "+f.root) {
		t.Errorf("the first pass does not say it pruned the projects:\n%s", first)
	}
}

// TestThePassKeepsItsOwnWordsInTodaysLog pins the half the log's rule exists
// for: the pass writes what it says into the day's file itself, because a path
// the machine's agent opens every minute is one unbounded file whichever day it
// belongs to.
func TestThePassKeepsItsOwnWordsInTodaysLog(t *testing.T) {
	f := newFixture(t)

	out := f.run()

	today := f.logPath(0)
	data, err := os.ReadFile(today)
	if err != nil {
		t.Fatalf("the pass kept no log for today (%s): %v\n%s", today, err, out)
	}
	for _, want := range []string{"the forge schedule ran for", "checked 1 projects"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("today's log does not carry %q:\n%s", want, data)
		}
	}
}

// TestTheMachineDoesNotAppendToOneUnboundedLog pins the other end of the log's
// rule: the agent the machine loads runs the pass, and the pass keeps the log,
// so the agent must not be the thing appending to one file forever.
func TestTheMachineDoesNotAppendToOneUnboundedLog(t *testing.T) {
	f := newFixture(t)

	command := exec.Command(filepath.Join(projectRoot(t), "swarmforge", "scripts", "stall_watch.sh"), "print-agent", f.root)
	command.Dir = f.root
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("the agent could not be read: %v\n%s", err, out)
	}
	agent := string(out)
	if !strings.Contains(agent, "forge_schedule.sh") {
		t.Errorf("the agent does not run the pass that keeps the log:\n%s", agent)
	}
	if strings.Contains(agent, "stall-watch.log") {
		t.Errorf("the agent still appends to one unbounded log:\n%s", agent)
	}
}

// TestAForgeWithoutTheLayersPrunerIsNamed is the other layer's shape: a forge
// composed from a layer that ships no pruner still runs, and the pass says which
// forge it could not prune instead of failing on it.
func TestAForgeWithoutTheLayersPrunerIsNamed(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.root, "swarmforge", "scripts", "prune_build_debris.sh")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	out := f.run()

	if !strings.Contains(out, "the projects of "+f.root+" were not pruned because the pruner is not there") {
		t.Errorf("the pass does not name the forge it could not prune:\n%s", out)
	}
	if !strings.Contains(out, "the forge schedule ran for") {
		t.Errorf("the pass did not finish:\n%s", out)
	}
}
