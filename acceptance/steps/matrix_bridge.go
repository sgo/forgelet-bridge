package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
)

// stopAdapterBridge stops the bridge a forge root's adapter started, if the
// scenario started one.
func (w *World) stopAdapterBridge() {
	if w.servedRoot == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), stepTimeout)
	defer cancel()
	_, _ = w.adapterCommand(ctx, "stop")
}

// AdapterName is the file the forge's own copy of the adapter lives in, inside
// the forge root it serves.
const AdapterName = "matrix-bridge.sh"

// bridgeConfigName is the configuration the adapter works on.
const bridgeConfigName = "matrix-bridge.json"

// adapterEnv names the bridge binary the adapter is pointed at: the forge root
// a scenario serves has no bridge checkout of its own.
const adapterBinaryEnv = "MATRIX_BRIDGE_BINARY"

// servedRoot is the fixture forge root whose configuration and adapter the
// scenario is working with.
func (w *World) servedRootRoot() (string, error) {
	if w.servedRoot == "" {
		return "", fmt.Errorf("the scenario never named the forge root the adapter serves")
	}
	return w.servedRoot, nil
}

// bridgeConfigOf is the configuration the adapter of one forge root works on.
func (w *World) bridgeConfigOf(root string) string {
	return filepath.Join(root, ".swarmforge", bridgeConfigName)
}

// stateDirOf is where a bridge configuration keeps its state.
func (w *World) stateDirOf(root string) string {
	dir := filepath.Join(root, ".swarmforge", "matrix-bridge")
	if w.stateDir != "" && w.servedRoot == root {
		return w.stateDir
	}
	return dir
}

// configureForAdapter writes a bridge configuration for one forge root the way
// the operator's own deployment holds it: the root of the forge it serves, the
// operator, and the bridge's own login.
func configureForAdapter(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	named, err := w.declaredForge(captures[2])
	if err != nil {
		return err
	}
	synapse, err := w.homeserver(ctx)
	if err != nil {
		return err
	}
	if err := w.configureForge(captures[1]); err != nil {
		return err
	}
	root := served.Root()
	stateDir := w.stateDirOf(root)
	config := map[string]any{
		"homeserver_url": synapse.URL,
		"user_id":        w.bridgeUserID,
		"password":       w.bridgePassword,
		"operator":       w.operatorID,
		"forges":         []map[string]string{{"root": named.Root()}},
		"state_dir":      stateDir,
	}
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(w.bridgeConfigOf(root), append(body, '\n'), 0o644); err != nil {
		return err
	}
	w.servedRoot = root
	w.stateDir = stateDir
	w.configPath = w.bridgeConfigOf(root)
	return nil
}

// installAdapter gives the served forge root its own copy of the adapter, the
// way the operator installs it: the adapter is invoked where the forge keeps
// it, not from the bridge's repository.
func (w *World) installAdapter() (string, error) {
	root, err := w.servedRootRoot()
	if err != nil {
		return "", err
	}
	source := filepath.Join(fixtures.ProjectRoot(), "scripts", AdapterName)
	content, err := os.ReadFile(source)
	if err != nil {
		return "", fmt.Errorf("the adapter the repository ships is missing: %w", err)
	}
	installed := filepath.Join(root, "swarmforge", "scripts", AdapterName)
	if err := os.MkdirAll(filepath.Dir(installed), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(installed, content, 0o755); err != nil {
		return "", err
	}
	return installed, nil
}

// adapterCommand runs the served forge root's own adapter. It is run from
// wherever the scenario happens to be, the way an operator runs the copy the
// forge keeps rather than the bridge's own repository.
func (w *World) adapterCommand(ctx context.Context, args ...string) (string, error) {
	installed, err := w.installAdapter()
	if err != nil {
		return "", err
	}
	binary, err := w.bridgeBinary()
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, installed, args...)
	command.Dir = fixtures.ProjectRoot()
	command.Env = append(os.Environ(), adapterBinaryEnv+"="+binary)
	out, runErr := command.CombinedOutput()
	w.adapterOutput = string(out)
	return w.adapterOutput, runErr
}

// adapterStartedBridge starts the bridge the way the adapter does, and waits
// for it to report a tick.
func adapterStartedBridge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	w.servedRoot = served.Root()
	if _, err := w.adapterCommand(ctx, "start"); err != nil {
		return err
	}
	path := filepath.Join(w.stateDirOf(served.Root()), bridge.StatusName)
	return waitFor(ctx, "the bridge the adapter started never reported a tick", func() (bool, error) {
		_, err := readStatus(path)
		return err == nil, nil
	})
}

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

// keptConfiguration is the copy of the configuration the adapter kept.
func (w *World) keptConfiguration() (string, error) {
	root, err := w.servedRootRoot()
	if err != nil {
		return "", err
	}
	kept, err := filepath.Glob(w.bridgeConfigOf(root) + ".replaced-*")
	if err != nil {
		return "", err
	}
	if len(kept) == 0 {
		return "", fmt.Errorf("the adapter kept no copy of the configuration it replaced")
	}
	return kept[0], nil
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

// readForgeEntries is the forges one bridge configuration names.
func readForgeEntries(path string) ([]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config struct {
		Forges []map[string]string `json:"forges"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return config.Forges, nil
}

// adapterRefused checks the adapter refused the new forge.
func adapterRefused(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if !strings.Contains(w.adapterOutput, "refused") {
		return fmt.Errorf("the adapter did not refuse the new forge:\n%s", w.adapterOutput)
	}
	return nil
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
	name := captures[1]
	return waitFor(ctx, fmt.Sprintf("the bridge's status never named %s as one it reached", name), func() (bool, error) {
		status, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
		if err != nil {
			return false, nil
		}
		return fixtures.Contains(status.ReachedForges, name), nil
	})
}

// statusNamesTheForgeAsUnreached waits for the bridge's status to name one
// forge as one it did not reach.
func statusNamesTheForgeAsUnreached(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	name := captures[1]
	return waitFor(ctx, fmt.Sprintf("the bridge's status never named %s as one it did not reach", name), func() (bool, error) {
		status, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
		if err != nil {
			return false, nil
		}
		return fixtures.Contains(status.UnhappyForges, name), nil
	})
}

// configuredName is the name the bridge knows a configured forge by: the name
// the configuration gives it, otherwise the folder it lives in.
func configuredName(entry map[string]string) string {
	if name := strings.TrimSpace(entry["name"]); name != "" {
		return name
	}
	return filepath.Base(strings.TrimRight(entry["root"], string(filepath.Separator)))
}
