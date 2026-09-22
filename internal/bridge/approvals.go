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
func (b *Bridge) carryOutApprovals(ctx context.Context, root string, room Room, replies []relay.RoomEvent, reactions []relay.Reaction) (int, error) {
	store, ok := b.approvals[root]
	if !ok {
		return 0, fmt.Errorf("no approvals configured for forge root %s", root)
	}
	pending, err := store.Pending(ctx)
	if err != nil {
		return 0, fmt.Errorf("read pending approvals for %s: %w", root, err)
	}

	waiting := b.pendingApprovalsFor(root)
	for _, action := range relay.PlanApprovals(b.cfg.Operator, b.state.Relay, pending, reactions, replies) {
		waiting.keep(action)
	}

	carriedOut := 0
	for _, action := range waiting.list() {
		if err := b.applyApproval(ctx, store, room, action); err != nil {
			b.reportApprovalFailure(ctx, room, waiting, action, err)
			continue
		}
		waiting.done(action)
		carriedOut++
	}
	return carriedOut, nil
}

func (b *Bridge) applyApproval(ctx context.Context, store ApprovalStore, room Room, action relay.ApprovalAction) error {
	if err := b.carryOutApproval(ctx, store, room, action); err != nil {
		return err
	}
	return b.state.Save(b.statePath)
}

// carryOutApproval is the one piece of approvals work an action asks for.
func (b *Bridge) carryOutApproval(ctx context.Context, store ApprovalStore, room Room, action relay.ApprovalAction) error {
	switch action.Kind {
	case relay.PostApproval:
		return b.postApproval(ctx, room, action)
	case relay.ResolveApproval:
		return b.resolveApproval(ctx, store, action)
	case relay.ReplyApproval:
		return b.reportResolution(ctx, room, action)
	case relay.AnswerGestures:
		return b.answerGestures(ctx, room)
	}
	return fmt.Errorf("unknown approval action %q", action.Kind)
}

// postApproval posts a pending approval into the room and remembers its message.
func (b *Bridge) postApproval(ctx context.Context, room Room, action relay.ApprovalAction) error {
	eventID, err := b.rooms.SendText(ctx, room.ApprovalsRoomID, approvalMessage(action.Approval), "")
	if err != nil {
		return fmt.Errorf("post approval %s: %w", action.Key, err)
	}
	b.recordApproval(action.Key, func(state relay.ApprovalState) relay.ApprovalState {
		state.MessageID = eventID
		return state
	})
	return nil
}

// reportResolution reports in the approval's thread how it was resolved.
func (b *Bridge) reportResolution(ctx context.Context, room Room, action relay.ApprovalAction) error {
	eventID, err := b.rooms.SendText(ctx, room.ApprovalsRoomID, action.Text, action.MessageID)
	if err != nil {
		return fmt.Errorf("report approval %s: %w", action.Key, err)
	}
	b.recordApproval(action.Key, func(state relay.ApprovalState) relay.ApprovalState {
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

func (b *Bridge) resolveApproval(ctx context.Context, store ApprovalStore, action relay.ApprovalAction) error {
	approval := action.Approval
	switch action.Resolution {
	case relay.ResolutionApproved:
		if err := store.Approve(ctx, approval.Project, approval.ID); err != nil {
			return fmt.Errorf("approve %s: %w", action.Key, err)
		}
	case relay.ResolutionSentBack:
		if err := store.SendBack(ctx, approval.Project, approval.ID, action.Feedback); err != nil {
			return fmt.Errorf("send back %s: %w", action.Key, err)
		}
	default:
		return fmt.Errorf("unknown resolution %q for %s", action.Resolution, action.Key)
	}
	b.recordApproval(action.Key, func(state relay.ApprovalState) relay.ApprovalState {
		state.Resolution = action.Resolution
		return state
	})
	return nil
}

func (b *Bridge) recordApproval(key string, update func(relay.ApprovalState) relay.ApprovalState) {
	b.state.Relay.EnsureMaps()
	b.state.Relay.Approvals[key] = update(b.state.Relay.Approvals[key])
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T16:11:53+02:00","module_hash":"bf6db4601782952b14f9306e56e0e9919ef90cd4b2408b72805c2f728cf7b752","functions":[{"id":"func/approvalMessage","name":"approvalMessage","line":17,"end_line":27,"hash":"c966f60938103c21d172fec1ff43c92a9528355965f869398c2d7e6b1ed880a1"},{"id":"func/Bridge.carryOutApprovals","name":"Bridge.carryOutApprovals","line":31,"end_line":48,"hash":"3ce7216007de4376acf2b0d99d615f6a46b17f26196ca029aada157078f4dce5"},{"id":"func/Bridge.applyApproval","name":"Bridge.applyApproval","line":50,"end_line":86,"hash":"2e08a8e3ce00d39f35215a9ca10beaeaee85f98ee35d2790d345bf9f5bb1b7d2"},{"id":"func/Bridge.resolveApproval","name":"Bridge.resolveApproval","line":88,"end_line":103,"hash":"cea2619e88d108f0867fe96e1cf177db9bff2aa8a01efc028687cb766ab3de26"},{"id":"func/Bridge.recordApproval","name":"Bridge.recordApproval","line":105,"end_line":108,"hash":"6d8353e7b3a9a49604cd3afca95c652a42d4380552b33d4fe34c21ce9ae3cb54"}]}
// mutate4go-manifest-end
