// Package steps connects the acceptance features to the outside world: the
// fixture forges, the bridge process, the homeserver the tests start
// themselves, and the Matrix client standing in for the operator's phone.
package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	// servedRoot is the forge root whose own adapter and bridge configuration
	// a scenario works with, and adapterOutput is what that adapter said.
	servedRoot    string
	adapterOutput string
	forgeRoots    []string
	configured    []string
	forgeNames    map[string]string
	operatorID    string

	users      map[string]*fixtures.User
	dashboards map[string]*dashboard.Store
	stubs      map[string]*child
	running    map[string]*fixtures.Dashboard
	bridge     *child
	binaryPath string
	anchors    map[string]string
	devices    []fixtures.DeviceKey
	lane       *laneState
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
		forgeNames: map[string]string{},
		operatorID: DefaultOperator,
		users:      map[string]*fixtures.User{},
		dashboards: map[string]*dashboard.Store{},
		stubs:      map[string]*child{},
		running:    map[string]*fixtures.Dashboard{},
		anchors:    map[string]string{},
	}
}

// Close stops everything the scenario started.
func (w *World) Close() {
	w.stopBridge()
	w.stopAdapterBridge()
	for _, running := range w.stubs {
		running.stop()
	}
	for _, dashboard := range w.running {
		dashboard.Stop()
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
