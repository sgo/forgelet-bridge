package steps

import (
	"context"
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
	eventID, quote, err = stateEventToSwipe(ctx, w, roomID, relay.ApprovalFactType, approvalCardID(card))
	return roomID, eventID, quote, err
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
	return replyToStateEvent(ctx, w, roomID, eventID, quote, captures[1])
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-07T01:40:12+02:00","module_hash":"915f1627a02eb7dc8f47f1f41409579d9559864a7e949f28303505faee20a0b5","functions":[{"id":"func/approvalStateEvent","name":"approvalStateEvent","line":18,"end_line":25,"hash":"1dae417baa86049c822a1f503bfbb3fd0b324fe422d6d46f1fe333e8de2caafe"},{"id":"func/approvalStateEventTapped","name":"approvalStateEventTapped","line":30,"end_line":35,"hash":"5498437d1abe8bbb911018e618f1cb6dac4596de86ca5268f40a8890d8d76b1e"},{"id":"func/someoneReactsToTheApprovalStateEvent","name":"someoneReactsToTheApprovalStateEvent","line":39,"end_line":44,"hash":"8babf2fc690cee1b752d149cbcfc18c4614ed2551f403dd2d69038485edf593f"},{"id":"func/reactToApprovalStateEvent","name":"reactToApprovalStateEvent","line":48,"end_line":58,"hash":"731b494fb25fb7b78e64e327e7f7880bb3714fda7d45f0515269770fb0cf1c21"},{"id":"func/operatorRepliesToApprovalStateEvent","name":"operatorRepliesToApprovalStateEvent","line":62,"end_line":71,"hash":"5fb375d3303d81b76580455a956007f808c011a3cc8aa7f6924b2c6552183a2e"},{"id":"func/approvalMessageVariationSelectorTapped","name":"approvalMessageVariationSelectorTapped","line":75,"end_line":80,"hash":"cd576eb7e89bd2a6f59b43111e5e49210e266cddb903f074fdaecc5ecac34a2d"},{"id":"func/approvalTapped","name":"approvalTapped","line":83,"end_line":88,"hash":"b6b39ca49a322decd64ae765a955b43914b8b94808011f445f71e1acae79348c"},{"id":"func/someoneReacts","name":"someoneReacts","line":92,"end_line":102,"hash":"44e2249924d0b9aca74c62f2c6c14f7e00cefbeb0af7a44157f2eae0f8614fdf"},{"id":"func/reactToApproval","name":"reactToApproval","line":105,"end_line":115,"hash":"a9abd03fce0834b79003077b9a1e1841b8668757edac08fdb1d95f0c0a80e25b"},{"id":"func/operatorRepliesToApproval","name":"operatorRepliesToApproval","line":119,"end_line":135,"hash":"5b1245ed2fcd3bcade87ab0730378e2cded9156b83f93802d038a553cedf440d"},{"id":"func/operatorTalksInApprovalsRoom","name":"operatorTalksInApprovalsRoom","line":139,"end_line":148,"hash":"e9522705d61f40444441b71fa1a2b3eb1945b6c795ec22a6e5c1189528d848ca"},{"id":"func/operatorSendsInto","name":"operatorSendsInto","line":151,"end_line":158,"hash":"6856e4376f786f7a088cb6c87fc6a2415aa984a718e50ae01e8ec751a42616cc"},{"id":"func/operatorSwipesReplyToApproval","name":"operatorSwipesReplyToApproval","line":163,"end_line":177,"hash":"ab57be7af6b0a8c0cdcbd20c7576ab86204300b131baa5401a6fd08b2a489485"},{"id":"func/userJoinedApprovalsRoom","name":"userJoinedApprovalsRoom","line":181,"end_line":190,"hash":"befd09e46c4753d788e85d89741d027adf56ebfcc765a39f16de3768098030d6"},{"id":"func/gesturesAnswered","name":"gesturesAnswered","line":193,"end_line":217,"hash":"5edd3962d884fb036ce6e98fb819330d52b9b2ab71f01b79b3b5040e901f6926"}]}
// mutate4go-manifest-end
