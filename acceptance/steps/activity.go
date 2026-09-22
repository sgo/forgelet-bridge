package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unclebob/forgelet-bridge/internal/board"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/config"
)

// quietTicks is how many bridge ticks a quiet stretch covers: long enough that
// a heartbeat would have shown up, short enough to keep the suite quick.
const quietTicks = 10

// boardHoldsCard puts a card on the forge's board in a lane, the way the
// forge's own board does.
func boardHoldsCard(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	card, project, lane := captures[1], captures[2], captures[3]
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	if err := markProjectOpen(root); err != nil {
		return err
	}
	return setCardLane(root, project, card, lane)
}

// forgeMovesCard moves a card to another lane.
func forgeMovesCard(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	return setCardLane(root, approvalProject, captures[1], captures[2])
}

// forgeFinishesCard moves a card to the done lane, which is how the board
// records that a card finished.
func forgeFinishesCard(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	return setCardLane(root, approvalProject, captures[1], board.DoneLane)
}

// setCardLane writes one card into a project's board in the lane it is in,
// keeping the board's row shape: name, lane, timestamps, task id, audits.
func setCardLane(root, project, card, lane string) error {
	path := filepath.Join(root, "projects", project, filepath.FromSlash(board.TasksFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row := strings.Join([]string{card, lane, now, now, approvalTaskID(card), "0"}, "\t")

	rows := splitRows(string(existing))
	replaced := false
	for index, line := range rows {
		if name, _, ok := cardRowOf(line); ok && name == card {
			rows[index] = row
			replaced = true
			break
		}
	}
	if !replaced {
		rows = append(rows, row)
	}
	return os.WriteFile(path, []byte(strings.Join(rows, "\n")+"\n"), 0o644)
}

func splitRows(data string) []string {
	trimmed := strings.TrimRight(data, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// cardRowOf reads a board row the way the board's own tools do.
func cardRowOf(line string) (name, lane string, ok bool) {
	columns := strings.Split(line, "\t")
	if len(columns) < 2 {
		return "", "", false
	}
	name, lane = strings.TrimSpace(columns[0]), strings.TrimSpace(columns[1])
	return name, lane, name != "" && lane != ""
}

// activityRoom is the activity room of the configured forge.
func (w *World) activityRoom(ctx context.Context) (string, error) {
	space, err := w.forgeSpace()
	if err != nil {
		return "", err
	}
	return w.waitForSpaceChild(ctx, space, config.ActivityRoomName)
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
	count := func() int {
		found := 0
		for _, message := range operator.Messages(roomID) {
			if strings.HasPrefix(message.Body, "card ") {
				found++
			}
		}
		return found
	}
	want := captures[1]
	if err := waitFor(ctx, "the activity room never held the card updates", func() (bool, error) {
		return fmt.Sprintf("%d", count()) >= want, nil
	}); err != nil {
		return err
	}
	if found := count(); fmt.Sprintf("%d", found) != want {
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
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.Send(ctx, roomID, captures[1])
	return err
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
