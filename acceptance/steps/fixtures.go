package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

func matrixFixtureBackend(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.matrixServer != nil {
		return fmt.Errorf("the Matrix fixture backend must be chosen before the homeserver starts")
	}
	w.matrixBackend = fixtures.MatrixBackend(captures[1])
	return nil
}

// homeserver starts the scenario's Matrix server on first use.
func (w *World) homeserver(ctx context.Context) (fixtures.Homeserver, error) {
	if w.matrixServer != nil {
		return w.matrixServer, nil
	}
	started := time.Now()
	var (
		server fixtures.Homeserver
		err    error
	)
	switch w.matrixBackend {
	case fixtures.MatrixBackendFresh:
		server, err = fixtures.StartSynapse(ctx, filepath.Join(w.workDir, "synapse"))
	case fixtures.MatrixBackendShared:
		server, err = fixtures.SharedSynapse(ctx, filepath.Join(fixtures.WorkDir(), "shared-synapse"))
	case fixtures.MatrixBackendMemory:
		server, err = fixtures.StartMemoryHomeserver(ctx)
	default:
		err = fmt.Errorf("unsupported Matrix fixture backend %q", w.matrixBackend)
	}
	if err != nil {
		return nil, err
	}
	w.matrixServer = server
	w.setupTime += time.Since(started)

	userID, token, _, err := server.Register(ctx, "bridge", bridgePassword)
	if err != nil {
		return nil, err
	}
	w.bridgeUserID = userID
	w.bridgePassword = bridgePassword
	w.bridgeToken = token
	return server, nil
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

// addUserToRoom brings a fixture client into a room the bridge only invited the
// operator to, the way the operator would add a friend in Element. The room is
// named in the failure the way the step that asked for it names it.
func (w *World) addUserToRoom(ctx context.Context, userID, roomID, description string) error {
	user, err := w.user(ctx, userID)
	if err != nil {
		return err
	}
	if rooms, err := user.Rooms(ctx); err == nil && fixtures.Contains(rooms, roomID) {
		return nil
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	if err := operator.Invite(ctx, roomID, userID); err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("%s never joined %s", userID, description), func() (bool, error) {
		joined, err := user.JoinedRoomIDs(ctx)
		if err != nil {
			return false, err
		}
		return fixtures.Contains(joined, roomID), nil
	})
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
	config := map[string]any{
		"homeserver_url": synapse.BaseURL(),
		"user_id":        w.bridgeUserID,
		"password":       w.bridgePassword,
		"operator":       w.operatorID,
		"forges":         forges,
		"state_dir":      stateDir,
	}
	if w.matrixBackend == fixtures.MatrixBackendShared {
		config["device_id"] = fmt.Sprintf("FORGELETBRIDGE%d", time.Now().UnixNano())
	}
	body, err := json.MarshalIndent(config, "", "  ")
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
