// Package bridge keeps one or more forges' chat channels in step with their
// Matrix rooms: forge requests become messages, operator replies become chat
// requests, and answers arrive as thread replies.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// StatusName is the file the bridge keeps its progress in, so that an operator
// or a test can tell when it has caught up.
const StatusName = "status.json"

// ForgeStore is the forge side of one root: its dashboard chat-request queue.
type ForgeStore interface {
	Requests() ([]relay.Request, error)
	CreateRequest(body string) (string, error)
}

// Room is the Matrix side the bridge created for a forge.
type Room struct {
	SpaceID string
	RoomID  string
}

// Rooms is the Matrix side of the bridge: spaces, chat rooms, and the messages
// the operator has sent.
type Rooms interface {
	EnsureForge(ctx context.Context, forgeName, operator string) (Room, error)
	SendText(ctx context.Context, roomID, body, threadAnchor string) (string, error)
	DrainEvents(ctx context.Context) ([]relay.RoomEvent, error)
}

// Status is what the bridge writes after every tick. The device fields tell
// the operator (and the acceptance suite) which Matrix device the bridge is
// using, so a restart that changes it is visible instead of silent.
type Status struct {
	Tick              uint64 `json:"tick"`
	Idle              bool   `json:"idle"`
	DeviceID          string `json:"device_id,omitempty"`
	DeviceFingerprint string `json:"device_fingerprint,omitempty"`
}

// Device is the Matrix device the bridge is using.
type Device struct {
	ID          string
	Fingerprint string
}

// Bridge is the running relay.
type Bridge struct {
	cfg         config.Config
	rooms       Rooms
	stores      map[string]ForgeStore
	statePath   string
	statusDir   string
	state       *state.State
	log         *slog.Logger
	tick        uint64
	provisioned map[string]Room
	device      Device
}

// New builds a bridge around a Matrix client and one dashboard queue per forge
// root. The state file keeps restarts from repeating work.
func New(cfg config.Config, rooms Rooms, stores map[string]ForgeStore, log *slog.Logger) (*Bridge, error) {
	if log == nil {
		log = slog.Default()
	}
	statePath := filepath.Join(cfg.StateDir, "bridge-state.json")
	loaded, err := state.Load(statePath)
	if err != nil {
		return nil, fmt.Errorf("load bridge state: %w", err)
	}
	return &Bridge{
		cfg:         cfg,
		rooms:       rooms,
		stores:      stores,
		statePath:   statePath,
		statusDir:   cfg.StateDir,
		state:       loaded,
		log:         log,
		provisioned: map[string]Room{},
	}, nil
}

// State exposes the bridge's bookkeeping for tests and status reporting.
func (b *Bridge) State() *state.State {
	return b.state
}

// ReportDevice records the Matrix device the bridge is using, so that its
// status shows which device the operator is talking to.
func (b *Bridge) ReportDevice(device Device) {
	b.device = device
}

// Run serves every configured forge until the context is cancelled.
func (b *Bridge) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	if err := b.Tick(ctx); err != nil {
		b.log.Error("tick failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := b.Tick(ctx); err != nil {
				b.log.Error("tick failed", "error", err)
			}
		}
	}
}

// Tick carries out one round of work: catch the rooms up with the forges and
// the forges up with the rooms.
func (b *Bridge) Tick(ctx context.Context) error {
	events, err := b.rooms.DrainEvents(ctx)
	if err != nil {
		return fmt.Errorf("read chat room events: %w", err)
	}
	byRoom := map[string][]relay.RoomEvent{}
	for _, event := range events {
		byRoom[event.RoomID] = append(byRoom[event.RoomID], event)
	}

	carriedOut := 0
	for _, root := range b.cfg.ForgeRoots {
		room, err := b.roomFor(ctx, root)
		if err != nil {
			return err
		}
		store, ok := b.stores[root]
		if !ok {
			return fmt.Errorf("no dashboard queue configured for forge root %s", root)
		}
		requests, err := store.Requests()
		if err != nil {
			return fmt.Errorf("read dashboard requests for %s: %w", root, err)
		}

		actions := relay.Plan(b.cfg.Operator, b.state.Relay, requests, byRoom[room.RoomID])
		for _, action := range actions {
			if err := b.apply(ctx, root, store, room, action); err != nil {
				return err
			}
			carriedOut++
		}
	}

	b.tick++
	return b.writeStatus(Status{
		Tick:              b.tick,
		Idle:              carriedOut == 0,
		DeviceID:          b.device.ID,
		DeviceFingerprint: b.device.Fingerprint,
	})
}

func (b *Bridge) apply(ctx context.Context, root string, store ForgeStore, room Room, action relay.Action) error {
	if err := b.carryOut(ctx, root, store, room, action); err != nil {
		return err
	}
	return b.state.Save(b.statePath)
}

// carryOut is the one piece of work an action asks for.
func (b *Bridge) carryOut(ctx context.Context, root string, store ForgeStore, room Room, action relay.Action) error {
	switch action.Kind {
	case relay.PostRequestMessage:
		return b.postRequestMessage(ctx, room, action)
	case relay.PostRequestReply:
		return b.postRequestReply(ctx, room, action)
	case relay.CreateForgeRequest:
		return b.createForgeRequest(store, root, action)
	}
	return fmt.Errorf("unknown relay action %q", action.Kind)
}

func (b *Bridge) writeStatus(status Status) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(b.statusDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.statusDir, StatusName), append(data, '\n'), 0o644)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-21T23:32:33+02:00","module_hash":"53a2201f7da06c13b95cfa9d9b5e76df7462cf1bb82a7c170ceeec9e37a035fd","functions":[{"id":"func/New","name":"New","line":65,"end_line":84,"hash":"f3657729186b52d630853c6fd6aa4164e98b6fde9fb4fa444db6c050cdad49a6"},{"id":"func/Bridge.State","name":"Bridge.State","line":87,"end_line":89,"hash":"bf410b1bb8d53a98c0174a9807b8510a29bc02200bb47d241ad7e9f14ff2f7aa"},{"id":"func/Bridge.Run","name":"Bridge.Run","line":92,"end_line":111,"hash":"b8772f032b4a90a982599b8cabb97e5219cc7d6e4f681adba897b8427dd4f462"},{"id":"func/Bridge.Tick","name":"Bridge.Tick","line":115,"end_line":151,"hash":"08dc22ad303541cd6c0d9c4011678d97acac4b20e75b749a74acc4cf9ea896a1"},{"id":"func/Bridge.apply","name":"Bridge.apply","line":153,"end_line":158,"hash":"1be31d1a552df16d85d903a2d19730345bef233dc0c774af992cf9771631039e"},{"id":"func/Bridge.carryOut","name":"Bridge.carryOut","line":161,"end_line":171,"hash":"506f097a3daf97bea3e2e32cf7db5ff88bc6bd556a181d7dc6d47400b0bc6374"},{"id":"func/Bridge.writeStatus","name":"Bridge.writeStatus","line":173,"end_line":182,"hash":"79a205547a958b3a5f9d1cc48f96e9549e6f42304bed9bf643585c7433bfa3dd"}]}
// mutate4go-manifest-end
