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
