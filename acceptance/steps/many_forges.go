package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// dashboardOf is the running dashboard of a fixture forge.
func (w *World) dashboardOf(name string) (*fixtures.Dashboard, error) {
	if _, err := w.declaredForge(name); err != nil {
		return nil, err
	}
	running, ok := w.running[name]
	if !ok {
		return nil, fmt.Errorf("the fixture forge root %s does not have its dashboard running", name)
	}
	return running, nil
}

// dashboardStopped stops one forge's dashboard, leaving the bridge with a
// forge it cannot reach while the others keep working.
func dashboardStopped(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	running, err := w.dashboardOf(captures[1])
	if err != nil {
		return err
	}
	running.Stop()
	delete(w.running, captures[1])
	return nil
}

// dashboardStartedAgain brings one forge's dashboard back. The address it
// announced before is dropped first, so the fixture waits for the address the
// dashboard announces now rather than reading the stale one.
func dashboardStartedAgain(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(store.Root(), filepath.FromSlash(dashboard.URLFile))); err != nil && !os.IsNotExist(err) {
		return err
	}
	return w.startDashboard(captures[1])
}

// statusNamesTheOnlyUnhappyForge waits for the bridge to report one named
// forge as the only one it cannot serve.
func statusNamesTheOnlyUnhappyForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	name := captures[1]
	return waitFor(ctx, fmt.Sprintf("the bridge's status never named %s as the only unhappy forge", name), func() (bool, error) {
		status, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
		if err != nil {
			return false, nil
		}
		return len(status.UnhappyForges) == 1 && status.UnhappyForges[0] == name, nil
	})
}

// statusNamesNoUnhappyForge waits for the bridge to report every forge served.
func statusNamesNoUnhappyForge(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return waitFor(ctx, "the bridge's status never stopped naming an unhappy forge", func() (bool, error) {
		status, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
		if err != nil {
			return false, nil
		}
		return len(status.UnhappyForges) == 0, nil
	})
}

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
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.Send(ctx, roomID, captures[1])
	return err
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

// forgeApprovalMessage finds the approval message one named forge's room holds
// for a card.
func (w *World) forgeApprovalMessage(ctx context.Context, forgeName, card string) (roomID, messageID, body string, err error) {
	roomID, err = w.waitForSpaceChild(ctx, forgeName, config.ApprovalsRoomName)
	if err != nil {
		return "", "", "", err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return "", "", "", err
	}
	start := approvalMessageStart(card)
	err = waitFor(ctx, fmt.Sprintf("the operator never saw the approval message for %s in %s", card, forgeName), func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			if strings.HasPrefix(message.Body, start) {
				messageID, body = message.EventID, message.Body
				return true, nil
			}
		}
		return false, nil
	})
	return roomID, messageID, body, err
}

// waitForCardUpdateIn waits for a card update the operator can read in a room.
func waitForCardUpdateIn(ctx context.Context, w *World, roomID, card, want string) error {
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("the operator never read the update for %s", card), func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			if message.Body != want {
				continue
			}
			if !message.Encrypted {
				return false, fmt.Errorf("the card update %q was not decrypted from an encrypted event", want)
			}
			return true, nil
		}
		return false, nil
	})
}
