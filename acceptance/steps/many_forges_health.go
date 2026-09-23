package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// dashboardOf is the running dashboard of a fixture forge.
func (w *World) dashboardOf(name string) (*fixtures.Dashboard, error) {
	if _, err := w.declaredForge(name); err != nil {
		return nil, err
	}
	running, ok := w.running[name]
	if !ok {
		return nil, fmt.Errorf("the fixture forge root %s does not have its dashboard running", name)
	}
	return running, nil
}

// dashboardStopped stops one forge's dashboard, leaving the bridge with a
// forge it cannot reach while the others keep working.
func dashboardStopped(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	running, err := w.dashboardOf(captures[1])
	if err != nil {
		return err
	}
	running.Stop()
	delete(w.running, captures[1])
	return nil
}

// dashboardStartedAgain brings one forge's dashboard back. The address it
// announced before is dropped first, so the fixture waits for the address the
// dashboard announces now rather than reading the stale one.
func dashboardStartedAgain(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(store.Root(), filepath.FromSlash(dashboard.URLFile))); err != nil && !os.IsNotExist(err) {
		return err
	}
	return w.startDashboard(captures[1])
}

// statusNamesTheOnlyUnhappyForge waits for the bridge to report one named
// forge as the only one it cannot serve.
func statusNamesTheOnlyUnhappyForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	name := captures[1]
	return waitFor(ctx, fmt.Sprintf("the bridge's status never named %s as the only unhappy forge", name), func() (bool, error) {
		status, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
		if err != nil {
			return false, nil
		}
		return len(status.UnhappyForges) == 1 && status.UnhappyForges[0] == name, nil
	})
}

// statusNamesNoUnhappyForge waits for the bridge to report every forge served.
func statusNamesNoUnhappyForge(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return waitFor(ctx, "the bridge's status never stopped naming an unhappy forge", func() (bool, error) {
		status, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
		if err != nil {
			return false, nil
		}
		return len(status.UnhappyForges) == 0, nil
	})
}
