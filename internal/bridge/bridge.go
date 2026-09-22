// Package bridge keeps one or more forges in step with their Matrix rooms:
// forge requests become chat messages, operator replies become chat requests,
// answers arrive as thread replies, and approvals reach the operator's phone
// where a reaction or a reply decides them.
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
	// CreateRequest hands a chat request to the forge, which is what wakes the
	// lieutenant. The forge answers that it took the message, not which
	// request it became.
	CreateRequest(ctx context.Context, body string) (string, error)
}

// ApprovalStore is the forge side of one root's approvals: the handoffs its
// projects are waiting for.
type ApprovalStore interface {
	Pending(ctx context.Context) ([]relay.Approval, error)
	Approve(ctx context.Context, project, id string) error
	SendBack(ctx context.Context, project, id, feedback string) error
}

// BoardStore is the forge side of one root's boards: the cards its projects
// hold and the lanes they are in.
type BoardStore interface {
	Cards() ([]relay.Card, error)
}

// Room is the Matrix side the bridge created for a forge.
type Room struct {
	SpaceID         string
	RoomID          string
	ApprovalsRoomID string
	ActivityRoomID  string
}

// Rooms is the Matrix side of the bridge: spaces, chat rooms, and the messages
// the operator has sent.
type Rooms interface {
	EnsureForge(ctx context.Context, forgeName, operator string) (Room, error)
	SendText(ctx context.Context, roomID, body, threadAnchor string) (string, error)
	DrainEvents(ctx context.Context) ([]relay.RoomEvent, error)
	DrainReactions(ctx context.Context) ([]relay.Reaction, error)
}

// Bridge is the running relay.
type Bridge struct {
	cfg         config.Config
	rooms       Rooms
	stores      map[string]ForgeStore
	approvals   map[string]ApprovalStore
	boards      map[string]BoardStore
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
func New(cfg config.Config, rooms Rooms, stores map[string]ForgeStore, approvals map[string]ApprovalStore, boards map[string]BoardStore, log *slog.Logger) (*Bridge, error) {
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
		approvals:   approvals,
		boards:      boards,
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
	wait := interval
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			if err := b.Tick(ctx); err != nil {
				b.log.Error("tick failed", "error", err)
				wait = backOff(wait, interval)
				continue
			}
			wait = interval
		}
	}
}

// backOff spaces out retries after a failed tick, so a homeserver that is
// throttling or away is not hammered: the wait doubles, never drops below the
// interval the bridge was told to tick at, and never passes its ceiling.
func backOff(wait, interval time.Duration) time.Duration {
	return min(max(wait*2, interval), maxBackOff)
}

// maxBackOff is how long the bridge waits at most between tries.
const maxBackOff = 30 * time.Second

// Tick carries out one round of work: catch the rooms up with the forges and
// the forges up with the rooms.
func (b *Bridge) Tick(ctx context.Context) error {
	seen, err := b.drainRooms(ctx)
	if err != nil {
		return err
	}

	carriedOut := 0
	for _, forge := range b.cfg.Forges {
		done, err := b.tickForge(ctx, forge.Root, seen)
		if err != nil {
			return err
		}
		carriedOut += done
	}

	b.tick++
	return b.writeStatus(Status{
		Tick:              b.tick,
		Idle:              carriedOut == 0,
		DeviceID:          b.device.ID,
		DeviceFingerprint: b.device.Fingerprint,
	})
}

// roomEvents is what the Matrix side has said since the last tick, grouped by
// the room it was said in.
type roomEvents struct {
	messages  map[string][]relay.RoomEvent
	reactions map[string][]relay.Reaction
}

// drainRooms reads what the rooms have seen since the last tick.
func (b *Bridge) drainRooms(ctx context.Context) (roomEvents, error) {
	messages, err := b.rooms.DrainEvents(ctx)
	if err != nil {
		return roomEvents{}, fmt.Errorf("read chat room events: %w", err)
	}
	reactions, err := b.rooms.DrainReactions(ctx)
	if err != nil {
		return roomEvents{}, fmt.Errorf("read approvals room reactions: %w", err)
	}

	seen := roomEvents{messages: map[string][]relay.RoomEvent{}, reactions: map[string][]relay.Reaction{}}
	for _, message := range messages {
		seen.messages[message.RoomID] = append(seen.messages[message.RoomID], message)
	}
	for _, reaction := range reactions {
		seen.reactions[reaction.RoomID] = append(seen.reactions[reaction.RoomID], reaction)
	}
	return seen, nil
}

// tickForge catches one forge's rooms up with the forge and the forge up with
// its rooms, and reports how much work it carried out.
func (b *Bridge) tickForge(ctx context.Context, root string, seen roomEvents) (int, error) {
	room, err := b.roomFor(ctx, root)
	if err != nil {
		return 0, err
	}
	store, ok := b.stores[root]
	if !ok {
		return 0, fmt.Errorf("no dashboard queue configured for forge root %s", root)
	}
	requests, err := store.Requests()
	if err != nil {
		return 0, fmt.Errorf("read dashboard requests for %s: %w", root, err)
	}

	carriedOut := 0
	for _, action := range relay.Plan(b.cfg.Operator, b.state.Relay, requests, seen.messages[room.RoomID]) {
		if err := b.apply(ctx, root, store, room, action); err != nil {
			return 0, err
		}
		carriedOut++
	}

	paired, err := b.pairPendingRequests(store)
	if err != nil {
		return 0, err
	}
	carriedOut += paired

	approvals, err := b.carryOutApprovals(ctx, root, room, seen.messages[room.ApprovalsRoomID], seen.reactions[room.ApprovalsRoomID])
	if err != nil {
		return 0, err
	}

	activity, err := b.carryOutActivity(ctx, root, room)
	if err != nil {
		return 0, err
	}
	return carriedOut + approvals + activity, nil
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
		return b.createForgeRequest(ctx, store, root, action)
	}
	return fmt.Errorf("unknown relay action %q", action.Kind)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T16:14:31+02:00","module_hash":"8cde7f821ad7998e5851a7228127c535cafb30e76c0104020f7e42fad420e292","functions":[{"id":"func/New","name":"New","line":74,"end_line":95,"hash":"19816dc089b747fe932a6bba03d6598394cf98e00064c7886cfcfc206630edda"},{"id":"func/Bridge.State","name":"Bridge.State","line":98,"end_line":100,"hash":"bf410b1bb8d53a98c0174a9807b8510a29bc02200bb47d241ad7e9f14ff2f7aa"},{"id":"func/Bridge.Run","name":"Bridge.Run","line":103,"end_line":124,"hash":"8f3f629a65f21167539ddf1f5f571a073f55caec778b2ce61c1d37026e0dc233"},{"id":"func/backOff","name":"backOff","line":129,"end_line":131,"hash":"313dab7f39c47410d8e18174342f6c613f4c66b3081679d7099f2bf342b7a010"},{"id":"func/Bridge.Tick","name":"Bridge.Tick","line":138,"end_line":160,"hash":"5c9eb6c2be4c4793a314ff71034c5278fc243861ff92ff8d8f98e1a74f6b733a"},{"id":"func/Bridge.drainRooms","name":"Bridge.drainRooms","line":170,"end_line":188,"hash":"9bb5f3184873715385e70794087854cbdd08c8af11843ecd660f1b8902e100dd"},{"id":"func/Bridge.tickForge","name":"Bridge.tickForge","line":192,"end_line":224,"hash":"ad87f1bbd41f9c7ccd31ae6dec75d9cafa8530c6dbf4638e764899686083ad1c"},{"id":"func/Bridge.apply","name":"Bridge.apply","line":226,"end_line":231,"hash":"1be31d1a552df16d85d903a2d19730345bef233dc0c774af992cf9771631039e"},{"id":"func/Bridge.carryOut","name":"Bridge.carryOut","line":234,"end_line":244,"hash":"506f097a3daf97bea3e2e32cf7db5ff88bc6bd556a181d7dc6d47400b0bc6374"}]}
// mutate4go-manifest-end
