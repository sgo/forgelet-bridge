package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
)

// adapterAddsForge asks the served forge root's adapter to add a forge, and
// keeps what it said.
func adapterAddsForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	// The forge root the adapter serves has to be one of the fixtures, even
	// though the adapter itself is told nothing about it.
	if _, err := w.declaredForge(captures[1]); err != nil {
		return err
	}
	name, err := w.declaredForge(captures[2])
	if err != nil {
		return err
	}
	// The adapter is asked for the forge root it serves: the copy the forge
	// keeps, run where the forge keeps it.
	_, _ = w.adapterCommand(ctx, "add-forge", name.Root(), captures[3])
	return nil
}

// adapterOutputNamesForge checks what the adapter said named the new forge.
func adapterOutputNamesForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if !strings.Contains(w.adapterOutput, captures[1]) {
		return fmt.Errorf("the adapter's output does not name %s:\n%s", captures[1], w.adapterOutput)
	}
	return nil
}

// adapterOutputNamesTheCopy checks the adapter said where it kept the copy of
// the configuration it replaced.
func adapterOutputNamesTheCopy(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	kept, err := w.keptConfiguration()
	if err != nil {
		return err
	}
	if !strings.Contains(w.adapterOutput, kept) {
		return fmt.Errorf("the adapter's output does not name the copy %s:\n%s", kept, w.adapterOutput)
	}
	return nil
}

// adapterKeptACopy checks the copy of the replaced configuration is there, and
// that it is the configuration that was there before.
func adapterKeptACopy(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	kept, err := w.keptConfiguration()
	if err != nil {
		return err
	}
	before, err := readForgeEntries(kept)
	if err != nil {
		return err
	}
	replaced, err := w.servedRootRoot()
	if err != nil {
		return err
	}
	if len(before) != 1 || before[0]["root"] != replaced {
		return fmt.Errorf("the copy the adapter kept holds %+v, want the configuration it replaced", before)
	}
	return nil
}

// bridgeConfigNamesForge checks the configuration of one forge root carries an
// entry for another.
func bridgeConfigNamesForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	named, err := w.declaredForge(captures[2])
	if err != nil {
		return err
	}
	entries, err := readForgeEntries(w.bridgeConfigOf(served.Root()))
	if err != nil {
		return err
	}
	want := named.Root()
	if len(captures) > 3 {
		for _, entry := range entries {
			if entry["root"] == want && entry["name"] == captures[3] {
				return nil
			}
		}
		return fmt.Errorf("the configuration does not name %s as %s: %+v", want, captures[3], entries)
	}
	for _, entry := range entries {
		if entry["root"] == want {
			return nil
		}
	}
	return fmt.Errorf("the configuration does not name %s: %+v", want, entries)
}

// bridgeConfigHoldsOnlyForge checks the configuration carries one forge and no
// other.
func bridgeConfigHoldsOnlyForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	only, err := w.declaredForge(captures[2])
	if err != nil {
		return err
	}
	entries, err := readForgeEntries(w.bridgeConfigOf(served.Root()))
	if err != nil {
		return err
	}
	if len(entries) != 1 || entries[0]["root"] != only.Root() {
		return fmt.Errorf("the configuration holds %+v, want only %s", entries, only.Root())
	}
	return nil
}

// adapterRefused checks the adapter refused the new forge.
func adapterRefused(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	return outputSays(w.adapterOutput, "refused", "the adapter did not refuse the new forge")
}

// statusNamesEveryConfiguredForgeAsReached checks the bridge's status names
// every forge in the configuration as one it reached.
func statusNamesEveryConfiguredForgeAsReached(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	entries, err := readForgeEntries(w.configPath)
	if err != nil {
		return err
	}
	path := filepath.Join(w.stateDir, bridge.StatusName)
	return waitFor(ctx, "the bridge's status never named every configured forge as reached", func() (bool, error) {
		status, err := readStatus(path)
		if err != nil {
			return false, nil
		}
		for _, entry := range entries {
			if !fixtures.Contains(status.ReachedForges, configuredName(entry)) {
				return false, nil
			}
		}
		return true, nil
	})
}

// statusNamesTheForgeAsReached waits for the bridge's status to name one forge
// as one it reached.
func statusNamesTheForgeAsReached(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return w.waitForStatusForge(ctx, captures[1], "one it reached", reachedForges)
}

// statusNamesTheForgeAsUnreached waits for the bridge's status to name one
// forge as one it did not reach.
func statusNamesTheForgeAsUnreached(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return w.waitForStatusForge(ctx, captures[1], "one it did not reach", unreachedForges)
}

// waitForStatusForge waits for the bridge's status to name a forge the way one
// of its forge lists reads.
func (w *World) waitForStatusForge(ctx context.Context, name, want string, list func(bridge.Status) []string) error {
	return waitFor(ctx, fmt.Sprintf("the bridge's status never named %s as %s", name, want), func() (bool, error) {
		status, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
		if err != nil {
			return false, nil
		}
		return fixtures.Contains(list(status), name), nil
	})
}

// reachedForges is the forges a status names as served, unreachedForges the
// forges it names as ones it could not serve.
func reachedForges(status bridge.Status) []string   { return status.ReachedForges }
func unreachedForges(status bridge.Status) []string { return status.UnhappyForges }
