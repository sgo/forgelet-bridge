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

// Status is what the bridge writes after every tick.
type Status struct {
	Tick uint64 `json:"tick"`
	Idle bool   `json:"idle"`
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
	return b.writeStatus(Status{Tick: b.tick, Idle: carriedOut == 0})
}

func (b *Bridge) apply(ctx context.Context, root string, store ForgeStore, room Room, action relay.Action) error {
	switch action.Kind {
	case relay.PostRequestMessage:
		eventID, err := b.rooms.SendText(ctx, room.RoomID, action.Body, "")
		if err != nil {
			return fmt.Errorf("post chat message for %s: %w", action.RequestID, err)
		}
		b.state.Relay.Threads[action.RequestID] = eventID

	case relay.PostRequestReply:
		anchor := action.AnchorEventID
		if anchor == "" {
			anchor, _ = b.state.Relay.Anchor(action.RequestID)
		}
		if anchor == "" {
			return fmt.Errorf("no chat message to thread the answer of %s under", action.RequestID)
		}
		eventID, err := b.rooms.SendText(ctx, room.RoomID, action.Body, anchor)
		if err != nil {
			return fmt.Errorf("post chat reply for %s: %w", action.RequestID, err)
		}
		b.state.Relay.Replied[action.RequestID] = eventID

	case relay.CreateForgeRequest:
		requestID, err := store.CreateRequest(action.Body)
		if err != nil {
			return fmt.Errorf("queue chat request for %s: %w", root, err)
		}
		b.state.Relay.Relayed[action.SourceEventID] = requestID
		b.state.Relay.Threads[requestID] = action.SourceEventID

	default:
		return fmt.Errorf("unknown relay action %q", action.Kind)
	}

	return b.state.Save(b.statePath)
}

func (b *Bridge) roomFor(ctx context.Context, root string) (Room, error) {
	if room, ok := b.provisioned[root]; ok {
		return room, nil
	}
	if forge, ok := b.state.ForgeFor(root); ok {
		room := Room{SpaceID: forge.SpaceID, RoomID: forge.RoomID}
		b.provisioned[root] = room
		return room, nil
	}

	room, err := b.rooms.EnsureForge(ctx, config.ForgeName(root), b.cfg.Operator)
	if err != nil {
		return Room{}, fmt.Errorf("provision forge %s: %w", root, err)
	}
	b.state.RecordForge(root, state.Forge{SpaceID: room.SpaceID, RoomID: room.RoomID})
	if err := b.state.Save(b.statePath); err != nil {
		return Room{}, err
	}
	b.provisioned[root] = room
	b.log.Info("provisioned forge", "root", root, "space", room.SpaceID, "room", room.RoomID)
	return room, nil
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
