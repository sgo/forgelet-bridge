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
