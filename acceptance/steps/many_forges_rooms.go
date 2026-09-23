package steps

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/config"
)

// forgeBoardHoldsCard puts a card on one named forge's board in a lane.
func forgeBoardHoldsCard(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	if err := markProjectOpen(store.Root()); err != nil {
		return err
	}
	return setCardLane(store.Root(), captures[3], captures[2], captures[4])
}

// forgeCardAppearedSeen waits for the update in one named forge's activity room
// that says a card appeared.
func forgeCardAppearedSeen(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	want := fmt.Sprintf("card %s appeared in the project %s in the lane %s", captures[2], captures[3], captures[4])
	roomID, err := w.waitForSpaceChild(ctx, captures[1], config.ActivityRoomName)
	if err != nil {
		return err
	}
	return waitForCardUpdateIn(ctx, w, roomID, captures[2], want)
}

// operatorSendsIntoForgeChatRoom sends the operator's message into one named
// forge's chat room.
func operatorSendsIntoForgeChatRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.forgeChatRoom(ctx, captures[2])
	if err != nil {
		return err
	}
	return operatorSendsInto(ctx, w, roomID, captures[1])
}

// forgeHoldsTheRequestTheDashboardTook checks both halves of who wrote a
// request for one named forge: the forge holds it, and the forge's dashboard is
// the one that typed it into the lieutenant's pane.
func forgeHoldsTheRequestTheDashboardTook(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	if err := waitFor(ctx, fmt.Sprintf("the forge %s never held the chat request %q", captures[1], captures[2]), func() (bool, error) {
		_, found, err := store.RequestForBody(captures[2])
		return found, err
	}); err != nil {
		return err
	}
	running, err := w.dashboardOf(captures[1])
	if err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("the dashboard of %s never typed the chat request %q into the lieutenant's pane", captures[1], captures[2]), func() (bool, error) {
		return running.WokeWith(captures[2])
	})
}

// forgeApprovalCannotBeSentBack seeds an approval in one named forge whose
// send-back the forge will refuse.
func forgeApprovalCannotBeSentBack(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	return seedApprovalIn(store.Root(), captures[2], false)
}

// forgeRepairsItsApproval gives one named forge's approval a commit it has,
// which is what "the forge repaired it" means from the bridge's side.
func forgeRepairsItsApproval(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	return seedApprovalIn(store.Root(), captures[2], true)
}

// operatorRepliesToForgeApproval sends the operator's feedback in the thread of
// an approval one named forge is showing.
func operatorRepliesToForgeApproval(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, messageID, _, err := w.forgeApprovalMessage(ctx, captures[3], captures[2])
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.SendInThread(ctx, roomID, captures[1], messageID)
	return err
}

// operatorDecryptsForgeApprovalReply waits for the bridge's reply in the thread
// of an approval one named forge is showing.
func operatorDecryptsForgeApprovalReply(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	body, card, forgeName := captures[1], captures[2], captures[3]
	roomID, messageID, _, err := w.forgeApprovalMessage(ctx, forgeName, card)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	reply, err := operator.WaitForThreadReply(ctx, roomID, messageID, body, stepTimeout)
	if err != nil {
		return err
	}
	if !reply.Encrypted {
		return fmt.Errorf("the approval reply %q was not decrypted from an encrypted event", body)
	}
	return nil
}
