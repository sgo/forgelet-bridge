package bridge

import (
	"context"
	"fmt"
	"sort"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// pendingChat is the work one forge still owes the rooms: an action the forge
// refused is kept and tried again, so what a tick took from the rooms is not
// lost when the work it planned then fails, and one refusal does not hold up
// the rest of the tick.
type pendingChat struct {
	actions map[string]relay.Action
}

func newPendingChat() *pendingChat {
	return &pendingChat{actions: map[string]relay.Action{}}
}

func (p *pendingChat) keep(action relay.Action) {
	p.actions[chatActionKey(action)] = action
}

func (p *pendingChat) done(action relay.Action) {
	delete(p.actions, chatActionKey(action))
}

func (p *pendingChat) list() []relay.Action {
	keys := make([]string, 0, len(p.actions))
	for key := range p.actions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	kept := make([]relay.Action, 0, len(keys))
	for _, key := range keys {
		kept = append(kept, p.actions[key])
	}
	return kept
}

func chatActionKey(action relay.Action) string {
	return fmt.Sprintf("%s/%s/%s", action.Kind, action.RequestID, action.SourceEventID)
}

// pendingApprovals is the same for the approvals room.
type pendingApprovals struct {
	actions  map[string]relay.ApprovalAction
	reported map[string]bool
}

func newPendingApprovals() *pendingApprovals {
	return &pendingApprovals{actions: map[string]relay.ApprovalAction{}, reported: map[string]bool{}}
}

func (p *pendingApprovals) keep(action relay.ApprovalAction) {
	p.actions[approvalActionKey(action)] = action
}

func (p *pendingApprovals) done(action relay.ApprovalAction) {
	delete(p.actions, approvalActionKey(action))
	delete(p.reported, approvalActionKey(action))
}

func (p *pendingApprovals) list() []relay.ApprovalAction {
	keys := make([]string, 0, len(p.actions))
	for key := range p.actions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	kept := make([]relay.ApprovalAction, 0, len(keys))
	for _, key := range keys {
		kept = append(kept, p.actions[key])
	}
	return kept
}

func approvalActionKey(action relay.ApprovalAction) string {
	return fmt.Sprintf("%s/%s/%s", action.Kind, action.Key, action.Resolution)
}

// count is how much work is waiting for the forge.
func (b *Bridge) pendingCount() int {
	count := 0
	for _, pending := range b.pending {
		count += len(pending.actions)
	}
	for _, pending := range b.pendingApprovals {
		count += len(pending.actions)
	}
	return count
}

// reportApprovalFailure tells the operator, once, that the room could not carry
// out what they asked for.
func (b *Bridge) reportApprovalFailure(ctx context.Context, room Room, pending *pendingApprovals, action relay.ApprovalAction, cause error) {
	key := approvalActionKey(action)
	if pending == nil || pending.reported[key] {
		return
	}
	pending.reported[key] = true

	text := fmt.Sprintf("Could not %s", approvalFailureVerb(action))
	anchor := action.MessageID
	if anchor == "" {
		anchor = b.state.Relay.Approvals[action.Key].MessageID
	}
	if _, err := b.rooms.SendText(ctx, room.ApprovalsRoomID, text, anchor); err != nil {
		b.log.Error("could not report a refused approval", "key", action.Key, "error", err)
	}
	b.log.Error("the forge refused an approval action", "key", action.Key, "resolution", action.Resolution, "error", cause)
}

func approvalFailureVerb(action relay.ApprovalAction) string {
	switch action.Resolution {
	case relay.ResolutionSentBack:
		return "send it back"
	default:
		return "approve it"
	}
}
