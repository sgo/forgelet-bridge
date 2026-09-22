package bridge

import (
	"context"
	"fmt"
	"sort"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// pendingWork is the work one forge still owes a room: an action the forge
// refused is kept and tried again, so what a tick took from the rooms is not
// lost when the work it planned then fails, and one refusal does not hold up
// the rest of the tick. Actions are keyed by what makes them the same piece of
// work, so a tick cannot ask the forge for the same thing twice.
type pendingWork[T any] struct {
	keyOf func(T) string
	items map[string]T
}

func newPendingWork[T any](keyOf func(T) string) *pendingWork[T] {
	return &pendingWork[T]{keyOf: keyOf, items: map[string]T{}}
}

func (p *pendingWork[T]) keep(action T) { p.items[p.keyOf(action)] = action }

func (p *pendingWork[T]) done(action T) { delete(p.items, p.keyOf(action)) }

func (p *pendingWork[T]) count() int { return len(p.items) }

// list is the work waiting, in a stable order so the forge is asked for it the
// same way on every tick.
func (p *pendingWork[T]) list() []T {
	keys := make([]string, 0, len(p.items))
	for key := range p.items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	kept := make([]T, 0, len(keys))
	for _, key := range keys {
		kept = append(kept, p.items[key])
	}
	return kept
}

// pendingChat is the chat work one forge owes the room.
type pendingChat = pendingWork[relay.Action]

func newPendingChat() *pendingChat {
	return newPendingWork[relay.Action](chatActionKey)
}

func chatActionKey(action relay.Action) string {
	return fmt.Sprintf("%s/%s/%s", action.Kind, action.RequestID, action.SourceEventID)
}

// pendingApprovals is the same for the approvals room, and remembers which
// refusals it has already reported.
type pendingApprovals struct {
	work     *pendingWork[relay.ApprovalAction]
	reported map[string]bool
}

func newPendingApprovals() *pendingApprovals {
	return &pendingApprovals{work: newPendingWork[relay.ApprovalAction](approvalActionKey), reported: map[string]bool{}}
}

func (p *pendingApprovals) keep(action relay.ApprovalAction) { p.work.keep(action) }

func (p *pendingApprovals) count() int { return p.work.count() }

func (p *pendingApprovals) done(action relay.ApprovalAction) {
	p.work.done(action)
	delete(p.reported, approvalActionKey(action))
}

func (p *pendingApprovals) list() []relay.ApprovalAction { return p.work.list() }

func approvalActionKey(action relay.ApprovalAction) string {
	return fmt.Sprintf("%s/%s/%s", action.Kind, action.Key, action.Resolution)
}

// count is how much work is waiting for the forge.
func (b *Bridge) pendingCount() int {
	count := 0
	for _, pending := range b.pending {
		count += pending.count()
	}
	for _, pending := range b.pendingApprovals {
		count += pending.count()
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
