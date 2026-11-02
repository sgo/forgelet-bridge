package steps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/state"
)

func (w *World) waitForRemovalForge(ctx context.Context, root string) (*state.State, error) {
	var loaded *state.State
	err := waitFor(ctx, "the bridge never recorded the forge's rooms", func() (bool, error) {
		var err error
		loaded, err = state.Load(w.removalStatePath())
		if err != nil {
			return false, err
		}
		forge, ok := loaded.Forges[root]
		return ok && forge.SpaceID != "" && forge.RoomID != "" && forge.ApprovalsRoomID != "" &&
			forge.ActivityRoomID != "" && forge.ClarificationsRoomID != "", nil
	})
	return loaded, err
}

func (w *World) invokeRemoval(servedName, targetRoot string) error {
	return w.invokeRemovalArgs(servedName, targetRoot)
}

func (w *World) invokeRemovalArgs(servedName string, args ...string) error {
	ctx, cancel := stepContext()
	defer cancel()
	served, err := w.declaredForge(servedName)
	if err != nil {
		return err
	}
	if w.removal == nil {
		target, err := w.declaredForge("forge-b")
		if err != nil {
			return err
		}
		w.removal = &removalScenario{target: target.Root()}
	}
	w.removal.stateBefore, err = os.ReadFile(w.removalStatePath())
	if err != nil {
		return err
	}
	w.removal.configBefore, err = os.ReadFile(w.bridgeConfigOf(served.Root()))
	if err != nil {
		return err
	}
	w.removal.targetBefore, err = snapshotRemovalTree(w.removal.target)
	if err != nil {
		return err
	}
	w.removal.roomsBefore = append([]string(nil), w.removal.rooms...)
	w.removal.operationDone = false

	_, runErr := w.adapterCommand(ctx, append([]string{"remove-forge"}, args...)...)
	if runErr != nil {
		return runErr
	}
	var report removalReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(w.adapterStdout)), &report); err != nil {
		return fmt.Errorf("removal did not return one JSON object: %q: %w", w.adapterStdout, err)
	}
	w.removal.status = report.Status
	w.removal.reason = report.Reason
	w.removal.operationDone = true
	return nil
}

func (w *World) removalStatePath() string {
	return filepath.Join(w.stateDir, "bridge-state.json")
}

func (w *World) removalProjectRoom() (string, error) {
	if w.removal == nil {
		return "", fmt.Errorf("no removal scenario")
	}
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return "", err
	}
	forge := loaded.Forges[w.removal.target]
	project, ok := forge.Projects[w.removal.project]
	if !ok || project.RoomID == "" {
		return "", fmt.Errorf("the removal scenario has no recorded project room")
	}
	return project.RoomID, nil
}

func (w *World) removalRoomsBefore() []string {
	if w.removal == nil {
		return nil
	}
	return w.removal.roomsBefore
}

func (w *World) duplicateConfiguredBasename(servedRoot, basename string) error {
	cfg := map[string]any{
		"homeserver_url": w.synapse.URL,
		"user_id":        w.bridgeUserID,
		"password":       w.bridgePassword,
		"operator":       w.operatorID,
		"forges": []map[string]string{
			{"root": servedRoot},
			{"root": filepath.Join(w.workDir, "same-one", basename)},
			{"root": filepath.Join(w.workDir, "same-two", basename)},
		},
		"state_dir": w.stateDir,
	}
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(w.bridgeConfigOf(servedRoot), append(body, '\n'), 0o644)
}

func snapshotRemovalTree(root string) (map[string]string, error) {
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[relative] = hex.EncodeToString(sum[:])
		return nil
	})
	return snapshot, err
}

func sameSnapshot(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, hash := range left {
		if right[path] != hash {
			return false
		}
	}
	return true
}
