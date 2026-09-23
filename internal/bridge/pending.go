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

// pendingAt is the work a key already holds, made on first use.
func pendingAt[T any](work map[string]T, key string, make func() T) T {
	item, ok := work[key]
	if !ok {
		item = make()
		work[key] = item
	}
	return item
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

// pendingClarifications is the clarifications room's share of that work.
type pendingClarifications = pendingWork[relay.ClarificationAction]

func newPendingClarifications() *pendingClarifications {
	return newPendingWork[relay.ClarificationAction](clarificationActionKey)
}

func clarificationActionKey(action relay.ClarificationAction) string {
	return fmt.Sprintf("%s/%s/%s", action.Kind, action.Key, action.Answer)
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
	for _, pending := range b.pendingClarifications {
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T22:58:51+02:00","module_hash":"91f5ad2ab4e91eaa2f83cd76e7b4932e5b2d84511d246d68ac8357e3d553dc18","functions":[{"id":"func/newPendingWork","name":"newPendingWork","line":21,"end_line":23,"hash":"993ed0c1d553f17d08a7a6d0ab6d1336281c179b5618c1dd66bf5cd798d58cfc"},{"id":"func/pendingWork.keep","name":"pendingWork.keep","line":25,"end_line":25,"hash":"e5b1359da5680095132730eb4af61965fbacf484e87b8c432ab12280b7842acd"},{"id":"func/pendingWork.done","name":"pendingWork.done","line":27,"end_line":27,"hash":"d9559e93d28f1b42b86b342e790e3a1a4d8dd638829b33f3e31ff4756545c94f"},{"id":"func/pendingWork.count","name":"pendingWork.count","line":29,"end_line":29,"hash":"6a6436ee5977eb656905d6d5b004e2e0acdfaaf9aba68716227764e837c7af81"},{"id":"func/pendingWork.list","name":"pendingWork.list","line":33,"end_line":44,"hash":"8cabe7fc0afeb395434372b797590d9764a271cad4fba44db2e279253eb4c17c"},{"id":"func/newPendingChat","name":"newPendingChat","line":49,"end_line":51,"hash":"aa801fbffe2e10e1e050c9cd7d68f7760430df7c712990e492fb313e8b89f6c0"},{"id":"func/chatActionKey","name":"chatActionKey","line":53,"end_line":55,"hash":"14eb4a05db96be7325b7fe1a28cc1af721524202f6673d988b7029805ba55dc2"},{"id":"func/newPendingApprovals","name":"newPendingApprovals","line":64,"end_line":66,"hash":"b74b4f4f264893b0b42f369962363995b10fbdaf150c4b21bb9c0bb1acc25af8"},{"id":"func/pendingApprovals.keep","name":"pendingApprovals.keep","line":68,"end_line":68,"hash":"c777f0d5fb5733a676294c1d72e0e4f45fa55e41da9818eac3b4518ac258271e"},{"id":"func/pendingApprovals.count","name":"pendingApprovals.count","line":70,"end_line":70,"hash":"be8324cc70885657853e13feeabdc59d8b530e6f26c9e61579288fdd4fa0df90"},{"id":"func/pendingApprovals.done","name":"pendingApprovals.done","line":72,"end_line":75,"hash":"f21b41751f1779334a9772d010ad8a206995ba150025f1386bbae4e4dd7cb594"},{"id":"func/pendingApprovals.list","name":"pendingApprovals.list","line":77,"end_line":77,"hash":"58d81754397baf178c384c893ab96a42543e37541b33d73b95bcd8ef01e74e46"},{"id":"func/approvalActionKey","name":"approvalActionKey","line":79,"end_line":81,"hash":"b672a745e53515e387382e596c9e10fc83f9127fad3339ff4275f78100f27727"},{"id":"func/Bridge.pendingCount","name":"Bridge.pendingCount","line":84,"end_line":93,"hash":"1a5d3cfbaa5020a5e35e8f1aa1870c88ade4d4c5c6bb4f5fc487fc2a264c7f56"},{"id":"func/Bridge.reportApprovalFailure","name":"Bridge.reportApprovalFailure","line":97,"end_line":113,"hash":"61989b95a28c6e43274a259d499480f3bd40648b0a45d241130fc5f8341e386d"},{"id":"func/approvalFailureVerb","name":"approvalFailureVerb","line":115,"end_line":122,"hash":"24c96b2aadaaa899e8ab907eb2e70d2b05b2292129085ee53f30f635bb79b134"}]}
// mutate4go-manifest-end
