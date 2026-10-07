package steps

import (
	"context"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// clarificationStateEvent is the state event the clarifications room carries for
// a project's question: the event a phone reads and acts on, keyed by the
// question's own id. The quote is what a phone shows for it when it swipes it.
func clarificationStateEvent(ctx context.Context, w *World, project string) (roomID, eventID, quote string, err error) {
	roomID, err = w.clarificationsRoom(ctx)
	if err != nil {
		return "", "", "", err
	}
	eventID, quote, err = stateEventToSwipe(ctx, w, roomID, relay.ClarificationFactType, clarificationID(project))
	return roomID, eventID, quote, err
}

// operatorRepliesToClarificationStateEvent sends the operator's answer as a
// reply that names the clarification's state event, which is the event a phone
// swipes.
func operatorRepliesToClarificationStateEvent(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, eventID, quote, err := clarificationStateEvent(ctx, w, captures[2])
	if err != nil {
		return err
	}
	return replyToStateEvent(ctx, w, roomID, eventID, quote, captures[1])
}

// operatorRepliesToClarification sends the operator's answer in the
// clarification's thread.
func operatorRepliesToClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return oneUserRepliesToClarification(ctx, w, w.operatorID, captures[1], captures[2])
}

// someoneRepliesToClarification sends a reply from any fixture client, so the
// allowlist can be checked.
func someoneRepliesToClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return oneUserRepliesToClarification(ctx, w, captures[1], captures[2], captures[3])
}

// oneUserRepliesToClarification has one Matrix user reply in the
// clarification's thread.
func oneUserRepliesToClarification(ctx context.Context, w *World, userID, answer, project string) error {
	roomID, messageID, _, err := w.clarificationMessage(ctx, project)
	if err != nil {
		return err
	}
	user, err := w.user(ctx, userID)
	if err != nil {
		return err
	}
	_, err = user.SendInThread(ctx, roomID, answer, messageID)
	return err
}

// operatorSwipesReplyToClarification sends the answer the way a phone does:
// by quoting the clarification message.
func operatorSwipesReplyToClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, messageID, _, err := w.clarificationMessage(ctx, captures[2])
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.SwipeReply(ctx, roomID, messageID, captures[1])
	return err
}

// operatorTalksInClarificationsRoom sends a plain message into the
// clarifications room, which must answer nothing.
func operatorTalksInClarificationsRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.clarificationsRoom(ctx)
	if err != nil {
		return err
	}
	return operatorSendsInto(ctx, w, roomID, captures[1])
}

// userJoinedClarificationsRoom brings a fixture client into the clarifications
// room, the way the operator would add someone.
func userJoinedClarificationsRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.clarificationsRoom(ctx)
	if err != nil {
		return err
	}
	return w.addUserToRoom(ctx, captures[1], roomID, "the clarifications room")
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-07T01:40:31+02:00","module_hash":"e5fc484210623a418c0099627c68e5afa5483fe06259f548f13515218fddef3b","functions":[{"id":"func/clarificationStateEvent","name":"clarificationStateEvent","line":12,"end_line":19,"hash":"c317becaf2bfffb6481acbbe7750338445b624c25d3d7c0f5ce04555db10f6f3"},{"id":"func/operatorRepliesToClarificationStateEvent","name":"operatorRepliesToClarificationStateEvent","line":24,"end_line":33,"hash":"2c9e86aef8d62f76b3d0617f9e32b942a47bfac70f4da2b4af4c174201cf2406"},{"id":"func/operatorRepliesToClarification","name":"operatorRepliesToClarification","line":37,"end_line":42,"hash":"8bbbe5126b115c8a054c73b79d6978a038fde760cb076321bc45a181940c9dd9"},{"id":"func/someoneRepliesToClarification","name":"someoneRepliesToClarification","line":46,"end_line":51,"hash":"daff45dfada4ad0733ae6d0a2dd9eb621348afb6c6b31f3c9c9dc764e538b3d2"},{"id":"func/oneUserRepliesToClarification","name":"oneUserRepliesToClarification","line":55,"end_line":66,"hash":"b96699da500690bdd06108fbee95d756a9321bf66356aaf964aaef18cb86b26e"},{"id":"func/operatorSwipesReplyToClarification","name":"operatorSwipesReplyToClarification","line":70,"end_line":84,"hash":"13d8fa190378c87852673c6d2052a90999ad6480cfeeedcd65c0a6009ad98994"},{"id":"func/operatorTalksInClarificationsRoom","name":"operatorTalksInClarificationsRoom","line":88,"end_line":97,"hash":"aba0571dc306731ec8c8e509bb7123723b528235756edeceb41c46ff719ad99c"},{"id":"func/userJoinedClarificationsRoom","name":"userJoinedClarificationsRoom","line":101,"end_line":110,"hash":"0e8323f1048c89e34b85214e85fcbc1ce37ebcd10a4f2963451a48bf2537ba27"}]}
// mutate4go-manifest-end
