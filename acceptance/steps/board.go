package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unclebob/forgelet-bridge/internal/board"
)

// boardHoldsCard puts a card on the forge's board in a lane, the way the
// forge's own board does.
func boardHoldsCard(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	card, project, lane := captures[1], captures[2], captures[3]
	root, err := boardForge(w)
	if err != nil {
		return err
	}
	if err := markProjectOpen(root); err != nil {
		return err
	}
	return setCardLane(root, project, card, lane)
}

// boardHoldsCards puts several cards on the forge's board in one lane, in one
// write, so a tick sees them all appear together.
func boardHoldsCards(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	project, lane := captures[2], captures[3]
	root, err := boardForge(w)
	if err != nil {
		return err
	}
	if err := markProjectOpen(root); err != nil {
		return err
	}
	changes := make([]cardLane, 0, len(forgeNames(captures[1])))
	for _, card := range forgeNames(captures[1]) {
		changes = append(changes, cardLane{card, lane})
	}
	return setCardLanes(root, project, changes)
}

// forgeMovesAndFinishes writes two changes in one board: the card that moved on
// and the card that finished, so one tick carries both.
func forgeMovesAndFinishes(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	moved, lane, finished := captures[1], captures[2], captures[3]
	root, err := boardForge(w)
	if err != nil {
		return err
	}
	return setCardLanes(root, approvalProject, []cardLane{{moved, lane}, {finished, board.DoneLane}})
}

// forgeMovesCard moves a card to another lane.
func forgeMovesCard(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := boardForge(w)
	if err != nil {
		return err
	}
	return setCardLane(root, approvalProject, captures[1], captures[2])
}

// forgeFinishesCard moves a card to the done lane, which is how the board
// records that a card finished.
func forgeFinishesCard(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := boardForge(w)
	if err != nil {
		return err
	}
	return setCardLane(root, approvalProject, captures[1], board.DoneLane)
}

// boardForge is the one forge a step works with when the scenario is about that
// forge's own files: the forge the bridge serves when it serves one, and
// otherwise the one fixture forge the scenario declared. A scenario about a
// forge's board, or its dashboard queue, need not run the bridge at all.
func boardForge(w *World) (string, error) {
	if len(w.configured) == 1 {
		return w.configured[0], nil
	}
	if len(w.forgeRoots) == 1 {
		return w.forgeRoots[0], nil
	}
	return "", fmt.Errorf("the scenario works with %d configured and %d declared forge roots, want exactly one for a board",
		len(w.configured), len(w.forgeRoots))
}

// setCardLane writes one card into a project's board in the lane it is in,
// keeping the board's row shape: name, lane, timestamps, task id, audits.
func setCardLane(root, project, card, lane string) error {
	return setCardLanes(root, project, []cardLane{{card, lane}})
}

// cardLane is one card and the lane it belongs in.
type cardLane struct {
	card string
	lane string
}

// setCardLanes writes several cards into a project's board in one write, so a
// tick sees every one of those changes at once.
func setCardLanes(root, project string, changes []cardLane) error {
	path := filepath.Join(root, "projects", project, filepath.FromSlash(board.TasksFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows := splitRows(string(existing))
	for _, change := range changes {
		row := strings.Join([]string{change.card, change.lane, now, now, approvalTaskID(change.card), "0"}, "\t")
		replaced := false
		for index, line := range rows {
			if name, _, ok := board.ParseRow(line); ok && name == change.card {
				rows[index] = row
				replaced = true
				break
			}
		}
		if !replaced {
			rows = append(rows, row)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(rows, "\n")+"\n"), 0o644)
}

// splitRows is the board's rows, without the empty one a trailing newline
// leaves behind.
func splitRows(data string) []string {
	trimmed := strings.TrimRight(data, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
