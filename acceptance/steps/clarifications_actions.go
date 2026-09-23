package steps

import (
	"context"
)

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
