package steps

import (
	"context"
)

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
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.Send(ctx, roomID, captures[1])
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
