package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// servedRootRoot is the fixture forge root whose configuration and adapter the
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

// configuredName is the name the bridge knows a configured forge by: the name
// the configuration gives it, otherwise the folder it lives in.
func configuredName(entry map[string]string) string {
	if name := strings.TrimSpace(entry["name"]); name != "" {
		return name
	}
	return filepath.Base(strings.TrimRight(entry["root"], string(filepath.Separator)))
}
