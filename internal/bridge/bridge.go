// Package bridge keeps one or more forges' chat channels in step with their
// Matrix rooms: forge requests become messages, operator replies become chat
// requests, and answers arrive as thread replies.
package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

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
	for _, forge := range b.cfg.Forges {
		root := forge.Root
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T12:38:16+02:00","module_hash":"3194cb9d63d977711a0b534f35933e8f6f9a8ef3ef288af351ea65fbc3c61015","functions":[{"id":"func/New","name":"New","line":54,"end_line":73,"hash":"f3657729186b52d630853c6fd6aa4164e98b6fde9fb4fa444db6c050cdad49a6"},{"id":"func/Bridge.State","name":"Bridge.State","line":76,"end_line":78,"hash":"bf410b1bb8d53a98c0174a9807b8510a29bc02200bb47d241ad7e9f14ff2f7aa"},{"id":"func/Bridge.Run","name":"Bridge.Run","line":81,"end_line":100,"hash":"b8772f032b4a90a982599b8cabb97e5219cc7d6e4f681adba897b8427dd4f462"},{"id":"func/Bridge.Tick","name":"Bridge.Tick","line":104,"end_line":145,"hash":"c430c7418f4c191089e200b5a14cec7ab20f34a52cca4e9518f18852d46276d1"},{"id":"func/Bridge.apply","name":"Bridge.apply","line":147,"end_line":152,"hash":"1be31d1a552df16d85d903a2d19730345bef233dc0c774af992cf9771631039e"},{"id":"func/Bridge.carryOut","name":"Bridge.carryOut","line":155,"end_line":165,"hash":"506f097a3daf97bea3e2e32cf7db5ff88bc6bd556a181d7dc6d47400b0bc6374"}]}
// mutate4go-manifest-end
