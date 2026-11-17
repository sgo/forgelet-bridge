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
		return b.answerClarification(ctx, root, store, room, action)
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
func (b *Bridge) answerClarification(ctx context.Context, root string, store ClarificationStore, room Room, action relay.ClarificationAction) error {
	if b.state.Relay.Clarifications[action.Key].Answer != "" {
		return b.markClarificationAnswer(ctx, root, room, action)
	}
	if err := store.AnswerTyped(
		ctx, action.Clarification.Project, action.Clarification.ID, action.Answer); err != nil {
		return fmt.Errorf("answer %s through project chat: %w", action.Key, err)
	}
	b.recordClarification(root, action.Key, func(state relay.ClarificationState) relay.ClarificationState {
		state.Answer = action.Answer
		return state
	})
	return b.markClarificationAnswer(ctx, root, room, action)
}

// markClarificationAnswer puts the bridge's single carried-out mark on the
// notice before a later snapshot can retire the local record.
func (b *Bridge) markClarificationAnswer(ctx context.Context, root string, room Room, action relay.ClarificationAction) error {
	state := b.state.Relay.Clarifications[action.Key]
	if state.MessageID == "" || state.ReactionID != "" {
		return nil
	}
	return b.reactClarification(ctx, root, room, relay.ClarificationAction{
		Kind:      relay.ReactClarification,
		Key:       action.Key,
		MessageID: state.MessageID,
		Reaction:  relay.CarriedOutReaction,
	})
}

func (b *Bridge) recordClarification(root, key string, update func(relay.ClarificationState) relay.ClarificationState) {
	b.state.Relay.EnsureMaps()
	state := update(b.state.Relay.Clarifications[key])
	state.ForgeRoot = root
	b.state.Relay.Clarifications[key] = state
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-17T16:38:59+01:00","module_hash":"ff7607d85ac285f7bbc98a7d011c2c87110ca76351e01500b21ecf0ad915a691","functions":[{"id":"func/clarificationMessage","name":"clarificationMessage","line":14,"end_line":21,"hash":"bf629b53647781870cc687f0e3b611af29eaa93ea8200c62e142c80ab282e220"},{"id":"func/Bridge.carryOutClarifications","name":"Bridge.carryOutClarifications","line":25,"end_line":47,"hash":"52866d51495d563766b84f1f29f155a3fad5185fe31faccec410a09585ae2add"},{"id":"func/Bridge.clarificationInputsForTick","name":"Bridge.clarificationInputsForTick","line":51,"end_line":56,"hash":"d7bbaba5054468f039e691b3f5a50b95c9e1f24f28d17dfbed13f2af54d5f886"},{"id":"func/Bridge.carryOutPendingClarifications","name":"Bridge.carryOutPendingClarifications","line":60,"end_line":71,"hash":"9788b2a086a8b264c33d3740eaae223372070696fbcb43c80302c7f1027a8b65"},{"id":"func/clarificationInputs","name":"clarificationInputs","line":75,"end_line":83,"hash":"8863830b6146e2e3b3d38dd80efd56c41fca454a8c223a3adba80cef3dc157b5"},{"id":"func/Bridge.reconcileClarifications","name":"Bridge.reconcileClarifications","line":88,"end_line":103,"hash":"aa87ca1360ce640d3d8ac2822b6af359919b31f9c310bf79879bd68f0998c9d2"},{"id":"func/openTypedClarificationKeys","name":"openTypedClarificationKeys","line":105,"end_line":113,"hash":"05e1e520629df702874a91e4642c065664e07c0896457d823ff78b401a889d91"},{"id":"func/staleClarification","name":"staleClarification","line":115,"end_line":119,"hash":"c9abe70e579cbd4ae046e01b93fce4accadaea246366cbef9daa64819b9e61aa"},{"id":"func/clarificationFacts","name":"clarificationFacts","line":124,"end_line":130,"hash":"8f7bee29d22539209d8d5582da3a08cfdc5f50b0eb11695284b63a6153d000d6"},{"id":"func/Bridge.rememberClarificationStateEvents","name":"Bridge.rememberClarificationStateEvents","line":137,"end_line":152,"hash":"bac0d5760e8a8216a8fa04475dcd345c293716b8cb3780d4855b5b578d3eafef"},{"id":"func/Bridge.applyClarification","name":"Bridge.applyClarification","line":156,"end_line":161,"hash":"e31656ea20123ed7424523606ec1eb56608dda90579eef65ea7c73e613c67526"},{"id":"func/Bridge.carryOutClarification","name":"Bridge.carryOutClarification","line":165,"end_line":175,"hash":"7d9bb73722341ba3863f1fdec6aa8a3fcea9f7ea2b51d8b5e892c34ef61a6f60"},{"id":"func/Bridge.postClarification","name":"Bridge.postClarification","line":179,"end_line":192,"hash":"94d6c2eede647dca15869d2a76f9285b79d7f19ad239342b9258e2ca05743ad1"},{"id":"func/Bridge.reactClarification","name":"Bridge.reactClarification","line":196,"end_line":206,"hash":"05b42f82e2829109c53c6a513a373046c70a24cb627c6fa0ac325e00809c194a"},{"id":"func/Bridge.clarificationState","name":"Bridge.clarificationState","line":211,"end_line":215,"hash":"977f6e2d654d6eb9617e5d702196122d6941f84fef1c999310fa6e13fd0f162b"},{"id":"func/Bridge.answerClarification","name":"Bridge.answerClarification","line":219,"end_line":232,"hash":"5304ed2855929a75de78a56e39264f14e7f70cf7e29f5a4514465bb0084a89c3"},{"id":"func/Bridge.markClarificationAnswer","name":"Bridge.markClarificationAnswer","line":236,"end_line":247,"hash":"ca944d90e10b744737a23f6a51e410e2f54eb36a837793afb053c45e24249a45"},{"id":"func/Bridge.recordClarification","name":"Bridge.recordClarification","line":249,"end_line":254,"hash":"433830e3e38a3a899521b908a1cc247d040f8886124660b4f3a968c1defdfe71"}]}
// mutate4go-manifest-end
