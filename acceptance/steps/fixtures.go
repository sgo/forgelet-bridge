package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// homeserver starts the scenario's Synapse on first use.
func (w *World) homeserver(ctx context.Context) (*fixtures.Synapse, error) {
	if w.synapse != nil {
		return w.synapse, nil
	}
	synapse, err := fixtures.StartSynapse(ctx, filepath.Join(w.workDir, "synapse"))
	if err != nil {
		return nil, err
	}
	w.synapse = synapse

	userID, _, _, err := synapse.Register(ctx, "bridge", bridgePassword)
	if err != nil {
		return nil, err
	}
	w.bridgeUserID = userID
	w.bridgePassword = bridgePassword
	return synapse, nil
}

// forge returns the fixture forge root and dashboard queue of a forge name,
// starting its dashboard stub the first time it is asked for.
func (w *World) forge(ctx context.Context, name string) (*dashboard.Store, error) {
	if store, ok := w.dashboards[name]; ok {
		return store, nil
	}
	root := filepath.Join(w.workDir, name)
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge", "dashboard", "requests", "pending"), 0o755); err != nil {
		return nil, err
	}
	store := dashboard.New(root)
	w.dashboards[name] = store
	w.forgeRoots = appendUnique(w.forgeRoots, root)
	return store, nil
}

// declaredForge is the dashboard queue of a fixture forge the scenario has
// already declared. A forge no fixture stands behind would run the bridge
// against a forge that is not there, so every step that names a forge asks
// this one question.
func (w *World) declaredForge(name string) (*dashboard.Store, error) {
	store, ok := w.dashboards[name]
	if !ok {
		return nil, fmt.Errorf("the fixture forge root %s does not have its dashboard running", name)
	}
	return store, nil
}

// configureForge records the root the bridge is configured with.
func (w *World) configureForge(name string) error {
	store, err := w.declaredForge(name)
	if err != nil {
		return err
	}
	w.configured = appendUnique(w.configured, store.Root())
	return nil
}

// nameForge records the name the operator knows a fixture forge by. The name
// has nothing to do with the folder the forge lives in.
func (w *World) nameForge(name, displayName string) error {
	if err := w.configureForge(name); err != nil {
		return err
	}
	if w.forgeNames == nil {
		w.forgeNames = map[string]string{}
	}
	w.forgeNames[filepath.Join(w.workDir, name)] = displayName
	return nil
}

// user registers and connects a fixture Matrix user on first use.
func (w *World) user(ctx context.Context, userID string) (*fixtures.User, error) {
	if user, ok := w.users[userID]; ok {
		return user, nil
	}
	synapse, err := w.homeserver(ctx)
	if err != nil {
		return nil, err
	}
	localpart, err := localpartOf(userID)
	if err != nil {
		return nil, err
	}
	user, err := fixtures.NewUser(ctx, synapse, localpart, filepath.Join(w.workDir, "clients", localpart))
	if err != nil {
		return nil, err
	}
	w.users[userID] = user
	return user, nil
}

func (w *World) operator(ctx context.Context) (*fixtures.User, error) {
	return w.user(ctx, w.operatorID)
}

// configure records the forge roots and operator the bridge is configured
// with, and writes the bridge configuration file.
func (w *World) configure(ctx context.Context) error {
	synapse, err := w.homeserver(ctx)
	if err != nil {
		return err
	}
	if len(w.configured) == 0 {
		return fmt.Errorf("no forge root has been configured")
	}
	configPath := filepath.Join(w.workDir, "bridge.json")
	stateDir := filepath.Join(w.workDir, "bridge-state")
	forges := make([]map[string]string, 0, len(w.configured))
	for _, root := range w.configured {
		forge := map[string]string{"root": root}
		if name := w.forgeNames[root]; name != "" {
			forge["name"] = name
		}
		forges = append(forges, forge)
	}
	body, err := json.MarshalIndent(map[string]any{
		"homeserver_url": synapse.URL,
		"user_id":        w.bridgeUserID,
		"password":       w.bridgePassword,
		"operator":       w.operatorID,
		"forges":         forges,
		"state_dir":      stateDir,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(w.workDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, append(body, '\n'), 0o644); err != nil {
		return err
	}
	w.configPath = configPath
	w.stateDir = stateDir
	return nil
}

// waitForOperator waits until the operator stand-in is in every forge the
// bridge was told about: the phone is on, so it accepts the invites the bridge
// sends. Relaying before that would leave messages nobody can decrypt, which is
// not what the features describe.
func (w *World) waitForOperator(ctx context.Context) error {
	want := len(w.configured) * 2 // the forge space and its chat room
	return waitFor(ctx, "the operator never joined the forge rooms", func() (bool, error) {
		operator, err := w.operator(ctx)
		if err != nil {
			return false, err
		}
		rooms, err := operator.JoinedRoomIDs(ctx)
		if err != nil {
			return false, err
		}
		return len(rooms) >= want, nil
	})
}
