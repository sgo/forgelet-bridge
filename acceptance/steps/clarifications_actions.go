package steps

import (
	"context"
	"fmt"

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
	operator, err := w.operator(ctx)
	if err != nil {
		return "", "", "", err
	}
	err = waitFor(ctx, fmt.Sprintf("the clarifications room never carried the state event for %s", project), func() (bool, error) {
		id, content, found, err := operator.StateEvent(ctx, roomID, relay.ClarificationFactType, clarificationID(project))
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}
		eventID, quote = id, stateQuote(content)
		return true, nil
	})
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
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.SwipeReplyTo(ctx, roomID, eventID, quote, captures[1])
	return err
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
