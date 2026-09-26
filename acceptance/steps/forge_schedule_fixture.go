package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// The things the schedule trims, as the fixtures leave them. The stale scratch
// is left under a name that says nothing about what left it, so only its age can
// remove it; the fresh one is left under the name a compose gives its scratch,
// which is what a sweep reading names instead of age would delete.
const (
	staleScratch = "notes-from-a-run-2f91"
	freshScratch = "swarmforge-compose-9c17"
	logName      = "stall-watch-%s.log"
	prunerName   = "prune_build_debris.sh"
)

// staleSince is when the fixture says the untouched things were last touched:
// weeks ago, so any bound the schedule keeps is older than them.
var staleSince = func() time.Time { return time.Now().UTC().Add(-21 * 24 * time.Hour) }

// forgeTmp is one fixture forge's own scratch.
func (w *World) forgeTmp(name string) (string, error) {
	root, err := w.forgeRootOf(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "tmp"), nil
}

// scratchDirectory leaves one directory in a fixture forge's tmp, untouched
// since the fixture says.
func (w *World) scratchDirectory(name, directory string, once time.Time) (string, error) {
	tmp, err := w.forgeTmp(name)
	if err != nil {
		return "", err
	}
	path := filepath.Join(tmp, directory)
	if err := os.MkdirAll(filepath.Join(path, "inner"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(path, "inner", "work.txt"), []byte("a run's scratch\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.Chtimes(filepath.Join(path, "inner", "work.txt"), once, once); err != nil {
		return "", err
	}
	if err := os.Chtimes(filepath.Join(path, "inner"), once, once); err != nil {
		return "", err
	}
	if err := os.Chtimes(path, once, once); err != nil {
		return "", err
	}
	return path, nil
}

// forgeTmpHoldsAScratchNobodyHasTouched leaves the scratch a failed run left
// weeks ago, under a name that says nothing about it.
func forgeTmpHoldsAScratchNobodyHasTouched(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	path, err := w.scratchDirectory(captures[1], staleScratch, staleSince())
	if err != nil {
		return err
	}
	w.staleScratch = path
	return nil
}

// forgeTmpHoldsAFreshScratch leaves the scratch a run left just now, under the
// name a compose gives its scratch.
func forgeTmpHoldsAFreshScratch(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	path, err := w.scratchDirectory(captures[1], freshScratch, time.Now().UTC())
	if err != nil {
		return err
	}
	w.freshScratch = path
	return nil
}

// dayOf turns the way a scenario names a day into how long ago it was: the
// schedule keeps a day to a file, so a scenario names the file it left.
func dayOf(named string) (time.Duration, error) {
	switch named {
	case "today":
		return 0, nil
	case "yesterday":
		return 24 * time.Hour, nil
	case "a day three weeks ago":
		return 21 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("the step does not know the day %q", named)
}

// scheduleLogPath is the name the schedule keeps one day's own log under. The
// pass names its days in UTC, the way its heartbeat does, so the fixture does
// too: a fixture on a machine whose local day differs still leaves the file the
// pass will look for.
func scheduleLogPath(root string, ago time.Duration) string {
	day := time.Now().UTC().Add(-ago).Format("2006-01-02")
	return filepath.Join(root, ".swarmforge", fmt.Sprintf(logName, day))
}

// forgeHoldsTheScheduleLogFor leaves one day of the schedule's own log behind,
// where the pass looks for it and drops what is older than it keeps.
func forgeHoldsTheScheduleLogFor(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	ago, err := dayOf(captures[2])
	if err != nil {
		return err
	}
	path := scheduleLogPath(root, ago)
	if err := writeFile(path, "checked 3 projects across 2 forges\n"); err != nil {
		return err
	}
	if w.scheduleLogs == nil {
		w.scheduleLogs = map[string]string{}
	}
	w.scheduleLogs[captures[2]] = path
	return nil
}

// forgeCarriesTheLayerPruner puts the layer's own pruner where the pass looks
// for it: the script the layer ships, not a stand-in, so the scenario proves
// what that script does and does not touch.
func forgeCarriesTheLayerPruner(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	shipped := filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", prunerName)
	body, err := os.ReadFile(shipped)
	if err != nil {
		return fmt.Errorf("the layer ships no pruner at %s: %w", shipped, err)
	}
	path := filepath.Join(root, "swarmforge", "scripts", prunerName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, body, 0o755); err != nil {
		return err
	}
	w.prunerPath = path
	return nil
}

// forgeCarriesNoPruner is the forge composed from a layer that ships no pruner:
// the pass has to say so and carry on.
func forgeCarriesNoPruner(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	w.prunerPath = filepath.Join(root, "swarmforge", "scripts", prunerName)
	if err := os.Remove(w.prunerPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// oldTree leaves a tree that was built long ago, at one path.
func oldTree(path string) error {
	if err := os.MkdirAll(filepath.Join(path, "classes"), 0o755); err != nil {
		return err
	}
	for _, file := range []string{filepath.Join(path, "classes", "Main.class"), filepath.Join(path, "build.log")} {
		if err := os.WriteFile(file, []byte("a build long past\n"), 0o644); err != nil {
			return err
		}
		if err := os.Chtimes(file, staleSince(), staleSince()); err != nil {
			return err
		}
	}
	if err := os.Chtimes(filepath.Join(path, "classes"), staleSince(), staleSince()); err != nil {
		return err
	}
	return os.Chtimes(path, staleSince(), staleSince())
}

// projectHoldsTargetDirs leaves the build output a Maven project keeps - in its
// own tree and in one of its worktrees - which is the output the pass must never
// reach, however old it is.
func projectHoldsTargetDirs(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[2])
	if err != nil {
		return err
	}
	project := filepath.Join(root, "projects", captures[1])
	targets := []string{
		filepath.Join(project, "target"),
		filepath.Join(project, ".worktrees", "coder", "target"),
	}
	for _, target := range targets {
		if err := oldTree(target); err != nil {
			return err
		}
		w.targetDirs = append(w.targetDirs, target)
	}
	return nil
}

// projectHoldsAcceptanceBinaries leaves the binaries the acceptance suite runs
// from, which the pruner that walks the runs beside them must not remove.
func projectHoldsAcceptanceBinaries(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[2])
	if err != nil {
		return err
	}
	project := filepath.Join(root, "projects", captures[1])
	for _, name := range []string{"forgelet-bridge", "acceptance-entrypoint-generator"} {
		path := filepath.Join(project, "build", "acceptance", "bin", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("the binary the suite runs\n"), 0o755); err != nil {
			return err
		}
		w.binaries = append(w.binaries, path)
	}
	return nil
}
