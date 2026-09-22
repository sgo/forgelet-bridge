package steps

import (
	"context"
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
		if name, _, ok := board.ParseRow(line); ok && name == card {
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

// splitRows is the board's rows, without the empty one a trailing newline
// leaves behind.
func splitRows(data string) []string {
	trimmed := strings.TrimRight(data, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
