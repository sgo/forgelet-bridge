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

	actions := relay.PlanApprovals(b.cfg.Operator, b.state.Relay, pending, reactions, replies)
	for _, action := range actions {
		if err := b.applyApproval(ctx, store, room, action); err != nil {
			return 0, err
		}
	}
	return len(actions), nil
}

func (b *Bridge) applyApproval(ctx context.Context, store ApprovalStore, room Room, action relay.ApprovalAction) error {
	switch action.Kind {
	case relay.PostApproval:
		eventID, err := b.rooms.SendText(ctx, room.ApprovalsRoomID, approvalMessage(action.Approval), "")
		if err != nil {
			return fmt.Errorf("post approval %s: %w", action.Key, err)
		}
		b.recordApproval(action.Key, func(state relay.ApprovalState) relay.ApprovalState {
			state.MessageID = eventID
			return state
		})

	case relay.ResolveApproval:
		if err := b.resolveApproval(ctx, store, action); err != nil {
			return err
		}
		b.recordApproval(action.Key, func(state relay.ApprovalState) relay.ApprovalState {
			state.Resolution = action.Resolution
			return state
		})

	case relay.ReplyApproval:
		eventID, err := b.rooms.SendText(ctx, room.ApprovalsRoomID, action.Text, action.MessageID)
		if err != nil {
			return fmt.Errorf("report approval %s: %w", action.Key, err)
		}
		b.recordApproval(action.Key, func(state relay.ApprovalState) relay.ApprovalState {
			state.Resolution = action.Resolution
			state.ReplyID = eventID
			return state
		})

	default:
		return fmt.Errorf("unknown approval action %q", action.Kind)
	}
	return b.state.Save(b.statePath)
}

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
	return nil
}

func (b *Bridge) recordApproval(key string, update func(relay.ApprovalState) relay.ApprovalState) {
	b.state.Relay.EnsureMaps()
	b.state.Relay.Approvals[key] = update(b.state.Relay.Approvals[key])
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T15:51:29+02:00","module_hash":"e0ad4dc50d144bca3fde7768b51a030eb65f23c4b0d52a23ccc7664bf5c8667e","functions":[{"id":"func/approvalMessage","name":"approvalMessage","line":17,"end_line":27,"hash":"c966f60938103c21d172fec1ff43c92a9528355965f869398c2d7e6b1ed880a1"},{"id":"func/Bridge.carryOutApprovals","name":"Bridge.carryOutApprovals","line":31,"end_line":48,"hash":"d6f57c11dbed075dfd26d498815ea1786366b0b716271c549bab31b3a554408b"},{"id":"func/Bridge.applyApproval","name":"Bridge.applyApproval","line":50,"end_line":86,"hash":"8fbb23ce660e877c8fe6cd7ac1f4bfbbe6f6dc0a35f3e5fe34bc16dc38e9107e"},{"id":"func/Bridge.resolveApproval","name":"Bridge.resolveApproval","line":88,"end_line":103,"hash":"31a622927ad84e5cf3cad85cee2635ec97c9294c7abf0f2dcd1f6c7b0fef9cb0"},{"id":"func/Bridge.recordApproval","name":"Bridge.recordApproval","line":105,"end_line":108,"hash":"6d8353e7b3a9a49604cd3afca95c652a42d4380552b33d4fe34c21ce9ae3cb54"}]}
// mutate4go-manifest-end
