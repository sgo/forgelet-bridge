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
func (b *Bridge) carryOutClarifications(ctx context.Context, root string, store ClarificationStore, room Room, replies []relay.RoomEvent, snapshot relay.DashboardSnapshot, fromSnapshot bool) (carriedOut int, err error) {
	if store == nil {
		return 0, fmt.Errorf("no clarifications configured for forge root %s", root)
	}
	pending, err := clarificationInputs(ctx, root, store, snapshot, fromSnapshot)
	if err != nil {
		return
	}
	if fromSnapshot {
		if err := b.reconcileTypedClarifications(root, room, snapshot.TypedClarifications); err != nil {
			return carriedOut, err
		}
	}

	waiting := b.pendingClarificationsFor(root)
	for _, action := range relay.PlanClarifications(b.cfg.Operator, b.clarificationState(room), pending, replies) {
		waiting.keep(action)
	}

	carriedOut = b.carryOutPendingClarifications(ctx, root, store, room, waiting)
	written, err := b.carryOutFacts(ctx, room.ClarificationsRoomID, relay.ClarificationFactType, clarificationFacts(pending),
		b.rememberClarificationStateEvents(root, pending))
	carriedOut += written
	if err != nil {
		return carriedOut, err
	}
	return carriedOut, nil
}

// carryOutPendingClarifications performs the clarification actions this tick
// planned.
func (b *Bridge) carryOutPendingClarifications(ctx context.Context, root string, store ClarificationStore, room Room, waiting *pendingClarifications) int {
	carriedOut := 0
	for _, action := range waiting.list() {
		if err := b.applyClarification(ctx, root, store, room, action); err != nil {
			b.refuse(err, root)
			continue
		}
		waiting.done(action)
		carriedOut++
	}
	return carriedOut
}

// clarificationInputs returns the pending questions from the tick's snapshot,
// or from the independent fallback store.
func clarificationInputs(ctx context.Context, root string, store ClarificationStore, snapshot relay.DashboardSnapshot, fromSnapshot bool) ([]relay.Clarification, error) {
	if fromSnapshot {
		pending := append([]relay.Clarification(nil), snapshot.Clarifications...)
		for _, clarification := range snapshot.TypedClarifications {
			if clarification.Lifecycle == relay.ClarificationOpen {
				pending = append(pending, clarification)
			}
		}
		return pending, nil
	}
	pending, err := store.Pending(ctx)
	if err != nil {
		return nil, fmt.Errorf("read pending clarifications for %s: %w", root, err)
	}
	return pending, nil
}

// reconcileTypedClarifications clears typed notification records the
// authoritative snapshot says are answered, withdrawn or no longer present.
// The snapshot is authoritative for the whole tick, so absence is stale rather
// than a reason to retain an old notice.
func (b *Bridge) reconcileTypedClarifications(root string, room Room, typed []relay.Clarification) error {
	open := make(map[string]bool, len(typed))
	for _, clarification := range typed {
		if clarification.Lifecycle == relay.ClarificationOpen {
			open[clarification.Key] = true
		}
	}
	changed := false
	for key, state := range b.state.Relay.Clarifications {
		if state.Source != relay.ClarificationSourceTyped ||
			state.ForgeRoot != root ||
			state.RoomID != room.ClarificationsRoomID ||
			open[key] {
			continue
		}
		delete(b.state.Relay.Clarifications, key)
		changed = true
	}
	if changed {
		return b.state.Save(b.statePath)
	}
	return nil
}

// clarificationFacts is the questions the forge is still waiting on, each as
// the state its room carries: a question that is answered is cleared by
// carrying out the facts without it.
func clarificationFacts(pending []relay.Clarification) []relay.Fact {
	facts := make([]relay.Fact, 0, len(pending))
	for _, clarification := range pending {
		facts = append(facts, relay.ClarificationFact(clarification))
	}
	return facts
}

// rememberClarificationStateEvents records the id of each state event a room
// writes for a clarification, so a reply that names that event answers the
// question. The state keeps the question under its project-qualified key while
// the room keys the event the way the relay's own fact does, so the two are
// mapped here, the room's key asked of the relay rather than assumed.
func (b *Bridge) rememberClarificationStateEvents(root string, pending []relay.Clarification) func(stateKey, eventID string) {
	keys := make(map[string]string, len(pending))
	for _, clarification := range pending {
		keys[relay.ClarificationFact(clarification).Key] = clarification.Key
	}
	return func(stateKey, eventID string) {
		key, known := keys[stateKey]
		if !known {
			return
		}
		b.recordClarification(root, key, func(state relay.ClarificationState) relay.ClarificationState {
			state.StateEventID = eventID
			return state
		})
	}
}

// applyClarification is one piece of clarifications work, kept in the state as
// soon as it lands so a restart does not repeat it.
func (b *Bridge) applyClarification(ctx context.Context, root string, store ClarificationStore, room Room, action relay.ClarificationAction) error {
	if err := b.carryOutClarification(ctx, root, store, room, action); err != nil {
		return err
	}
	return b.state.Save(b.statePath)
}

// carryOutClarification is the one piece of clarifications work an action asks
// for.
func (b *Bridge) carryOutClarification(ctx context.Context, root string, store ClarificationStore, room Room, action relay.ClarificationAction) error {
	switch action.Kind {
	case relay.PostClarification:
		return b.postClarification(ctx, root, room, action)
	case relay.AnswerClarification:
		return b.answerClarification(ctx, root, store, action)
	case relay.ReplyClarification:
		return b.reportClarificationAnswer(ctx, root, room, action)
	case relay.ReactClarification:
		return b.reactClarification(ctx, root, room, action)
	}
	return fmt.Errorf("unknown clarification action %q", action.Kind)
}

// postClarification posts a pending clarification into the room and remembers
// its message.
func (b *Bridge) postClarification(ctx context.Context, root string, room Room, action relay.ClarificationAction) error {
	eventID, err := b.rooms.SendText(ctx, room.ClarificationsRoomID, clarificationMessage(action.Clarification), "")
	if err != nil {
		return fmt.Errorf("post clarification %s: %w", action.Key, err)
	}
	b.recordClarification(root, action.Key, func(state relay.ClarificationState) relay.ClarificationState {
		state.Source = action.Clarification.Source
		state.ForgeRoot = root
		state.RoomID = room.ClarificationsRoomID
		state.MessageID = eventID
		return state
	})
	return nil
}

// reactClarification marks the clarification's own message with the bridge's
// reaction, so an answer the operator gave here leaves the thread quiet.
func (b *Bridge) reactClarification(ctx context.Context, root string, room Room, action relay.ClarificationAction) error {
	eventID, err := b.rooms.SendReaction(ctx, room.ClarificationsRoomID, action.MessageID, action.Reaction)
	if err != nil {
		return fmt.Errorf("mark clarification %s: %w", action.Key, err)
	}
	b.recordClarification(root, action.Key, func(state relay.ClarificationState) relay.ClarificationState {
		state.ReactionID = eventID
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
func (b *Bridge) answerClarification(ctx context.Context, root string, store ClarificationStore, action relay.ClarificationAction) error {
	if action.Clarification.Source == relay.ClarificationSourceTyped {
		answerer, ok := store.(TypedClarificationAnswerer)
		if !ok {
			return fmt.Errorf("answer typed clarification %s: project chat is unavailable", action.Key)
		}
		if err := answerer.AnswerTyped(
			ctx, action.Clarification.Project, action.Clarification.ID, action.Answer); err != nil {
			return fmt.Errorf("answer %s through project chat: %w", action.Key, err)
		}
	} else if err := store.Answer(ctx, action.Clarification.Project, action.Clarification.ID, action.Answer); err != nil {
		return fmt.Errorf("answer %s: %w", action.Key, err)
	}
	b.recordClarification(root, action.Key, func(state relay.ClarificationState) relay.ClarificationState {
		state.Answer = action.Answer
		return state
	})
	return nil
}

// reportClarificationAnswer reports in the clarification's thread that it was
// answered, and how.
func (b *Bridge) reportClarificationAnswer(ctx context.Context, root string, room Room, action relay.ClarificationAction) error {
	eventID, err := b.rooms.SendText(ctx, room.ClarificationsRoomID, action.Text, action.MessageID)
	if err != nil {
		return fmt.Errorf("report clarification %s: %w", action.Key, err)
	}
	b.recordClarification(root, action.Key, func(state relay.ClarificationState) relay.ClarificationState {
		state.ReplyID = eventID
		return state
	})
	return nil
}

func (b *Bridge) recordClarification(root, key string, update func(relay.ClarificationState) relay.ClarificationState) {
	b.state.Relay.EnsureMaps()
	state := update(b.state.Relay.Clarifications[key])
	state.ForgeRoot = root
	b.state.Relay.Clarifications[key] = state
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-14T23:37:03+01:00","module_hash":"341ba2fc049405f9e821c065902151b765665c1b3c171746293d3c9eba306dc4","functions":[{"id":"func/clarificationMessage","name":"clarificationMessage","line":14,"end_line":21,"hash":"bf629b53647781870cc687f0e3b611af29eaa93ea8200c62e142c80ab282e220"},{"id":"func/Bridge.carryOutClarifications","name":"Bridge.carryOutClarifications","line":25,"end_line":47,"hash":"b09a172d31524ee30aa093f8836c8ac7ffd2f12df712da33a8c7d64a3a53fe0d"},{"id":"func/Bridge.carryOutPendingClarifications","name":"Bridge.carryOutPendingClarifications","line":51,"end_line":62,"hash":"9788b2a086a8b264c33d3740eaae223372070696fbcb43c80302c7f1027a8b65"},{"id":"func/clarificationInputs","name":"clarificationInputs","line":66,"end_line":75,"hash":"e5bd986d820696271f1d5f43b74d87fcfa98e397e1b9148a7771073de6673711"},{"id":"func/clarificationFacts","name":"clarificationFacts","line":80,"end_line":86,"hash":"8f7bee29d22539209d8d5582da3a08cfdc5f50b0eb11695284b63a6153d000d6"},{"id":"func/Bridge.rememberClarificationStateEvents","name":"Bridge.rememberClarificationStateEvents","line":93,"end_line":108,"hash":"bac0d5760e8a8216a8fa04475dcd345c293716b8cb3780d4855b5b578d3eafef"},{"id":"func/Bridge.applyClarification","name":"Bridge.applyClarification","line":112,"end_line":117,"hash":"e31656ea20123ed7424523606ec1eb56608dda90579eef65ea7c73e613c67526"},{"id":"func/Bridge.carryOutClarification","name":"Bridge.carryOutClarification","line":121,"end_line":133,"hash":"3169fbe1073a82ee093075cdd846dcd2e9e873c828f1d77e54f4cdb4eec5188a"},{"id":"func/Bridge.postClarification","name":"Bridge.postClarification","line":137,"end_line":149,"hash":"c9f480820006752e79cd43229f04c8c6522ce422b2d57c1248969a10f35113d5"},{"id":"func/Bridge.reactClarification","name":"Bridge.reactClarification","line":153,"end_line":163,"hash":"05b42f82e2829109c53c6a513a373046c70a24cb627c6fa0ac325e00809c194a"},{"id":"func/Bridge.clarificationState","name":"Bridge.clarificationState","line":168,"end_line":172,"hash":"977f6e2d654d6eb9617e5d702196122d6941f84fef1c999310fa6e13fd0f162b"},{"id":"func/Bridge.answerClarification","name":"Bridge.answerClarification","line":176,"end_line":185,"hash":"b3a0478d1e9d7ed020f745003a796a118f0612d10dbad9c579abb8e2315f5096"},{"id":"func/Bridge.reportClarificationAnswer","name":"Bridge.reportClarificationAnswer","line":189,"end_line":199,"hash":"021c897f61f35e1238b5e903a50fefbf31b4fc042d75cddb1ba93eea55eee32d"},{"id":"func/Bridge.recordClarification","name":"Bridge.recordClarification","line":201,"end_line":206,"hash":"433830e3e38a3a899521b908a1cc247d040f8886124660b4f3a968c1defdfe71"}]}
// mutate4go-manifest-end
