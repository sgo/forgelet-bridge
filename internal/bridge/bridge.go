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

// Project is one single-agent project root served inside its forge's space.
// Its store is the project's own dashboard queue; its room is created in the
// forge's space rather than in a space of its own.
type Project struct {
	Name           string
	Root           string
	Store          ForgeStore
	Clarifications ClarificationStore
}

// ProjectSource discovers the single-agent projects a forge root currently has
// open. A project without the marker is a pack project and is not returned.
type ProjectSource interface {
	Projects(forgeRoot string) ([]Project, error)
}

// ApprovalStore is the forge side of one root's approvals: the handoffs its
// projects are waiting for.
type ApprovalStore interface {
	Pending(ctx context.Context) ([]relay.Approval, error)
	// Resolved reads how the desk resolved the approvals it made away from the
	// room, keyed by approval, so the room can report that ending rather than
	// only that the approval is gone.
	Resolved(ctx context.Context) (map[string]string, error)
	Approve(ctx context.Context, project, id string) error
	SendBack(ctx context.Context, project, id, feedback string) error
}

// ClarificationStore is the forge side of typed clarification answers. The
// questions themselves come only from the automation snapshot.
type ClarificationStore interface {
	AnswerTyped(ctx context.Context, project, id, answer string) error
}

// SnapshotStore reads all the waiting work one dashboard reports in one
// request. Identity names the dashboard so the same response can serve every
// consumer that addresses it in one tick.
type SnapshotStore interface {
	Identity() (string, error)
	Snapshot(ctx context.Context) (relay.DashboardSnapshot, error)
}

// BoardStore is the forge side of one root's boards: the cards its projects
// hold and the lanes they are in.
type BoardStore interface {
	// Boards lists every open project's board: the lanes it runs and the cards
	// each lane holds, so the room's state carries both.
	Boards() ([]relay.Board, error)
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
	// EnsureProjectRoom finds or creates one single-agent project's room inside
	// the forge's space.
	EnsureProjectRoom(ctx context.Context, spaceID, forgeName, projectName, operator string) (string, error)
	// RefreshProjectRoom applies the project's name and the operator's
	// membership to a room the bridge already has.
	RefreshProjectRoom(ctx context.Context, roomID, forgeName, projectName, operator string) error
	SendText(ctx context.Context, roomID, body, threadAnchor string) (string, error)
	// SendReaction marks an existing message with a reaction, the way the
	// operator's own check mark does.
	SendReaction(ctx context.Context, roomID, targetEventID, key string) (string, error)
	// SetState writes a state event into a room, or clears it with empty
	// content when there is nothing left to carry.
	SetState(ctx context.Context, roomID, eventType, stateKey string, content map[string]any) (string, error)
	// SendNotice posts a message clients do not notify on: something worth
	// seeing in the room and not worth waking anyone for.
	SendNotice(ctx context.Context, roomID, body string) (string, error)
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
	snapshots      map[string]SnapshotStore
	boards         map[string]BoardStore
	statePath      string
	statusDir      string
	state          *state.State
	log            *slog.Logger
	tick           uint64
	provisioned    map[string]Room
	device         Device
	projects       ProjectSource
	// Work the forge refused, kept so it is tried again rather than lost.
	pending               map[string]*pendingChat
	pendingApprovals      map[string]*pendingApprovals
	pendingClarifications map[string]*pendingClarifications
	// unhappy is why each forge could not be served in the tick now running,
	// so one sick forge is named while the rest keep their rooms.
	unhappy map[string]string
	// clarificationFailures names a forge whose typed clarification consumer
	// had no automation snapshot. Other consumers can still be served.
	clarificationFailures map[string]bool
}

// New builds a bridge around a Matrix client and one dashboard queue per forge
// root. The state file keeps restarts from repeating work.
func New(cfg config.Config, rooms Rooms, stores map[string]ForgeStore, approvals map[string]ApprovalStore, clarifications map[string]ClarificationStore, boards map[string]BoardStore, log *slog.Logger) (*Bridge, error) {
	if log == nil {
		log = slog.Default()
	}
	statePath := filepath.Join(cfg.StateDir, "bridge-state.json")
	loaded, err := loadBridgeState(statePath)
	if err != nil {
		return nil, err
	}
	return &Bridge{
		cfg:                   cfg,
		rooms:                 rooms,
		stores:                stores,
		approvals:             approvals,
		clarifications:        clarifications,
		snapshots:             snapshotStores(approvals),
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
		clarificationFailures: map[string]bool{},
	}, nil
}

// loadBridgeState loads the durable relay bookkeeping for one bridge.
func loadBridgeState(path string) (*state.State, error) {
	loaded, err := state.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load bridge state: %w", err)
	}
	return loaded, nil
}

// snapshotStores finds dashboard adapters that can serve one complete state
// response. The capability travels with the existing approvals adapter, so the
// process boundary does not need a second configuration surface.
func snapshotStores(approvals map[string]ApprovalStore) map[string]SnapshotStore {
	sources := make(map[string]SnapshotStore, len(approvals))
	for root, store := range approvals {
		if source, ok := store.(SnapshotStore); ok {
			sources[root] = source
		}
	}
	return sources
}

// SetProjectSource lets the bridge discover single-agent projects from the
// forge's own open-project list. The bridge tests and a forge with no
// single-agent projects leave it unset.
func (b *Bridge) SetProjectSource(source ProjectSource) {
	b.projects = source
}

// SetSnapshotSources lets the production adapters read each dashboard once
// per tick. A bridge without them keeps the independent stores as a fallback.
func (b *Bridge) SetSnapshotSources(sources map[string]SnapshotStore) {
	b.snapshots = sources
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
	b.runTick(ctx)
	wait := interval
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			if b.runTick(ctx) {
				wait = backOff(wait, interval)
				continue
			}
			wait = interval
		}
	}
}

// runTick performs one relay pass and reports whether it failed.
func (b *Bridge) runTick(ctx context.Context) bool {
	err := b.Tick(ctx)
	if err != nil {
		b.log.Error("tick failed", "error", err)
	}
	// The caller uses this result to choose whether to back off.
	return err != nil
}

// backOff spaces out retries after a failed tick, so a homeserver that is
// throttling or away is not hammered: the wait doubles, never drops below the
// interval the bridge was told to tick at, and never passes its ceiling.
func backOff(wait, interval time.Duration) time.Duration {
	return min(max(wait*2, interval), 30*time.Second)
}

// Tick carries out one round of work: catch the rooms up with the forges and
// the forges up with the rooms.
func (b *Bridge) Tick(ctx context.Context) error {
	seen, err := b.drainRooms(ctx)
	if err != nil {
		return err
	}
	snapshots := snapshotCache{}

	b.unhappy = map[string]string{}
	b.clarificationFailures = map[string]bool{}
	carriedOut := b.tickConfiguredForges(ctx, seen, snapshots)
	b.tick++
	return b.writeTickStatus(carriedOut)
}

// tickConfiguredForges serves every configured forge and the projects that
// forge has open, keeping one snapshot cache for the whole tick.
func (b *Bridge) tickConfiguredForges(ctx context.Context, seen roomEvents, snapshots snapshotCache) int {
	carriedOut := 0
	for _, forge := range b.cfg.Forges {
		carriedOut += b.tickForgeAndProjects(ctx, forge.Root, seen, snapshots)
	}
	return carriedOut
}

// tickForgeAndProjects keeps a forge failure local to that forge. Its projects
// are not attempted when the forge room itself cannot be served.
func (b *Bridge) tickForgeAndProjects(ctx context.Context, root string, seen roomEvents, snapshots snapshotCache) int {
	done, err := b.tickForge(ctx, root, seen, snapshots)
	if err != nil {
		b.refuse(err, root)
		// Projects are not attempted when their forge room cannot be served.
		return 0
	}
	// The tick's work is the forge room plus every open project it served.
	return done + b.tickProjects(ctx, root, seen, snapshots)
}

// tickProjects serves the open projects inside one forge.
func (b *Bridge) tickProjects(ctx context.Context, root string, seen roomEvents, snapshots snapshotCache) (carriedOut int) {
	if b.projects == nil {
		return
	}
	projects, err := b.projects.Projects(root)
	if err != nil {
		b.refuse(err, root)
		// A failed project discovery carries out no project work.
		return
	}
	for _, project := range projects {
		carriedOut += b.tickProjectIfOpen(ctx, root, project, seen, snapshots)
	}
	// Counts start empty and grow by the work each project carried out.
	return
}

// tickProjectIfOpen skips a project whose removal is already in progress and
// keeps any other project failure local to that project.
func (b *Bridge) tickProjectIfOpen(ctx context.Context, root string, project Project, seen roomEvents, snapshots snapshotCache) (carriedOut int) {
	if b.state.ProjectRemovalPending(root, project.Name) {
		// Removal progress suppresses this project until removal completes.
		return
	}
	done, err := b.tickProject(ctx, root, project, seen, snapshots)
	if err != nil {
		b.refuse(err, project.Root)
		return
	}
	carriedOut = done
	return
}

// writeTickStatus records the result of the relay pass that just finished.
func (b *Bridge) writeTickStatus(carriedOut int) error {
	status := Status{
		Tick:              b.tick,
		Idle:              carriedOut == 0 && b.pendingCount() == 0,
		DeviceID:          b.device.ID,
		DeviceFingerprint: b.device.Fingerprint,
		Pending:           b.pendingCount(),
	}
	status.ReachedForges, status.UnhappyForges, status.LastError = b.forgeReport()
	status.Owed = b.owedReport()
	status.ClarificationFailures = b.clarificationFailureReport()
	if b.tick == 1 {
		// What the bridge carries is the first thing an operator or a script
		// adding a forge wants to know, so it is said out loud once, at
		// startup, rather than left to a guess about the log.
		b.log.Info("the forges the bridge serves", "reached", status.ReachedForges, "unreached", status.UnhappyForges)
	}
	return b.writeStatus(status)
}

// clarificationFailureReport names configured Families whose clarification
// consumer lacked its required automation snapshot this tick.
func (b *Bridge) clarificationFailureReport() []string {
	var failed []string
	for _, forge := range b.cfg.Forges {
		if b.clarificationFailures[forge.Root] {
			failed = append(failed, b.cfg.ForgeName(forge.Root))
		}
	}
	return failed
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

// owedReport is the work the bridge still has to carry out for every configured
// forge, in the order the configuration names them.
func (b *Bridge) owedReport() []ForgeOwed {
	owed := make([]ForgeOwed, 0, len(b.cfg.Forges))
	for _, forge := range b.cfg.Forges {
		owed = append(owed, ForgeOwed{Name: b.cfg.ForgeName(forge.Root), Items: b.owedFor(forge.Root)})
	}
	return owed
}

// owedFor is the work one forge still owes the rooms: what it refused and what
// the tick could not carry out yet.
func (b *Bridge) owedFor(root string) int {
	return b.pendingFor(root).count() +
		b.pendingApprovalsFor(root).count() +
		b.pendingClarificationsFor(root).count()
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
func (b *Bridge) tickForge(ctx context.Context, root string, seen roomEvents, snapshots snapshotCache) (int, error) {
	room, store, requests, err := b.forgeTickInputs(ctx, root)
	if err != nil {
		return 0, err
	}

	carriedOut := b.carryOutChat(ctx, root, "", store, room, seen, requests)
	snapshot, fromSnapshot, stateWork, err := b.carryOutStateConsumers(ctx, root, room, seen, snapshots)
	if err != nil {
		b.refuse(err, root)
	} else {
		carriedOut += stateWork
		activity, activityErr := b.activityFrom(ctx, root, room, snapshot, fromSnapshot)
		if activityErr != nil {
			b.refuse(activityErr, root)
		} else {
			carriedOut += activity
		}
	}

	return carriedOut, nil
}

// forgeTickInputs reads the room and dashboard queue one forge needs before
// its consumers can run.
func (b *Bridge) forgeTickInputs(ctx context.Context, root string) (Room, ForgeStore, []relay.Request, error) {
	room, err := b.roomFor(ctx, root)
	if err != nil {
		return Room{}, nil, nil, err
	}
	store, ok := b.stores[root]
	if !ok {
		return Room{}, nil, nil, fmt.Errorf("no dashboard queue configured for forge root %s", root)
	}
	requests, err := store.Requests()
	if err != nil {
		return Room{}, nil, nil, fmt.Errorf("read dashboard requests for %s: %w", root, err)
	}
	return room, store, requests, nil
}

// carryOutStateConsumers reads one dashboard snapshot when available and
// carries its approval and clarification work. Approval or clarification
// action failures remain local to their consumer.
func (b *Bridge) carryOutStateConsumers(ctx context.Context, root string, room Room, seen roomEvents, snapshots snapshotCache) (relay.DashboardSnapshot, bool, int, error) {
	source := b.snapshots[root]
	snapshot := relay.DashboardSnapshot{}
	var err error
	if source != nil {
		snapshot, err = b.snapshotFrom(ctx, snapshots, source)
		if err != nil {
			// Both dependent consumers fail without any work for this tick.
			return snapshot, true, 0, err
		}
	}
	carriedOut := 0
	approvals, err := b.carryOutApprovals(
		ctx, root, room, seen.messages[room.ApprovalsRoomID], seen.reactions[room.ApprovalsRoomID],
		snapshot, source != nil)
	if err != nil {
		b.refuse(err, root)
	} else {
		carriedOut += approvals
	}
	if source == nil {
		b.clarificationFailures[root] = true
		b.log.Error("typed clarifications require an automation snapshot", "root", root)
	} else {
		clarifications, err := b.carryOutClarifications(
			ctx, root, b.clarifications[root], room, seen.messages[room.ClarificationsRoomID], snapshot)
		if err != nil {
			b.refuse(err, root)
		} else {
			carriedOut += clarifications
		}
	}
	return snapshot, source != nil, carriedOut, nil
}

// carryOutChat carries out the chat work one forge owes the room and reports
// how much of it was carried out. An action the forge refuses is kept and tried
// again, while the rest of the work still goes ahead.
func (b *Bridge) carryOutChat(ctx context.Context, root, project string, store ForgeStore, room Room, seen roomEvents, requests []relay.Request) int {
	pending := b.pendingFor(root)
	for _, action := range relay.Plan(b.cfg.Operator, b.state.Relay, requests, seen.messages[room.RoomID]) {
		pending.keep(action)
	}

	carriedOut := 0
	for _, action := range pending.list() {
		if err := b.apply(ctx, root, project, store, room, action); err != nil {
			// One action the forge refuses is reported and tried again; the
			// rest of the tick still goes ahead.
			b.refuse(err, root)
			continue
		}
		pending.done(action)
		carriedOut++
	}

	paired, err := b.pairPendingRequests(root, project, store)
	if err != nil {
		b.refuse(err, root)
		return carriedOut
	}
	return carriedOut + paired
}

func (b *Bridge) apply(ctx context.Context, root, project string, store ForgeStore, room Room, action relay.Action) error {
	if err := b.carryOut(ctx, root, project, store, room, action); err != nil {
		return err
	}
	return b.state.Save(b.statePath)
}

// carryOut is the one piece of work an action asks for.
func (b *Bridge) carryOut(ctx context.Context, root, project string, store ForgeStore, room Room, action relay.Action) error {
	switch action.Kind {
	case relay.PostRequestMessage:
		return b.postRequestMessage(ctx, root, project, room, action)
	case relay.PostRequestReply:
		return b.postRequestReply(ctx, root, project, room, action)
	case relay.CreateForgeRequest:
		return b.createForgeRequest(ctx, store, root, project, action)
	}
	return fmt.Errorf("unknown relay action %q", action.Kind)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-17T15:17:52+01:00","module_hash":"eb37771d6a5d5c28a49492b4161cfd19e30f44b453ba05ad0626e90cf9665aa3","functions":[{"id":"func/New","name":"New","line":146,"end_line":174,"hash":"4e2d5d9c568eab7db474158c40b322f49f8c248949d5580684d85fe366692181"},{"id":"func/loadBridgeState","name":"loadBridgeState","line":177,"end_line":183,"hash":"3af36a0c01e53921f684edbc2cc0e7f6549f682fc7d208ac7a60ec80b8d7c9d2"},{"id":"func/snapshotStores","name":"snapshotStores","line":188,"end_line":196,"hash":"d58459d5645da3fdf440e91b006e008095fe054e16f45d35f84c29eee994ad2c"},{"id":"func/Bridge.SetProjectSource","name":"Bridge.SetProjectSource","line":201,"end_line":203,"hash":"81cceca900d1598467e463637cdaa435545ec6b05d6b3d2cc5e04ceb31f71d06"},{"id":"func/Bridge.SetSnapshotSources","name":"Bridge.SetSnapshotSources","line":207,"end_line":209,"hash":"4dac8d7a533f0beb0ae0c129b8e3fef340ad9842b73a8206bb31ae942214d694"},{"id":"func/Bridge.State","name":"Bridge.State","line":212,"end_line":214,"hash":"bf410b1bb8d53a98c0174a9807b8510a29bc02200bb47d241ad7e9f14ff2f7aa"},{"id":"func/Bridge.Run","name":"Bridge.Run","line":217,"end_line":235,"hash":"4c794a9bf882c89a9285371a77b07a4ebdb83e767a88bce6c694c76c363c494a"},{"id":"func/Bridge.runTick","name":"Bridge.runTick","line":238,"end_line":245,"hash":"a9bcaa6d9f1f172c20ac37dcf91b794f30cd0030a6ac6e67b8c4335ec9645ab2"},{"id":"func/backOff","name":"backOff","line":250,"end_line":252,"hash":"f79b36ff71b470c2d08648b1816e8ead1da7c3aca5887a2cb0cc7424bf6a7090"},{"id":"func/Bridge.Tick","name":"Bridge.Tick","line":256,"end_line":268,"hash":"cf3f187cbb44d219ea2c4c022039915a88bf6edb1cf936ac294b817848af30a8"},{"id":"func/Bridge.tickConfiguredForges","name":"Bridge.tickConfiguredForges","line":272,"end_line":278,"hash":"5b427af14ba4f9c4ef4e6da52aba02dff492f507a7e51eed2e92cc300690f513"},{"id":"func/Bridge.tickForgeAndProjects","name":"Bridge.tickForgeAndProjects","line":282,"end_line":291,"hash":"6b5571a0eda85e6c6446b1847559874226323d7896c433496bbd5e00063f7f53"},{"id":"func/Bridge.tickProjects","name":"Bridge.tickProjects","line":294,"end_line":309,"hash":"a5a27217d534cd6d8c4d99540f1c079d7a06775a389bc71e03fa29c75fe8d6ed"},{"id":"func/Bridge.tickProjectIfOpen","name":"Bridge.tickProjectIfOpen","line":313,"end_line":325,"hash":"c1df7287938b576ac0ad17ece9420e773e201c42ad26607149e73a215247066f"},{"id":"func/Bridge.writeTickStatus","name":"Bridge.writeTickStatus","line":328,"end_line":346,"hash":"b8f66684e45837892fd5de14cc04312dd625839407ad32b81f8ff66ac3a7428b"},{"id":"func/Bridge.clarificationFailureReport","name":"Bridge.clarificationFailureReport","line":350,"end_line":358,"hash":"44ce9a87748c12c0856db2bfda95592fa8b39f0c6ef573a80f2614386dc08ab6"},{"id":"func/Bridge.forgeReport","name":"Bridge.forgeReport","line":363,"end_line":374,"hash":"ae1f631af0f579c29587141c1cc92631d2f0a11dce543cf99bb4a3464505a046"},{"id":"func/Bridge.pendingFor","name":"Bridge.pendingFor","line":377,"end_line":379,"hash":"92da8d17577eeedf6d82d5e3448f73896c18792e1a1d1c764c309f563e248b23"},{"id":"func/Bridge.owedReport","name":"Bridge.owedReport","line":383,"end_line":389,"hash":"9fba61b71ca397e843f6642fb1f4102a576086811147030152f04749cd2c2a9a"},{"id":"func/Bridge.owedFor","name":"Bridge.owedFor","line":393,"end_line":397,"hash":"0fc7db7e7230a2e5b8b8db380f8ed44932819fe4cedb25d27be537d6d388d104"},{"id":"func/Bridge.pendingApprovalsFor","name":"Bridge.pendingApprovalsFor","line":400,"end_line":402,"hash":"2b132b7f8b2d48d3dcf9b00f10c4d7582f2b4ee4a27b84383d6d05729ee9cb49"},{"id":"func/Bridge.pendingClarificationsFor","name":"Bridge.pendingClarificationsFor","line":406,"end_line":408,"hash":"dc933f20516563b5e8e5df0e708ac8c1e65ed054aad25810e3a5a19876bac975"},{"id":"func/scopedToRoom","name":"scopedToRoom","line":413,"end_line":421,"hash":"445da93034eadfdfe3a704a206e2fe52d35da11888e1e72fd0b2b30b2e5363a5"},{"id":"func/Bridge.refuse","name":"Bridge.refuse","line":426,"end_line":429,"hash":"a0000892d59676cca2151216c141d9666955fe0db459f8f469d62ef1d96cffa2"},{"id":"func/Bridge.drainRooms","name":"Bridge.drainRooms","line":439,"end_line":457,"hash":"9bb5f3184873715385e70794087854cbdd08c8af11843ecd660f1b8902e100dd"},{"id":"func/Bridge.tickForge","name":"Bridge.tickForge","line":461,"end_line":482,"hash":"6ee01653ee980ac5e1316c801757a94bc5d3dda95ce4e1d39a03bcdf835d9ea2"},{"id":"func/Bridge.forgeTickInputs","name":"Bridge.forgeTickInputs","line":486,"end_line":500,"hash":"4787f4356a9734b593311f0f6ecfaf900efb30fdbc5b307551c27faf97d199b8"},{"id":"func/Bridge.carryOutStateConsumers","name":"Bridge.carryOutStateConsumers","line":505,"end_line":538,"hash":"7103cd20208acec4a8cb96272c935fab8b3cf2941d6556b6436f7a0ecc9c03a6"},{"id":"func/Bridge.carryOutChat","name":"Bridge.carryOutChat","line":543,"end_line":567,"hash":"248d1f49bef5a99de88b154b21af4f14835e0fcbdee451364ad5a59ff002e68b"},{"id":"func/Bridge.apply","name":"Bridge.apply","line":569,"end_line":574,"hash":"eae760be9fe3e2d404da7f84cb87961a48db887e2a0149e422bc625ec61b3700"},{"id":"func/Bridge.carryOut","name":"Bridge.carryOut","line":577,"end_line":587,"hash":"aecb68109dc127d8cb20847827edd650d29c8c215e1fbddda6ba39560649b0a8"}]}
// mutate4go-manifest-end
