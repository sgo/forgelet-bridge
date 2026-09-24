package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/config"
)

// quietTicks is how many bridge ticks a quiet stretch covers: long enough that
// a heartbeat would have shown up, short enough to keep the suite quick.
const quietTicks = 10

// activityRoom is the activity room of the configured forge.
func (w *World) activityRoom(ctx context.Context) (string, error) {
	return w.roomNamed(ctx, config.ActivityRoomName)
}

// cardAppearedSeen waits for the update that says a card appeared.
func cardAppearedSeen(_ context.Context, world any, captures []string) error {
	card, project, lane := captures[1], captures[2], captures[3]
	want := fmt.Sprintf("card %s appeared in the project %s in the lane %s", card, project, lane)
	return waitForCardUpdate(world.(*World), card, want)
}

// cardMovedSeen waits for the update that says a card moved on.
func cardMovedSeen(_ context.Context, world any, captures []string) error {
	card, project, lane := captures[1], captures[2], captures[3]
	want := fmt.Sprintf("card %s moved on in the project %s to the lane %s", card, project, lane)
	return waitForCardUpdate(world.(*World), card, want)
}

// cardFinishedSeen waits for the update that says a card finished.
func cardFinishedSeen(_ context.Context, world any, captures []string) error {
	card, project := captures[1], captures[2]
	want := fmt.Sprintf("card %s finished in the project %s", card, project)
	return waitForCardUpdate(world.(*World), card, want)
}

// waitForCardUpdate waits for a card update the operator can read.
func waitForCardUpdate(w *World, card, want string) error {
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.activityRoom(ctx)
	if err != nil {
		return err
	}
	return waitForCardUpdateIn(ctx, w, roomID, card, want)
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

// cardUpdatesForTheTick waits for one card update that names every card the tick
// carried, which is what a tick's news batched into one message looks like.
func cardUpdatesForTheTick(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.activityRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	cards := forgeNames(captures[1])
	return waitFor(ctx, fmt.Sprintf("the activity room never carried one update naming %s", strings.Join(cards, ", ")), func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			names := true
			for _, card := range cards {
				if !strings.Contains(message.Body, card) {
					names = false
					break
				}
			}
			if names {
				return true, nil
			}
		}
		return false, nil
	})
}

// restartAddedNoCardUpdate checks the room holds no more updates than it did
// before the restart: the news a tick already delivered is never posted again.
func restartAddedNoCardUpdate(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.activityRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	if found, err := cardUpdatesIn(operator, roomID); err != nil {
		return err
	} else if found != w.activityCount {
		return fmt.Errorf("the restart added card updates: the room holds %d, and it held %d", found, w.activityCount)
	}
	return nil
}

// cardUpdateFinishedAsAMessage checks a card's finish was sent as an ordinary
// message, the kind clients notify on.
func cardUpdateFinishedAsAMessage(_ context.Context, world any, captures []string) error {
	return cardUpdateSentAs(world.(*World), captures[1], "finished", "m.text")
}

// cardUpdateMovedOnAsANotice checks a card's lane move was sent as a notice,
// which does not notify: sharing a tick with the news never promotes it.
func cardUpdateMovedOnAsANotice(_ context.Context, world any, captures []string) error {
	return cardUpdateSentAs(world.(*World), captures[1], "moved on", "m.notice")
}

// cardUpdateSentAs checks one card's update in the activity room went out as the
// kind of message the scenario says it should be.
func cardUpdateSentAs(w *World, card, marker, msgType string) error {
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.activityRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	for _, message := range operator.Messages(roomID) {
		if !strings.Contains(message.Body, "card "+card+" ") || !strings.Contains(message.Body, marker) {
			continue
		}
		if message.MsgType != msgType {
			return fmt.Errorf("the update %q was sent as %q, want %q", message.Body, message.MsgType, msgType)
		}
		return nil
	}
	return fmt.Errorf("the activity room never carried the update saying the card %s %s", card, marker)
}

// cardUpdatesIn counts the card updates one room holds.
func cardUpdatesIn(operator *fixtures.User, roomID string) (int, error) {
	found := 0
	for _, message := range operator.Messages(roomID) {
		if strings.HasPrefix(message.Body, "card ") {
			found++
		}
	}
	return found, nil
}

// cardUpdateCount checks how many card updates the activity room holds.
// cardUpdatesNotifyForTheNews checks which of a card's updates would buzz the
// operator's phone: a card arriving and a card finishing are ordinary messages,
// which clients notify on, while the routine lane-to-lane step is posted as the
// kind of event clients do not notify on. Which is which belongs in the spec
// rather than in a client setting, so it is pinned here.
func cardUpdatesNotifyForTheNews(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.activityRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	card := captures[1]
	// What each update is, and whether it is news worth waking anyone for.
	news := []struct {
		marker string
		notify bool
	}{
		{"appeared", true},
		{"moved on", false},
		{"finished", true},
	}
	found := map[string]bool{}
	for _, message := range operator.Messages(roomID) {
		if !strings.HasPrefix(message.Body, "card "+card+" ") {
			continue
		}
		for _, update := range news {
			if !strings.Contains(message.Body, update.marker) {
				continue
			}
			found[update.marker] = true
			notifies := message.MsgType == "m.text"
			if notifies != update.notify {
				return fmt.Errorf("the update %q was sent as %q: want the %s to %s",
					message.Body, message.MsgType, update.marker, notifyWording(update.notify))
			}
		}
	}
	for _, update := range news {
		if !found[update.marker] {
			return fmt.Errorf("the activity room never carried the update saying the card %s %s", card, update.marker)
		}
	}
	return nil
}

// notifyWording says what a kind of update should do to the operator's phone.
func notifyWording(notify bool) string {
	if notify {
		return "notify"
	}
	return "stay quiet"
}

// cardUpdateCount checks how many card updates the activity room holds.
func cardUpdateCount(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.activityRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	want := captures[1]
	if err := waitFor(ctx, "the activity room never held the card updates", func() (bool, error) {
		found, err := cardUpdatesIn(operator, roomID)
		if err != nil {
			return false, err
		}
		return fmt.Sprintf("%d", found) >= want, nil
	}); err != nil {
		return err
	}
	found, err := cardUpdatesIn(operator, roomID)
	if err != nil {
		return err
	}
	// What the room holds now is what a later restart must not add to.
	w.activityCount = found
	if fmt.Sprintf("%d", found) != want {
		return fmt.Errorf("the activity room holds %d card updates, want %s", found, want)
	}
	return nil
}

// approvalsRoomSilent checks the approvals room never got the card activity.
func approvalsRoomSilent(_ context.Context, world any, _ []string) error {
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
	if messages := operator.Messages(roomID); len(messages) != 0 {
		return fmt.Errorf("the approvals room holds %d messages, want none", len(messages))
	}
	return nil
}

// operatorTalksInActivityRoom sends a plain message into the activity room: it
// is a log to keep quiet, not a channel to work in.
func operatorTalksInActivityRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.activityRoom(ctx)
	if err != nil {
		return err
	}
	return operatorSendsInto(ctx, w, roomID, captures[1])
}

// quietStretch lets the bridge run on with nothing changing. A heartbeat would
// show up as extra updates in the counts that follow.
func quietStretch(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	path := filepath.Join(w.stateDir, bridge.StatusName)

	before, err := readStatus(path)
	if err != nil {
		return err
	}
	return waitFor(ctx, "the bridge never ran on through the quiet stretch", func() (bool, error) {
		status, err := readStatus(path)
		if err != nil {
			return false, nil
		}
		return status.Tick >= before.Tick+quietTicks, nil
	})
}
