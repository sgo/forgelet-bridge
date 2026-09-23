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

// ClarificationStore is the forge side of one root's clarifications: the
// questions its agents are blocked on.
type ClarificationStore interface {
	Pending(ctx context.Context) ([]relay.Clarification, error)
	Answer(ctx context.Context, project, id, answer string) error
}

// BoardStore is the forge side of one root's boards: the cards its projects
// hold and the lanes they are in.
type BoardStore interface {
	Cards() ([]relay.Card, error)
}

// Room is the Matrix side the bridge created for a forge.
type Room struct {
	SpaceID              string
	RoomID               string
	ApprovalsRoomID      string
	ActivityRoomID       string
	ClarificationsRoomID string
}

// Rooms is the Matrix side of the bridge: spaces, chat rooms, and the messages
// the operator has sent.
type Rooms interface {
	EnsureForge(ctx context.Context, forgeName, operator string) (Room, error)
	// RefreshForge applies the forge's current name and the operator's
	// membership to rooms the bridge already has. What the configuration says
	// about a forge is true on every start, not only when the rooms are new.
	RefreshForge(ctx context.Context, room Room, forgeName, operator string) error
	SendText(ctx context.Context, roomID, body, threadAnchor string) (string, error)
	DrainEvents(ctx context.Context) ([]relay.RoomEvent, error)
	DrainReactions(ctx context.Context) ([]relay.Reaction, error)
}

// Bridge is the running relay.
type Bridge struct {
	cfg            config.Config
	rooms          Rooms
	stores         map[string]ForgeStore
	approvals      map[string]ApprovalStore
	clarifications map[string]ClarificationStore
	boards         map[string]BoardStore
	statePath      string
	statusDir      string
	state          *state.State
	log            *slog.Logger
	tick           uint64
	provisioned    map[string]Room
	device         Device
	// Work the forge refused, kept so it is tried again rather than lost.
	pending               map[string]*pendingChat
	pendingApprovals      map[string]*pendingApprovals
	pendingClarifications map[string]*pendingClarifications
	// unhappy is why each forge could not be served in the tick now running,
	// so one sick forge is named while the rest keep their rooms.
	unhappy map[string]string
}

// New builds a bridge around a Matrix client and one dashboard queue per forge
// root. The state file keeps restarts from repeating work.
func New(cfg config.Config, rooms Rooms, stores map[string]ForgeStore, approvals map[string]ApprovalStore, clarifications map[string]ClarificationStore, boards map[string]BoardStore, log *slog.Logger) (*Bridge, error) {
	if log == nil {
		log = slog.Default()
	}
	statePath := filepath.Join(cfg.StateDir, "bridge-state.json")
	loaded, err := state.Load(statePath)
	if err != nil {
		return nil, fmt.Errorf("load bridge state: %w", err)
	}
	return &Bridge{
		cfg:                   cfg,
		rooms:                 rooms,
		stores:                stores,
		approvals:             approvals,
		clarifications:        clarifications,
		boards:                boards,
		statePath:             statePath,
		statusDir:             cfg.StateDir,
		state:                 loaded,
		log:                   log,
		provisioned:           map[string]Room{},
		pending:               map[string]*pendingChat{},
		pendingApprovals:      map[string]*pendingApprovals{},
		pendingClarifications: map[string]*pendingClarifications{},
		unhappy:               map[string]string{},
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

	b.unhappy = map[string]string{}
	carriedOut := 0
	for _, forge := range b.cfg.Forges {
		done, err := b.tickForge(ctx, forge.Root, seen)
		if err != nil {
			// A forge the bridge cannot serve is that forge's problem: it is
			// reported and tried again on its own, and what the rooms said for
			// every other forge is still carried out.
			b.refuse(err, forge.Root)
			continue
		}
		carriedOut += done
	}

	b.tick++
	status := Status{
		Tick:              b.tick,
		Idle:              carriedOut == 0 && b.pendingCount() == 0,
		DeviceID:          b.device.ID,
		DeviceFingerprint: b.device.Fingerprint,
		Pending:           b.pendingCount(),
	}
	status.ReachedForges, status.UnhappyForges, status.LastError = b.forgeReport()
	if b.tick == 1 {
		// What the bridge carries is the first thing an operator or a script
		// adding a forge wants to know, so it is said out loud once, at
		// startup, rather than left to a guess about the log.
		b.log.Info("the forges the bridge serves", "reached", status.ReachedForges, "unreached", status.UnhappyForges)
	}
	return b.writeStatus(status)
}

// forgeReport is the forges the tick served and the forges it could not, in
// the order the configuration names them, and the last of the reasons why. A
// tick that serves a forge again is a tick that stops naming it unhappy.
func (b *Bridge) forgeReport() (reached, unhappy []string, last string) {
	for _, forge := range b.cfg.Forges {
		why, skipped := b.unhappy[forge.Root]
		if skipped {
			unhappy = append(unhappy, b.cfg.ForgeName(forge.Root))
			last = why
			continue
		}
		reached = append(reached, b.cfg.ForgeName(forge.Root))
	}
	return reached, unhappy, last
}

// pendingFor is the work one forge still owes the rooms.
func (b *Bridge) pendingFor(root string) *pendingChat {
	return pendingAt(b.pending, root, newPendingChat)
}

// pendingApprovalsFor is the approvals work one forge still owes the rooms.
func (b *Bridge) pendingApprovalsFor(key string) *pendingApprovals {
	return pendingAt(b.pendingApprovals, key, newPendingApprovals)
}

// pendingClarificationsFor is the clarifications work one forge still owes the
// room.
func (b *Bridge) pendingClarificationsFor(key string) *pendingClarifications {
	return pendingAt(b.pendingClarifications, key, newPendingClarifications)
}

// scopedToRoom is the share of the bridge's bookkeeping one room reports on:
// the items whose message that room carries. Every forge keeps its own rooms,
// so one forge's room never reports on another forge's work.
func scopedToRoom[T any](all map[string]T, roomOf func(T) string, roomID string) map[string]T {
	scoped := map[string]T{}
	for key, item := range all {
		if roomOf(item) == roomID {
			scoped[key] = item
		}
	}
	return scoped
}

// refuse records a forge the bridge could not serve, so the tick can carry on
// with the other forges, the forge is named in the status, and what it could
// not do is tried again.
func (b *Bridge) refuse(err error, root string) {
	b.unhappy[root] = err.Error()
	b.log.Error("the forge could not be served", "root", root, "error", err)
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

	carriedOut := b.carryOutChat(ctx, root, store, room, seen, requests)

	approvals, err := b.carryOutApprovals(ctx, root, room, seen.messages[room.ApprovalsRoomID], seen.reactions[room.ApprovalsRoomID])
	if err != nil {
		b.refuse(err, root)
	} else {
		carriedOut += approvals
	}

	activity, err := b.carryOutActivity(ctx, root, room)
	if err != nil {
		b.refuse(err, root)
	} else {
		carriedOut += activity
	}

	clarifications, err := b.carryOutClarifications(ctx, root, room, seen.messages[room.ClarificationsRoomID])
	if err != nil {
		b.refuse(err, root)
	} else {
		carriedOut += clarifications
	}
	return carriedOut, nil
}

// carryOutChat carries out the chat work one forge owes the room and reports
// how much of it was carried out. An action the forge refuses is kept and tried
// again, while the rest of the work still goes ahead.
func (b *Bridge) carryOutChat(ctx context.Context, root string, store ForgeStore, room Room, seen roomEvents, requests []relay.Request) int {
	pending := b.pendingFor(root)
	for _, action := range relay.Plan(b.cfg.Operator, b.state.Relay, requests, seen.messages[room.RoomID]) {
		pending.keep(action)
	}

	carriedOut := 0
	for _, action := range pending.list() {
		if err := b.apply(ctx, root, store, room, action); err != nil {
			// One action the forge refuses is reported and tried again; the
			// rest of the tick still goes ahead.
			b.refuse(err, root)
			continue
		}
		pending.done(action)
		carriedOut++
	}

	paired, err := b.pairPendingRequests(store)
	if err != nil {
		b.refuse(err, root)
		return carriedOut
	}
	return carriedOut + paired
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
// {"version":1,"tested_at":"2026-09-23T14:12:31+02:00","module_hash":"7614022afb088a021faaea22b01370cadb44b13f511dd9646295dde412ec3856","functions":[{"id":"func/New","name":"New","line":97,"end_line":123,"hash":"e8586e0ff92b2cfe628b8eb3d8720d1e2f911844113b32202bcea3594258a87a"},{"id":"func/Bridge.State","name":"Bridge.State","line":126,"end_line":128,"hash":"bf410b1bb8d53a98c0174a9807b8510a29bc02200bb47d241ad7e9f14ff2f7aa"},{"id":"func/Bridge.Run","name":"Bridge.Run","line":131,"end_line":152,"hash":"8f3f629a65f21167539ddf1f5f571a073f55caec778b2ce61c1d37026e0dc233"},{"id":"func/backOff","name":"backOff","line":157,"end_line":159,"hash":"313dab7f39c47410d8e18174342f6c613f4c66b3081679d7099f2bf342b7a010"},{"id":"func/Bridge.Tick","name":"Bridge.Tick","line":166,"end_line":196,"hash":"b2c082fd1734357325e44bd6c6c49dced78d1cf9cedaaf5de85c14191dbea687"},{"id":"func/Bridge.unhappyForges","name":"Bridge.unhappyForges","line":201,"end_line":213,"hash":"11fbc888ef8c67fa327d2e7ca2d500e4554cec4a99f7a1321fdf7d3eede04290"},{"id":"func/Bridge.pendingFor","name":"Bridge.pendingFor","line":216,"end_line":218,"hash":"92da8d17577eeedf6d82d5e3448f73896c18792e1a1d1c764c309f563e248b23"},{"id":"func/Bridge.pendingApprovalsFor","name":"Bridge.pendingApprovalsFor","line":221,"end_line":223,"hash":"2b132b7f8b2d48d3dcf9b00f10c4d7582f2b4ee4a27b84383d6d05729ee9cb49"},{"id":"func/Bridge.pendingClarificationsFor","name":"Bridge.pendingClarificationsFor","line":227,"end_line":229,"hash":"dc933f20516563b5e8e5df0e708ac8c1e65ed054aad25810e3a5a19876bac975"},{"id":"func/scopedToRoom","name":"scopedToRoom","line":234,"end_line":242,"hash":"445da93034eadfdfe3a704a206e2fe52d35da11888e1e72fd0b2b30b2e5363a5"},{"id":"func/Bridge.refuse","name":"Bridge.refuse","line":247,"end_line":250,"hash":"a0000892d59676cca2151216c141d9666955fe0db459f8f469d62ef1d96cffa2"},{"id":"func/Bridge.drainRooms","name":"Bridge.drainRooms","line":260,"end_line":278,"hash":"9bb5f3184873715385e70794087854cbdd08c8af11843ecd660f1b8902e100dd"},{"id":"func/Bridge.tickForge","name":"Bridge.tickForge","line":282,"end_line":319,"hash":"3d2858f7cb4695f83ef67806f676bbe8905e71717872786f6c546b838d43d8f2"},{"id":"func/Bridge.carryOutChat","name":"Bridge.carryOutChat","line":324,"end_line":348,"hash":"19f031febc9abce01cb10dfec83c320b71839f9d240a22d9e0ab45ef6e2ead65"},{"id":"func/Bridge.apply","name":"Bridge.apply","line":350,"end_line":355,"hash":"1be31d1a552df16d85d903a2d19730345bef233dc0c774af992cf9771631039e"},{"id":"func/Bridge.carryOut","name":"Bridge.carryOut","line":358,"end_line":368,"hash":"73839d70b2ed29dd317ee1a565750e233d3fd802787c531d6a98987f5a067a0c"}]}
// mutate4go-manifest-end
