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
	found, err := runsIn(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return removeStale(found, keep)
}

// runsIn is the run directories one directory of runs holds, each with when it
// was last touched.
func runsIn(dir string) ([]run, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	found := make([]run, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		found = append(found, run{path: filepath.Join(dir, entry.Name()), modified: info.ModTime().UnixNano()})
	}
	return found, nil
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
		runs, err := runsIn(path)
		if err != nil {
			return err
		}
		found = append(found, runs...)
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T22:22:56+02:00","module_hash":"9b54c94dc010d77aa986248b9c95853e94d437596a4d12edf4ed7fba15b4e010","functions":[{"id":"func/Limits.atLeastOne","name":"Limits.atLeastOne","line":33,"end_line":41,"hash":"8423fe3d1b45f8c0bb1b5df9d7eb50909ee6f127bd1bf0937aedcd95b3e74128"},{"id":"func/Report.Removed","name":"Report.Removed","line":50,"end_line":50,"hash":"76b1245b45575dda0bbb73a5fa9fa1dbf01801ce28edfa432a81cdf826d54df2"},{"id":"func/Report.String","name":"Report.String","line":55,"end_line":60,"hash":"405a4fcb239722d5354330bf64ff337ad7330c252ad5f6b3146d959182f99c99"},{"id":"func/count","name":"count","line":63,"end_line":68,"hash":"77e1033bf7de6f4175869d79248c8c63f80cdb24474787766a945bbcc3ea93cd"},{"id":"func/Clean","name":"Clean","line":72,"end_line":86,"hash":"abd2199ea8e2c21587502e6d69a56233348e8ffb92a68afed15a03d31960e561"},{"id":"func/prune","name":"prune","line":96,"end_line":105,"hash":"d0f8558f7c44e8aca40db6c6457eb6e4becfe57a28fb39419caa10a8a6d699f7"},{"id":"func/runsIn","name":"runsIn","line":109,"end_line":126,"hash":"d80e14917d7e1d62c8dcf105aeafb55ac4154f2e27182960112a668bfe965c87"},{"id":"func/pruneEverywhere","name":"pruneEverywhere","line":131,"end_line":159,"hash":"b80393c9d05f58f88498bb1dfa5e7be05d9d0c3ba67197a7610aa45b4500b352"},{"id":"func/removeStale","name":"removeStale","line":164,"end_line":182,"hash":"e3a545bf9e7e77373662a718dda2a2945f707e44496ec845f9c3489304c17e04"}]}
// mutate4go-manifest-end
