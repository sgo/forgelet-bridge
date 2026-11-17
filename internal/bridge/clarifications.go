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

// carryOutClarifications brings the clarifications room and the typed project
// rows in step with each other, and reports how much work it did.
func (b *Bridge) carryOutClarifications(ctx context.Context, root string, store ClarificationStore, room Room, replies []relay.RoomEvent, snapshot relay.DashboardSnapshot) (carriedOut int, err error) {
	if store == nil {
		return 0, fmt.Errorf("no typed clarification answerer configured for forge root %s", root)
	}
	pending, err := b.clarificationInputsForTick(root, room, snapshot)
	if err != nil {
		return
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

// clarificationInputsForTick uses the snapshot's typed rows and first clears
// bridge records the authoritative snapshot retired.
func (b *Bridge) clarificationInputsForTick(root string, room Room, snapshot relay.DashboardSnapshot) ([]relay.Clarification, error) {
	if err := b.reconcileClarifications(root, room, snapshot.TypedClarifications); err != nil {
		return nil, err
	}
	return clarificationInputs(snapshot), nil
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

// clarificationInputs returns the open typed questions from the tick's
// snapshot.
func clarificationInputs(snapshot relay.DashboardSnapshot) []relay.Clarification {
	pending := make([]relay.Clarification, 0, len(snapshot.TypedClarifications))
	for _, clarification := range snapshot.TypedClarifications {
		if clarification.Lifecycle == relay.ClarificationOpen {
			pending = append(pending, clarification)
		}
	}
	return pending
}

// reconcileClarifications clears records the authoritative typed snapshot says
// are answered, withdrawn or no longer present. It also retires obsolete
// legacy bridge marks; Matrix messages and room state are not touched.
func (b *Bridge) reconcileClarifications(root string, room Room, typed []relay.Clarification) error {
	open := openTypedClarificationKeys(typed)
	var stale []string
	for key, state := range b.state.Relay.Clarifications {
		if staleClarification(state, root, room, open[key]) {
			stale = append(stale, key)
		}
	}
	if len(stale) == 0 {
		return nil
	}
	for _, key := range stale {
		delete(b.state.Relay.Clarifications, key)
	}
	return b.state.Save(b.statePath)
}

func openTypedClarificationKeys(typed []relay.Clarification) map[string]bool {
	open := make(map[string]bool, len(typed))
	for _, clarification := range typed {
		if clarification.Lifecycle == relay.ClarificationOpen {
			open[clarification.Key] = true
		}
	}
	return open
}

func staleClarification(state relay.ClarificationState, root string, room Room, open bool) bool {
	return state.ForgeRoot == root &&
		(state.RoomID == "" || state.RoomID == room.ClarificationsRoomID) &&
		!open
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
	if err := store.AnswerTyped(
		ctx, action.Clarification.Project, action.Clarification.ID, action.Answer); err != nil {
		return fmt.Errorf("answer %s through project chat: %w", action.Key, err)
	}
	b.recordClarification(root, action.Key, func(state relay.ClarificationState) relay.ClarificationState {
		state.Answer = action.Answer
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
// {"version":1,"tested_at":"2026-11-16T18:25:18+01:00","module_hash":"9c3ae13afb4e7b1b011d54cfeafa195825cf384de175a5a9c3a1bf38d101b2eb","functions":[{"id":"func/clarificationMessage","name":"clarificationMessage","line":14,"end_line":21,"hash":"bf629b53647781870cc687f0e3b611af29eaa93ea8200c62e142c80ab282e220"},{"id":"func/Bridge.carryOutClarifications","name":"Bridge.carryOutClarifications","line":25,"end_line":47,"hash":"6cb93827cec63c3805ee105056d4e41c1e0093a4c62681ad7449f26847733842"},{"id":"func/Bridge.clarificationInputsForTick","name":"Bridge.clarificationInputsForTick","line":51,"end_line":60,"hash":"322cc8df29fd093b396dbe1290670b3fae1426deb52d76a95b56780ffb215514"},{"id":"func/Bridge.carryOutPendingClarifications","name":"Bridge.carryOutPendingClarifications","line":64,"end_line":75,"hash":"9788b2a086a8b264c33d3740eaae223372070696fbcb43c80302c7f1027a8b65"},{"id":"func/clarificationInputs","name":"clarificationInputs","line":79,"end_line":94,"hash":"5d39a137513748a5b52070c23b39869cd260a3a6d4722abac44c0c7cd35f850f"},{"id":"func/Bridge.reconcileTypedClarifications","name":"Bridge.reconcileTypedClarifications","line":100,"end_line":115,"hash":"4871d4841021c15fb0018dec678c5f3e6da3c9afac1ab6bdf0edc36649e51d72"},{"id":"func/openTypedClarificationKeys","name":"openTypedClarificationKeys","line":117,"end_line":125,"hash":"05e1e520629df702874a91e4642c065664e07c0896457d823ff78b401a889d91"},{"id":"func/staleTypedClarification","name":"staleTypedClarification","line":127,"end_line":132,"hash":"926af51b2d987d9a43043f9f436183b294a7fbc833eb49131876a60c35bb5da7"},{"id":"func/clarificationFacts","name":"clarificationFacts","line":137,"end_line":143,"hash":"8f7bee29d22539209d8d5582da3a08cfdc5f50b0eb11695284b63a6153d000d6"},{"id":"func/Bridge.rememberClarificationStateEvents","name":"Bridge.rememberClarificationStateEvents","line":150,"end_line":165,"hash":"bac0d5760e8a8216a8fa04475dcd345c293716b8cb3780d4855b5b578d3eafef"},{"id":"func/Bridge.applyClarification","name":"Bridge.applyClarification","line":169,"end_line":174,"hash":"e31656ea20123ed7424523606ec1eb56608dda90579eef65ea7c73e613c67526"},{"id":"func/Bridge.carryOutClarification","name":"Bridge.carryOutClarification","line":178,"end_line":190,"hash":"3169fbe1073a82ee093075cdd846dcd2e9e873c828f1d77e54f4cdb4eec5188a"},{"id":"func/Bridge.postClarification","name":"Bridge.postClarification","line":194,"end_line":207,"hash":"94d6c2eede647dca15869d2a76f9285b79d7f19ad239342b9258e2ca05743ad1"},{"id":"func/Bridge.reactClarification","name":"Bridge.reactClarification","line":211,"end_line":221,"hash":"05b42f82e2829109c53c6a513a373046c70a24cb627c6fa0ac325e00809c194a"},{"id":"func/Bridge.clarificationState","name":"Bridge.clarificationState","line":226,"end_line":230,"hash":"977f6e2d654d6eb9617e5d702196122d6941f84fef1c999310fa6e13fd0f162b"},{"id":"func/Bridge.answerClarification","name":"Bridge.answerClarification","line":234,"end_line":252,"hash":"1c45f7292087018203b20e70029e0686ee6c2b3e52fe0224870364de211befae"},{"id":"func/Bridge.reportClarificationAnswer","name":"Bridge.reportClarificationAnswer","line":256,"end_line":266,"hash":"021c897f61f35e1238b5e903a50fefbf31b4fc042d75cddb1ba93eea55eee32d"},{"id":"func/Bridge.recordClarification","name":"Bridge.recordClarification","line":268,"end_line":273,"hash":"433830e3e38a3a899521b908a1cc247d040f8886124660b4f3a968c1defdfe71"}]}
// mutate4go-manifest-end
