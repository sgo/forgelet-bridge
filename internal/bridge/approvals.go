package bridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// approvalsRoomName is the room inside a forge's space that carries its
// approvals.
const approvalsRoomName = "Approvals"

// approvalMessage is what the operator reads on their phone: which project and
// card the approval is for, who is handing what to whom, and what changed.
func approvalMessage(approval relay.Approval) string {
	lines := []string{
		fmt.Sprintf("Approval for %s in %s", approval.Card, approval.Project),
		fmt.Sprintf("Gate: %s", approval.Gate),
	}
	if len(approval.Artifacts) > 0 {
		lines = append(lines, fmt.Sprintf("Changed files: %s", strings.Join(approval.Artifacts, ", ")))
	}
	lines = append(lines, fmt.Sprintf("React %s to approve, or reply in this thread to send it back.", relay.ApproveReaction))
	return strings.Join(lines, "\n")
}

// carryOutApprovals brings the approvals room and the forge's projects in step
// with each other, and reports how much work it did.
func (b *Bridge) carryOutApprovals(ctx context.Context, root string, room Room, replies []relay.RoomEvent, reactions []relay.Reaction, snapshot relay.DashboardSnapshot, fromSnapshot bool) (int, error) {
	store, ok := b.approvals[root]
	if !ok {
		return 0, fmt.Errorf("no approvals configured for forge root %s", root)
	}
	pending, desk, err := approvalInputs(ctx, root, store, snapshot, fromSnapshot)
	if err != nil {
		// A failed input read carries out no approval work in this tick.
		return 0, err
	}

	waiting := b.pendingApprovalsFor(root)
	for _, action := range relay.PlanApprovalsWithDesk(b.cfg.Operator, b.approvalState(room), pending, desk, reactions, replies) {
		waiting.keep(action)
	}

	carriedOut := b.carryOutPendingApprovals(ctx, root, store, room, waiting)
	written, err := b.carryOutFacts(ctx, room.ApprovalsRoomID, relay.ApprovalFactType, approvalFacts(pending),
		b.rememberApprovalStateEvents(root, pending))
	if err != nil {
		return carriedOut + written, err
	}
	carriedOut += written
	return carriedOut, nil
}

// carryOutPendingApprovals performs the approval actions this tick planned.
func (b *Bridge) carryOutPendingApprovals(ctx context.Context, root string, store ApprovalStore, room Room, waiting *pendingApprovals) int {
	carriedOut := 0
	for _, action := range waiting.list() {
		if err := b.applyApproval(ctx, root, store, room, action); err != nil {
			b.reportApprovalFailure(ctx, room, waiting, action, err)
			// The forge refused this action: it is named in the status, and
			// the rest of the tick - the other forges included - goes ahead.
			b.refuse(err, root)
			continue
		}
		waiting.done(action)
		carriedOut++
	}
	return carriedOut
}

// approvalInputs returns the pending and desk-resolved approvals from the
// tick's snapshot, or from the independent fallback stores.
func approvalInputs(ctx context.Context, root string, store ApprovalStore, snapshot relay.DashboardSnapshot, fromSnapshot bool) ([]relay.Approval, map[string]string, error) {
	if fromSnapshot {
		return snapshot.Approvals, snapshot.ResolvedApprovals, nil
	}
	pending, err := store.Pending(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("read pending approvals for %s: %w", root, err)
	}
	desk, err := store.Resolved(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("read the desk's resolved approvals for %s: %w", root, err)
	}
	return pending, desk, nil
}

// approvalFacts is the approvals the forge is still holding, each as the state
// its room carries: an approval that stops waiting is cleared by carrying out
// the facts without it.
func approvalFacts(pending []relay.Approval) []relay.Fact {
	facts := make([]relay.Fact, 0, len(pending))
	for _, approval := range pending {
		facts = append(facts, relay.ApprovalFact(approval))
	}
	return facts
}

// rememberApprovalStateEvents records the id of each state event a room writes
// for an approval, so a reaction or a reply that names that event decides the
// approval. The state keeps the approval under its project-qualified key while
// the room keys the event the way the relay's own fact does, so the two are
// mapped here, the room's key asked of the relay rather than assumed.
func (b *Bridge) rememberApprovalStateEvents(root string, pending []relay.Approval) func(stateKey, eventID string) {
	keys := make(map[string]string, len(pending))
	for _, approval := range pending {
		keys[relay.ApprovalFact(approval).Key] = approval.Key
	}
	return func(stateKey, eventID string) {
		key, known := keys[stateKey]
		if !known {
			return
		}
		b.recordApproval(root, key, func(state relay.ApprovalState) relay.ApprovalState {
			state.StateEventID = eventID
			return state
		})
	}
}

func (b *Bridge) applyApproval(ctx context.Context, root string, store ApprovalStore, room Room, action relay.ApprovalAction) error {
	if err := b.carryOutApproval(ctx, root, store, room, action); err != nil {
		return err
	}
	return b.state.Save(b.statePath)
}

// carryOutApproval is the one piece of approvals work an action asks for.
func (b *Bridge) carryOutApproval(ctx context.Context, root string, store ApprovalStore, room Room, action relay.ApprovalAction) error {
	actions := map[relay.ApprovalKind]func() error{
		relay.PostApproval:    func() error { return b.postApproval(ctx, root, room, action) },
		relay.ResolveApproval: func() error { return b.resolveApproval(ctx, root, store, action) },
		relay.ReplyApproval:   func() error { return b.reportResolution(ctx, root, room, action) },
		relay.ReactApproval:   func() error { return b.reactApproval(ctx, root, room, action) },
		relay.AnswerGestures:  func() error { return b.answerGestures(ctx, room) },
	}
	carryOut, ok := actions[action.Kind]
	if !ok {
		return fmt.Errorf("unknown approval action %q", action.Kind)
	}
	return carryOut()
}

// postApproval posts a pending approval into the room and remembers its message.
func (b *Bridge) postApproval(ctx context.Context, root string, room Room, action relay.ApprovalAction) error {
	eventID, err := b.rooms.SendText(ctx, room.ApprovalsRoomID, approvalMessage(action.Approval), "")
	if err != nil {
		return fmt.Errorf("post approval %s: %w", action.Key, err)
	}
	b.recordApproval(root, action.Key, func(state relay.ApprovalState) relay.ApprovalState {
		state.ForgeRoot = root
		state.RoomID = room.ApprovalsRoomID
		state.MessageID = eventID
		return state
	})
	return nil
}

// reactApproval marks the approval's own message with the bridge's reaction -
// the arrow that answers how the approval went - so the outcome sits on the
// line the operator acted on and the approval's thread stays quiet.
func (b *Bridge) reactApproval(ctx context.Context, root string, room Room, action relay.ApprovalAction) error {
	eventID, err := b.rooms.SendReaction(ctx, room.ApprovalsRoomID, action.MessageID, action.Reaction)
	if err != nil {
		return fmt.Errorf("mark approval %s: %w", action.Key, err)
	}
	b.recordApproval(root, action.Key, func(state relay.ApprovalState) relay.ApprovalState {
		state.Resolution = action.Resolution
		state.ReactionID = eventID
		return state
	})
	return nil
}

// approvalState is the share of the bridge's bookkeeping this forge's
// approvals room reports on: what it carries itself, never what another
// forge's room carries.
func (b *Bridge) approvalState(room Room) relay.State {
	scoped := b.state.Relay
	scoped.Approvals = scopedToRoom(b.state.Relay.Approvals, relay.ApprovalState.Room, room.ApprovalsRoomID)
	return scoped
}

// reportResolution reports in the approval's thread how it was resolved.
func (b *Bridge) reportResolution(ctx context.Context, root string, room Room, action relay.ApprovalAction) error {
	eventID, err := b.rooms.SendText(ctx, room.ApprovalsRoomID, action.Text, action.MessageID)
	if err != nil {
		return fmt.Errorf("report approval %s: %w", action.Key, err)
	}
	b.recordApproval(root, action.Key, func(state relay.ApprovalState) relay.ApprovalState {
		state.Resolution = action.Resolution
		state.ReplyID = eventID
		return state
	})
	return nil
}

// answerGestures tells the operator which gestures this room takes.
func (b *Bridge) answerGestures(ctx context.Context, room Room) error {
	if _, err := b.rooms.SendText(ctx, room.ApprovalsRoomID, gestureAnswer, ""); err != nil {
		return fmt.Errorf("answer the approval room: %w", err)
	}
	return nil
}

// gestureAnswer tells the operator, briefly, what this room can read.
const gestureAnswer = `Reply "approve" or react ✅ to approve; reply with anything else in the thread to send it back.`

func (b *Bridge) resolveApproval(ctx context.Context, root string, store ApprovalStore, action relay.ApprovalAction) error {
	if err := resolveInForge(ctx, store, action); err != nil {
		return err
	}
	b.recordApproval(root, action.Key, func(state relay.ApprovalState) relay.ApprovalState {
		state.Resolution = action.Resolution
		return state
	})
	return nil
}

// resolveInForge sends the one resolution the room decided to the dashboard.
func resolveInForge(ctx context.Context, store ApprovalStore, action relay.ApprovalAction) error {
	switch action.Resolution {
	case relay.ResolutionApproved:
		return approveInForge(ctx, store, action)
	case relay.ResolutionSentBack:
		return sendBackInForge(ctx, store, action)
	default:
		// Only the two resolutions the Matrix room can decide reach the forge.
		return fmt.Errorf("unknown resolution %q for %s", action.Resolution, action.Key)
	}
}

func approveInForge(ctx context.Context, store ApprovalStore, action relay.ApprovalAction) error {
	approval := action.Approval
	if err := store.Approve(ctx, approval.Project, approval.ID); err != nil {
		return fmt.Errorf("approve %s: %w", action.Key, err)
	}
	return nil
}

func sendBackInForge(ctx context.Context, store ApprovalStore, action relay.ApprovalAction) error {
	approval := action.Approval
	if err := store.SendBack(ctx, approval.Project, approval.ID, action.Feedback); err != nil {
		return fmt.Errorf("send back %s: %w", action.Key, err)
	}
	return nil
}

func (b *Bridge) recordApproval(root, key string, update func(relay.ApprovalState) relay.ApprovalState) {
	b.state.Relay.EnsureMaps()
	state := update(b.state.Relay.Approvals[key])
	state.ForgeRoot = root
	b.state.Relay.Approvals[key] = state
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-14T23:27:11+01:00","module_hash":"039e40b564f7f4fb863fbe2b68fdc1d6a57aa1a9ebab320e50f560e7e5f879fb","functions":[{"id":"func/approvalMessage","name":"approvalMessage","line":17,"end_line":27,"hash":"c966f60938103c21d172fec1ff43c92a9528355965f869398c2d7e6b1ed880a1"},{"id":"func/Bridge.carryOutApprovals","name":"Bridge.carryOutApprovals","line":31,"end_line":55,"hash":"148cb046c234c5b55ee27e952a487b02800d13c08b435a248451bd202432a2fb"},{"id":"func/Bridge.carryOutPendingApprovals","name":"Bridge.carryOutPendingApprovals","line":58,"end_line":72,"hash":"752ef3b21cc7950ec256ad0832db1fb0cf07391b42221776f73c0afcbd550b19"},{"id":"func/approvalInputs","name":"approvalInputs","line":76,"end_line":89,"hash":"6ff1524179765141b7f33bb64958f1e6467d34425445edeff903032ac7ace7c1"},{"id":"func/approvalFacts","name":"approvalFacts","line":94,"end_line":100,"hash":"bfab7e803860e26c7472ca3925ee6b0559b15d6113f4c01b54209fcd676435a3"},{"id":"func/Bridge.rememberApprovalStateEvents","name":"Bridge.rememberApprovalStateEvents","line":107,"end_line":122,"hash":"da5875ce221a9fe62fe73905672d0eb4f355cde28d046ab5592bbbbb45a88f76"},{"id":"func/Bridge.applyApproval","name":"Bridge.applyApproval","line":124,"end_line":129,"hash":"9eb8f4284cd0a737311629956fe635764febb60b2fcc2a611f7543f8a1e4b610"},{"id":"func/Bridge.carryOutApproval","name":"Bridge.carryOutApproval","line":132,"end_line":145,"hash":"eb29f1281f6dee300a720861858a99c288a25fbfdf1ce3fa7f24bc29adbf0767"},{"id":"func/Bridge.postApproval","name":"Bridge.postApproval","line":148,"end_line":160,"hash":"6d5c1bfd1a30f09d6a4d0b2f712702e48f062cff8e537e3f066cacf7d2c9628c"},{"id":"func/Bridge.reactApproval","name":"Bridge.reactApproval","line":165,"end_line":176,"hash":"a6d693a0a39e5c3c9fb9d140c3ccfd1aed9cdde9b6babe41240daaf5fa7f2a4d"},{"id":"func/Bridge.approvalState","name":"Bridge.approvalState","line":181,"end_line":185,"hash":"777b5a34f904eef029117545da2043c749d91f1667d44d461dbdd9ec5bd0e2a3"},{"id":"func/Bridge.reportResolution","name":"Bridge.reportResolution","line":188,"end_line":199,"hash":"a7faae6529e905f8c054b2aaabaac781c44c629f1c826cb8eb701a4bc4c038a2"},{"id":"func/Bridge.answerGestures","name":"Bridge.answerGestures","line":202,"end_line":207,"hash":"b3bf6c31bf3f45bc120e5e244e99da02854e89dce884b4d8df8e1c42512cb293"},{"id":"func/Bridge.resolveApproval","name":"Bridge.resolveApproval","line":212,"end_line":221,"hash":"3abafb58bc87ef4919bf4bb144d1c863755ff8561f0b9d7bec94d2c7d932c4a5"},{"id":"func/resolveInForge","name":"resolveInForge","line":224,"end_line":233,"hash":"db335e77a050f9338d605d03a777b4ce348a810a4bf7243961d4c792eca6e310"},{"id":"func/approveInForge","name":"approveInForge","line":235,"end_line":241,"hash":"a1270dcc83e543b982b2e13370241c8bacda128de19cd604a6c47530c73ac4dc"},{"id":"func/sendBackInForge","name":"sendBackInForge","line":243,"end_line":249,"hash":"c1981c3cc00f7138faedaaffa1481decce43c8443a043772aeb5a46264065d87"},{"id":"func/Bridge.recordApproval","name":"Bridge.recordApproval","line":251,"end_line":256,"hash":"9ac61e1e25c64c0a2e1891a2c35f9d191adb6340cfe2e7dd2657ff179a2589d9"}]}
// mutate4go-manifest-end
