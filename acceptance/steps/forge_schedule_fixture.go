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
	inner := filepath.Join(path, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		return "", err
	}
	work := filepath.Join(inner, "work.txt")
	if err := os.WriteFile(work, []byte("a run's scratch\n"), 0o644); err != nil {
		return "", err
	}
	// The sweep reads a scratch by when anything in it was last touched, so the
	// work, the room it sits in, and the scratch itself all read as the
	// fixture's own time rather than as the moment this step ran.
	for _, touched := range []string{work, inner, path} {
		if err := os.Chtimes(touched, once, once); err != nil {
			return "", err
		}
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-26T23:37:48+02:00","module_hash":"14d93250af5b4e3bf11415a8898b24f2d4295b805c703945aaff049677062572","functions":[{"id":"func/World.forgeTmp","name":"World.forgeTmp","line":29,"end_line":35,"hash":"03bc5ef8afa47a49d8f0d77557774c7c7665e0d69aefb94cbe647dd103bed7f6"},{"id":"func/World.scratchDirectory","name":"World.scratchDirectory","line":39,"end_line":62,"hash":"80b5373c50bd2de2fe4dedda5eabb71bb30d735d19d403543549c027cc64d295"},{"id":"func/forgeTmpHoldsAScratchNobodyHasTouched","name":"forgeTmpHoldsAScratchNobodyHasTouched","line":66,"end_line":74,"hash":"c1218f5f30a3a2e0e6ca99e0281d5c1a41d02529d84426a7ac82a7fd72f93fa4"},{"id":"func/forgeTmpHoldsAFreshScratch","name":"forgeTmpHoldsAFreshScratch","line":78,"end_line":86,"hash":"3ad0c154fcb4f29b4792f55130ec1808a67a6653b93827ae660cb5d621e6ed58"},{"id":"func/dayOf","name":"dayOf","line":90,"end_line":100,"hash":"1f275d00974c0bee9cc6db3b8cabe604f886716904cd1a66ec7780b63e302e26"},{"id":"func/scheduleLogPath","name":"scheduleLogPath","line":106,"end_line":109,"hash":"e2dfa4fff30f3a72d05e2199214d99099dd389dc46671eb23c9b44a98ff67c47"},{"id":"func/forgeHoldsTheScheduleLogFor","name":"forgeHoldsTheScheduleLogFor","line":113,"end_line":132,"hash":"1fe10586e8ad5bea15cf0765138e83e64e83673f3a8ddf93c4aff7d5a8c5bef3"},{"id":"func/forgeCarriesTheLayerPruner","name":"forgeCarriesTheLayerPruner","line":137,"end_line":157,"hash":"2e9eccf972ccdd25e3eefe608917f858b6183fe10128ad557907bb2dfcd674d5"},{"id":"func/forgeCarriesNoPruner","name":"forgeCarriesNoPruner","line":161,"end_line":172,"hash":"563b6827f4a0960d14cf5d1f69a1149429ee36099b0f109881004d38fd5a251a"},{"id":"func/oldTree","name":"oldTree","line":175,"end_line":191,"hash":"263574dd1fac5710a5be54fe008b8ebaad7ae727254acd87241ad69920787ed0"},{"id":"func/projectHoldsTargetDirs","name":"projectHoldsTargetDirs","line":196,"end_line":214,"hash":"0e6eab4cbd2174687711180ccd54ad2480a7deed1246e67f57f3c60588c9b2aa"},{"id":"func/projectHoldsAcceptanceBinaries","name":"projectHoldsAcceptanceBinaries","line":218,"end_line":236,"hash":"fb74c5d33d71575343ee47bad5afc8ad6c14d806e88cbda303b8ab2d9587fc51"}]}
// mutate4go-manifest-end
