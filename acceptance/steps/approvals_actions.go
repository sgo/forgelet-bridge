package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// approveWithVariationSelector is the check mark one side sends with the
// variation selector and the other without. The bridge counts either, so a mark
// the room reads as a decision is one it acts on.
const approveWithVariationSelector = "\u2705\ufe0f"

// approvalStateEvent is the state event the approvals room carries for a card's
// approval: the event a phone reads and acts on, keyed by the approval's own id.
// The quote is what a phone shows for it when it swipes the event.
func approvalStateEvent(ctx context.Context, w *World, card string) (roomID, eventID, quote string, err error) {
	roomID, err = w.approvalsRoom(ctx)
	if err != nil {
		return "", "", "", err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return "", "", "", err
	}
	err = waitFor(ctx, fmt.Sprintf("the approvals room never carried the state event for %s", card), func() (bool, error) {
		id, content, found, err := operator.StateEvent(ctx, roomID, relay.ApprovalFactType, approvalCardID(card))
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

// stateQuote is what a phone shows for a state event it swipes: the event's own
// facts, as the JSON the room holds. The bridge reads only the operator's own
// words out of a reply, so this is the client's own rendering of the quote.
func stateQuote(content map[string]any) string {
	data, err := json.Marshal(content)
	if err != nil {
		return "the room's state event"
	}
	return string(data)
}

// approvalStateEventTapped reacts to the approval's state event, the way the
// phone does: its reading is the room's state, so the event it can act on is the
// state event rather than the message Element shows.
func approvalStateEventTapped(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return reactToApprovalStateEvent(ctx, w, w.operatorID, "✅", captures[1])
}

// someoneReactsToTheApprovalStateEvent sends the same mark from another client,
// which must decide nothing.
func someoneReactsToTheApprovalStateEvent(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return reactToApprovalStateEvent(ctx, w, captures[1], "✅", captures[2])
}

// reactToApprovalStateEvent reacts to the state event the room carries for a
// card's approval.
func reactToApprovalStateEvent(ctx context.Context, w *World, userID, reaction, card string) error {
	roomID, eventID, _, err := approvalStateEvent(ctx, w, card)
	if err != nil {
		return err
	}
	user, err := w.user(ctx, userID)
	if err != nil {
		return err
	}
	return user.React(ctx, roomID, eventID, reaction)
}

// operatorRepliesToApprovalStateEvent sends the operator's decision as a reply
// that names the approval's state event, which is the event a phone swipes.
func operatorRepliesToApprovalStateEvent(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, eventID, quote, err := approvalStateEvent(ctx, w, captures[2])
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

// approvalMessageVariationSelectorTapped taps the check mark with the variation
// selector on the approval's message, the form one side sends for the mark.
func approvalMessageVariationSelectorTapped(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return reactToApproval(ctx, w, w.operatorID, approveWithVariationSelector, captures[1])
}

// approvalTapped sends the operator's approval reaction.
func approvalTapped(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return reactToApproval(ctx, w, w.operatorID, "✅", captures[1])
}

// someoneReacts sends a reaction from any fixture client, so the allowlist can
// be checked.
func someoneReacts(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	reactor, reaction, card := captures[1], captures[2], captures[3]
	userID := w.operatorID
	if reactor != "the operator" {
		userID = reactor
	}
	return reactToApproval(ctx, w, userID, reaction, card)
}

// reactToApproval reacts to the approval message of a card.
func reactToApproval(ctx context.Context, w *World, userID, reaction, card string) error {
	roomID, messageID, _, err := w.approvalMessage(ctx, card)
	if err != nil {
		return err
	}
	user, err := w.user(ctx, userID)
	if err != nil {
		return err
	}
	return user.React(ctx, roomID, messageID, reaction)
}

// operatorRepliesToApproval sends the operator's feedback in the approval's
// thread.
func operatorRepliesToApproval(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	feedback, card := captures[1], captures[2]

	roomID, messageID, _, err := w.approvalMessage(ctx, card)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.SendInThread(ctx, roomID, feedback, messageID)
	return err
}

// operatorTalksInApprovalsRoom sends a plain message into the approvals room,
// which must decide nothing.
func operatorTalksInApprovalsRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.approvalsRoom(ctx)
	if err != nil {
		return err
	}
	return operatorSendsInto(ctx, w, roomID, captures[1])
}

// operatorSendsInto has the operator send a plain message into a room.
func operatorSendsInto(ctx context.Context, w *World, roomID, body string) error {
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.Send(ctx, roomID, body)
	return err
}

// operatorSwipesReplyToApproval sends the operator's decision the way a phone
// does: by quoting the approval message, which counts as a reply in its thread
// once the quote the phone wrote into the body is left out.
func operatorSwipesReplyToApproval(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, messageID, _, err := w.approvalMessage(ctx, captures[2])
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

// userJoinedApprovalsRoom brings a fixture client into the approvals room, the
// way the operator would add someone.
func userJoinedApprovalsRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.approvalsRoom(ctx)
	if err != nil {
		return err
	}
	return w.addUserToRoom(ctx, captures[1], roomID, "the approvals room")
}

// gesturesAnswered checks the room told the operator what it can read.
func gesturesAnswered(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.approvalsRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	return waitFor(ctx, "the bridge never answered with the gestures the room takes", func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			if message.Sender != w.bridgeUserID {
				continue
			}
			text := strings.ToLower(message.Body)
			if strings.Contains(text, "approve") && strings.Contains(text, "✅") {
				return true, nil
			}
		}
		return false, nil
	})
}
