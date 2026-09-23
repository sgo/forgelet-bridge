// Package runs prunes the throwaway runs a build leaves behind: the scenario
// runs an acceptance pass starts under the build tree, and the mutant runs a
// mutation pass leaves there.
//
// A run is a directory of its own. The newest of each kind is what a run still
// in flight is using, so a clean up keeps the newest few and removes the rest,
// and says what it removed: a failure can still be looked at, and the tree
// stops growing without limit. The two kinds are kept to different limits
// because the sizes differ: a scenario run is a few megabytes, a mutant run is
// hundreds.
package runs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// mutationsDir is the directory a mutation pass puts its runs in, wherever the
// pass rooted its work.
const mutationsDir = "mutations"

// Limits are how many runs of each kind to keep.
type Limits struct {
	Scenarios int
	Mutants   int
}

// atLeastOne keeps the newest run of each kind: that is what a run still in
// flight is using, and it is never a candidate.
func (l Limits) atLeastOne() Limits {
	if l.Scenarios < 1 {
		l.Scenarios = 1
	}
	if l.Mutants < 1 {
		l.Mutants = 1
	}
	return l
}

// Report is what a clean up removed, by kind.
type Report struct {
	Scenarios int
	Mutants   int
}

// Removed is every run the clean up took away.
func (r Report) Removed() int { return r.Scenarios + r.Mutants }

// String is what the clean up says, and is the whole of what a person sees of
// it: a clean up that says nothing is indistinguishable from one that did not
// run.
func (r Report) String() string {
	if r.Removed() == 0 {
		return "there was nothing to remove"
	}
	return fmt.Sprintf("removed %s and %s", count(r.Scenarios, "scenario run"), count(r.Mutants, "mutant run"))
}

// count names how many of one kind went.
func count(found int, what string) string {
	if found == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", found, what)
}

// Clean removes the scenario runs and the mutant runs a build tree no longer
// needs, keeping the newest of each kind, and reports what it removed.
func Clean(buildDir string, limits Limits) (Report, error) {
	limits = limits.atLeastOne()
	var report Report
	scenarios, err := prune(filepath.Join(buildDir, "acceptance", "run"), limits.Scenarios)
	if err != nil {
		return report, err
	}
	report.Scenarios = scenarios
	mutants, err := pruneEverywhere(filepath.Join(buildDir, "acceptance-mutation"), mutationsDir, limits.Mutants)
	if err != nil {
		return report, err
	}
	report.Mutants = mutants
	return report, nil
}

// run is one directory of runs: where it is, and when it was last touched.
type run struct {
	path     string
	modified int64
}

// prune removes every subdirectory of dir but the newest keep, and reports how
// many it removed. A directory that is not there holds nothing to remove.
func prune(dir string, keep int) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	found := make([]run, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, err
		}
		found = append(found, run{path: filepath.Join(dir, entry.Name()), modified: info.ModTime().UnixNano()})
	}
	return removeStale(found, keep)
}

// pruneEverywhere removes runs that a pass left under a tree of its own: every
// directory named `name` anywhere below root holds runs, and the newest keep of
// them all stay.
func pruneEverywhere(root, name string, keep int) (int, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var found []run
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() || entry.Name() != name {
			return nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, child := range entries {
			if !child.IsDir() {
				continue
			}
			info, err := child.Info()
			if err != nil {
				return err
			}
			found = append(found, run{path: filepath.Join(path, child.Name()), modified: info.ModTime().UnixNano()})
		}
		// A run's own contents are not runs, so there is nothing to find in
		// there.
		return fs.SkipDir
	})
	if err != nil {
		return 0, err
	}
	return removeStale(found, keep)
}

// removeStale keeps the newest keep of the runs and removes every other one,
// newest first among the ones that go, so a failure can be looked at while the
// rest is cleared away.
func removeStale(found []run, keep int) (int, error) {
	sort.Slice(found, func(i, j int) bool {
		if found[i].modified != found[j].modified {
			return found[i].modified > found[j].modified
		}
		return found[i].path > found[j].path
	})
	if keep > len(found) {
		keep = len(found)
	}
	removed := 0
	for _, stale := range found[keep:] {
		if err := os.RemoveAll(stale.path); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
