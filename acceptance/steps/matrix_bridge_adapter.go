package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
)

// AdapterName is the file the forge's own copy of the adapter lives in, inside
// the forge root it serves.
const AdapterName = "matrix-bridge.sh"

// bridgeConfigName is the configuration the adapter works on.
const bridgeConfigName = "matrix-bridge.json"

// adapterBinaryEnv names the bridge binary the adapter is pointed at: the forge
// root a scenario serves has no bridge checkout of its own. The rule installer
// and the rules this repository ships are pointed at the same way.
const (
	adapterBinaryEnv      = "MATRIX_BRIDGE_BINARY"
	adapterRulesBinaryEnv = "MATRIX_BRIDGE_RULES_BINARY"
	adapterRulesDirEnv    = "MATRIX_BRIDGE_RULES"
	adapterKitBinaryEnv   = "MATRIX_BRIDGE_KIT_BINARY"
	adapterKitDirEnv      = "MATRIX_BRIDGE_KIT"
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
	installer, err := buildHelper("install-rules", "./cmd/install-rules")
	if err != nil {
		return "", err
	}
	kitInstaller, err := buildHelper("install-kit", "./cmd/install-kit")
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, installed, args...)
	command.Dir = fixtures.ProjectRoot()
	kitDir := filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts")
	if w.kitDir != "" {
		kitDir = w.kitDir
	}
	command.Env = append(os.Environ(),
		adapterBinaryEnv+"="+binary,
		adapterRulesBinaryEnv+"="+installer,
		adapterRulesDirEnv+"="+filepath.Join(fixtures.ProjectRoot(), "rules"),
		adapterKitBinaryEnv+"="+kitInstaller,
		adapterKitDirEnv+"="+kitDir,
	)
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
