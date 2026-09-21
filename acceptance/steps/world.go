// Package steps connects the acceptance features to the outside world: the
// fixture forges, the bridge process, the homeserver the tests start
// themselves, and the Matrix client standing in for the operator's phone.
package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/acceptance/runtime"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// BinaryEnv points the steps at a prebuilt bridge binary.
const BinaryEnv = "FORGELET_BRIDGE_BIN"

// DefaultOperator is the operator the features configure.
const DefaultOperator = "@operator:" + fixtures.ServerName

const stepTimeout = 60 * time.Second

// bridgePassword is the fixture bridge user's password; the bridge logs in
// with it on every start and keeps the same device.
const bridgePassword = "bridge-password"

// World is one scenario execution's fixtures. Every execution gets a fresh
// homeserver, fresh forge roots, and a fresh bridge, so scenarios cannot leak
// into each other.
type World struct {
	workDir string
	log     *slog.Logger

	synapse        *fixtures.Synapse
	bridgeUserID   string
	bridgePassword string

	configPath string
	stateDir   string
	forgeRoots []string
	configured []string
	operatorID string

	users      map[string]*fixtures.User
	dashboards map[string]*dashboard.Store
	stubs      map[string]*stub
	bridge     *bridgeProcess
	binaryPath string
	anchors    map[string]string
}

type stub struct {
	cmd *exec.Cmd
	log *os.File
}

type bridgeProcess struct {
	cmd *exec.Cmd
	log *os.File
}

// Registry builds the acceptance registry.
func Registry() *runtime.Registry {
	registry := runtime.NewRegistry(func() any { return newWorld() })
	registry.OnClose(func(world any) { world.(*World).Close() })
	if err := register(registry); err != nil {
		panic(err)
	}
	return registry
}

func newWorld() *World {
	workDir := filepath.Join(fixtures.WorkDir(), fmt.Sprintf("scenario-%d", time.Now().UnixNano()))
	return &World{
		workDir:    workDir,
		log:        slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		forgeRoots: nil,
		operatorID: DefaultOperator,
		users:      map[string]*fixtures.User{},
		dashboards: map[string]*dashboard.Store{},
		stubs:      map[string]*stub{},
		anchors:    map[string]string{},
	}
}

// Close stops everything the scenario started.
func (w *World) Close() {
	w.stopBridge()
	for _, running := range w.stubs {
		stopProcess(running.cmd, running.log)
	}
	for _, user := range w.users {
		_ = user.Close()
	}
	if w.synapse != nil {
		w.synapse.Stop()
	}
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), stepTimeout)
}

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

// configureForge records the root the bridge is configured with.
func (w *World) configureForge(ctx context.Context, name string) error {
	if _, err := w.forge(ctx, name); err != nil {
		return err
	}
	root := filepath.Join(w.workDir, name)
	w.configured = appendUnique(w.configured, root)
	return nil
}

// startDashboard runs the forge's dashboard stub.
func (w *World) startDashboard(name string) error {
	if _, ok := w.stubs[name]; ok {
		return nil
	}
	store, err := w.forge(context.Background(), name)
	if err != nil {
		return err
	}
	binary, err := buildHelper("forge-dashboard-stub", "./cmd/forge-dashboard-stub")
	if err != nil {
		return err
	}
	logFile, err := os.Create(filepath.Join(w.workDir, name+"-dashboard.log"))
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, "--root", store.Root())
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	w.stubs[name] = &stub{cmd: cmd, log: logFile}
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
	body, err := json.MarshalIndent(map[string]any{
		"homeserver_url": synapse.URL,
		"user_id":        w.bridgeUserID,
		"password":       w.bridgePassword,
		"operator":       w.operatorID,
		"forge_roots":    w.configured,
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

// startBridge runs the bridge process.
func (w *World) startBridge(ctx context.Context) error {
	if w.bridge != nil {
		return nil
	}
	if w.configPath == "" {
		if err := w.configure(ctx); err != nil {
			return err
		}
	}
	binary, err := w.bridgeBinary()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(w.workDir, "bridge.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, "--config", w.configPath, "--interval", "200ms")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	w.bridge = &bridgeProcess{cmd: cmd, log: logFile}
	return w.waitForOperator(ctx)
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

// waitForSpaceChild waits for a chat room of a given name inside a forge space
// the operator can see and is in.
func (w *World) waitForSpaceChild(ctx context.Context, spaceName, childName string) (string, error) {
	var found string
	err := waitFor(ctx, fmt.Sprintf("the forge space %s never showed a chat room %s", spaceName, childName), func() (bool, error) {
		spaces, err := w.spaces(ctx, spaceName)
		if err != nil {
			return false, err
		}
		operator, err := w.operator(ctx)
		if err != nil {
			return false, err
		}
		for _, spaceID := range spaces {
			children, err := operator.SpaceChildren(ctx, spaceID)
			if err != nil {
				continue
			}
			for _, child := range children {
				name, err := operator.RoomName(ctx, child)
				if err != nil || name != childName {
					continue
				}
				membership, err := operator.Membership(ctx, child, w.operatorID)
				if err != nil || membership != "join" {
					continue
				}
				found = child
				return true, nil
			}
		}
		return false, nil
	})
	return found, err
}

func (w *World) stopBridge() {
	if w.bridge == nil {
		return
	}
	stopProcess(w.bridge.cmd, w.bridge.log)
	w.bridge = nil
}

// caughtUp waits for the bridge to finish a tick with nothing left to do.
func (w *World) caughtUp(ctx context.Context) error {
	path := filepath.Join(w.stateDir, "status.json")
	before, _ := readStatus(path)
	for {
		status, err := readStatus(path)
		if err == nil && status.Tick > before.Tick && status.Idle {
			return nil
		}
		if err := sleep(ctx, 100*time.Millisecond); err != nil {
			return fmt.Errorf("the bridge never caught up: %w", err)
		}
	}
}

type status struct {
	Tick uint64 `json:"tick"`
	Idle bool   `json:"idle"`
}

func readStatus(path string) (status, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return status{}, err
	}
	var parsed status
	if err := json.Unmarshal(data, &parsed); err != nil {
		return status{}, err
	}
	return parsed, nil
}

// chatRoom finds the one chat room of a given name the operator is in.
func (w *World) chatRoom(ctx context.Context, name string) (string, error) {
	rooms, err := w.rooms(ctx, name)
	if err != nil {
		return "", err
	}
	if len(rooms) == 0 {
		return "", fmt.Errorf("the operator never saw a chat room named %s", name)
	}
	if len(rooms) > 1 {
		return "", fmt.Errorf("the operator sees %d chat rooms named %s: %v", len(rooms), name, rooms)
	}
	return rooms[0], nil
}

// rooms lists the rooms the operator has with a given name, whether joined or
// still only invited.
func (w *World) rooms(ctx context.Context, name string) ([]string, error) {
	deadline := time.Now().Add(stepTimeout)
	for {
		operator, err := w.operator(ctx)
		if err != nil {
			return nil, err
		}
		candidates, err := operator.Rooms(ctx)
		if err != nil {
			return nil, err
		}
		var found []string
		for _, roomID := range candidates {
			roomName, err := operator.RoomName(ctx, roomID)
			if err != nil || roomName != name {
				continue
			}
			found = append(found, roomID)
		}
		if len(found) > 0 || time.Now().After(deadline) {
			return found, nil
		}
		if err := sleep(ctx, 100*time.Millisecond); err != nil {
			return nil, err
		}
	}
}

// spaces lists the spaces of a given name the operator knows about.
func (w *World) spaces(ctx context.Context, name string) ([]string, error) {
	deadline := time.Now().Add(stepTimeout)
	for {
		operator, err := w.operator(ctx)
		if err != nil {
			return nil, err
		}
		candidates, err := operator.Rooms(ctx)
		if err != nil {
			return nil, err
		}
		var found []string
		for _, roomID := range candidates {
			isSpace, err := operator.IsSpace(ctx, roomID)
			if err != nil || !isSpace {
				continue
			}
			if name != "" {
				roomName, err := operator.RoomName(ctx, roomID)
				if err != nil || roomName != name {
					continue
				}
			}
			found = append(found, roomID)
		}
		if len(found) > 0 || time.Now().After(deadline) {
			return found, nil
		}
		if err := sleep(ctx, 100*time.Millisecond); err != nil {
			return nil, err
		}
	}
}

// allSpaces lists every space the operator knows about.
func (w *World) allSpaces(ctx context.Context) ([]string, error) {
	return w.spaces(ctx, "")
}

func (w *World) bridgeBinary() (string, error) {
	if w.binaryPath != "" {
		return w.binaryPath, nil
	}
	binary, err := buildHelper("forgelet-bridge", "./cmd/forgelet-bridge")
	if err != nil {
		return "", err
	}
	w.binaryPath = binary
	return binary, nil
}

var (
	buildMu     sync.Mutex
	builtBinary = map[string]string{}
)

// buildHelper builds a command once per test process and returns its path.
func buildHelper(name, pkg string) (string, error) {
	buildMu.Lock()
	defer buildMu.Unlock()
	if path, ok := builtBinary[name]; ok {
		return path, nil
	}
	path := filepath.Join(fixtures.ProjectRoot(), "build", "acceptance", "bin", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("go", "build", "-tags", "goolm", "-o", path, pkg)
	cmd.Dir = fixtures.ProjectRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build %s: %w: %s", name, err, out)
	}
	builtBinary[name] = path
	return path, nil
}

// waitFor polls until check succeeds or the context runs out.
func waitFor(ctx context.Context, describe string, check func() (bool, error)) error {
	for {
		ok, err := check()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if err := sleep(ctx, 100*time.Millisecond); err != nil {
			return fmt.Errorf("%s: %w", describe, err)
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func stopProcess(cmd *exec.Cmd, logFile *os.File) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
	if logFile != nil {
		logFile.Close()
	}
}

func localpartOf(userID string) (string, error) {
	trimmed := strings.TrimPrefix(userID, "@")
	localpart, _, found := strings.Cut(trimmed, ":")
	if !found || localpart == "" {
		return "", fmt.Errorf("not a matrix user id: %q", userID)
	}
	return localpart, nil
}

func appendUnique(list []string, value string) []string {
	for _, item := range list {
		if item == value {
			return list
		}
	}
	return append(list, value)
}

// forgeNames parses a Gherkin list like "forge-a, forge-b" or "forge-a".
func forgeNames(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ' '
	})
	var names []string
	for _, field := range fields {
		switch field {
		case "", "and":
			continue
		}
		names = append(names, field)
	}
	return names
}
