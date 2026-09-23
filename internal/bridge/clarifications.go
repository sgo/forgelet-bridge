package bridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// clarificationMessage is what the operator reads on their phone: which
// project and role the question comes from, the question itself, and what this
// room takes as the answer.
func clarificationMessage(clarification relay.Clarification) string {
	lines := []string{
		fmt.Sprintf("Clarification for %s from %s", clarification.Project, clarification.Role),
		fmt.Sprintf("Question: %s", clarification.Question),
		"Reply in this thread with the answer.",
	}
	return strings.Join(lines, "\n")
}

// carryOutClarifications brings the clarifications room and the forge's
// blocked roles in step with each other, and reports how much work it did.
func (b *Bridge) carryOutClarifications(ctx context.Context, root string, room Room, replies []relay.RoomEvent) (int, error) {
	store, ok := b.clarifications[root]
	if !ok {
		return 0, fmt.Errorf("no clarifications configured for forge root %s", root)
	}
	pending, err := store.Pending(ctx)
	if err != nil {
		return 0, fmt.Errorf("read pending clarifications for %s: %w", root, err)
	}

	waiting := b.pendingClarificationsFor(root)
	for _, action := range relay.PlanClarifications(b.cfg.Operator, b.clarificationState(room), pending, replies) {
		waiting.keep(action)
	}

	carriedOut := 0
	for _, action := range waiting.list() {
		if err := b.applyClarification(ctx, store, room, action); err != nil {
			b.refuse(err, root)
			continue
		}
		waiting.done(action)
		carriedOut++
	}
	return carriedOut, nil
}

// applyClarification is one piece of clarifications work, kept in the state as
// soon as it lands so a restart does not repeat it.
func (b *Bridge) applyClarification(ctx context.Context, store ClarificationStore, room Room, action relay.ClarificationAction) error {
	if err := b.carryOutClarification(ctx, store, room, action); err != nil {
		return err
	}
	return b.state.Save(b.statePath)
}

// carryOutClarification is the one piece of clarifications work an action asks
// for.
func (b *Bridge) carryOutClarification(ctx context.Context, store ClarificationStore, room Room, action relay.ClarificationAction) error {
	switch action.Kind {
	case relay.PostClarification:
		return b.postClarification(ctx, room, action)
	case relay.AnswerClarification:
		return b.answerClarification(ctx, store, action)
	case relay.ReplyClarification:
		return b.reportClarificationAnswer(ctx, room, action)
	}
	return fmt.Errorf("unknown clarification action %q", action.Kind)
}

// postClarification posts a pending clarification into the room and remembers
// its message.
func (b *Bridge) postClarification(ctx context.Context, room Room, action relay.ClarificationAction) error {
	eventID, err := b.rooms.SendText(ctx, room.ClarificationsRoomID, clarificationMessage(action.Clarification), "")
	if err != nil {
		return fmt.Errorf("post clarification %s: %w", action.Key, err)
	}
	b.recordClarification(action.Key, func(state relay.ClarificationState) relay.ClarificationState {
		state.RoomID = room.ClarificationsRoomID
		state.MessageID = eventID
		return state
	})
	return nil
}

// clarificationState is the share of the bridge's bookkeeping this forge's
// clarifications room reports on: what it carries itself, never what another
// forge's room carries.
func (b *Bridge) clarificationState(room Room) relay.State {
	scoped := b.state.Relay
	scoped.Clarifications = scopedToRoom(b.state.Relay.Clarifications, relay.ClarificationState.Room, room.ClarificationsRoomID)
	return scoped
}

// answerClarification carries the operator's answer back through the forge's
// dashboard, which is what wakes the blocked role with it.
func (b *Bridge) answerClarification(ctx context.Context, store ClarificationStore, action relay.ClarificationAction) error {
	if err := store.Answer(ctx, action.Clarification.Project, action.Clarification.ID, action.Answer); err != nil {
		return fmt.Errorf("answer %s: %w", action.Key, err)
	}
	b.recordClarification(action.Key, func(state relay.ClarificationState) relay.ClarificationState {
		state.Answer = action.Answer
		return state
	})
	return nil
}

// reportClarificationAnswer reports in the clarification's thread that it was
// answered, and how.
func (b *Bridge) reportClarificationAnswer(ctx context.Context, room Room, action relay.ClarificationAction) error {
	eventID, err := b.rooms.SendText(ctx, room.ClarificationsRoomID, action.Text, action.MessageID)
	if err != nil {
		return fmt.Errorf("report clarification %s: %w", action.Key, err)
	}
	b.recordClarification(action.Key, func(state relay.ClarificationState) relay.ClarificationState {
		state.ReplyID = eventID
		return state
	})
	return nil
}

func (b *Bridge) recordClarification(key string, update func(relay.ClarificationState) relay.ClarificationState) {
	b.state.Relay.EnsureMaps()
	b.state.Relay.Clarifications[key] = update(b.state.Relay.Clarifications[key])
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T14:18:04+02:00","module_hash":"f43a40677ddae48a2f926a82152daf71cf76c998e2253f4da937c7098a55198c","functions":[{"id":"func/clarificationMessage","name":"clarificationMessage","line":14,"end_line":21,"hash":"bf629b53647781870cc687f0e3b611af29eaa93ea8200c62e142c80ab282e220"},{"id":"func/Bridge.carryOutClarifications","name":"Bridge.carryOutClarifications","line":25,"end_line":50,"hash":"b12f8d6adf227e159c399aefe3180bad62c09c12f11153dd4983f0186f7c1446"},{"id":"func/Bridge.applyClarification","name":"Bridge.applyClarification","line":54,"end_line":59,"hash":"92f44e1017061c5e01053827c433bdf57ec0b25e89f60efd326aae0977c43992"},{"id":"func/Bridge.carryOutClarification","name":"Bridge.carryOutClarification","line":63,"end_line":73,"hash":"23d3356753d5cfd9f166c0a75aa8bc78c7b8c482df23c3c282e9c7c21653155d"},{"id":"func/Bridge.postClarification","name":"Bridge.postClarification","line":77,"end_line":88,"hash":"0e7d4abab7c8ffe9dd7cd98c9bf773647a0b68ed973800436a47f4c28becab62"},{"id":"func/Bridge.clarificationState","name":"Bridge.clarificationState","line":93,"end_line":97,"hash":"977f6e2d654d6eb9617e5d702196122d6941f84fef1c999310fa6e13fd0f162b"},{"id":"func/Bridge.answerClarification","name":"Bridge.answerClarification","line":101,"end_line":110,"hash":"655952e47bba67fbc012ce6b59791225df8b1dc09da8155f68e54c2f0206fda7"},{"id":"func/Bridge.reportClarificationAnswer","name":"Bridge.reportClarificationAnswer","line":114,"end_line":124,"hash":"1c1d4473cbe89c4be70a2025035d70a0aa9d6c0b5d2eeed99d21dddaeb60bf47"},{"id":"func/Bridge.recordClarification","name":"Bridge.recordClarification","line":126,"end_line":129,"hash":"e154e2528d82ec618ebe08700597ab36c8d776abb2087de7674421d1a31b6faf"}]}
// mutate4go-manifest-end
