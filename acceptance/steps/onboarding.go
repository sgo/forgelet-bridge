package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/board"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
)

// theForgeBoardHoldsFinishedCards gives the forge's open project a board of
// cards that finished before the bridge got to it: the history a forge arrives
// with, which must not be narrated back to the operator.
func theForgeBoardHoldsFinishedCards(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := boardForge(w)
	if err != nil {
		return err
	}
	count, err := countOfCaptures(captures, 1)
	if err != nil {
		return err
	}
	if err := markProjectOpen(root); err != nil {
		return err
	}
	for index := 1; index <= count; index++ {
		card := fmt.Sprintf("finished-card-%d", index)
		if err := setCardLane(root, approvalProject, card, board.DoneLane); err != nil {
			return err
		}
	}
	return nil
}

// theForgeHoldsPendingChatRequests fills the forge's dashboard queue with the
// requests the bridge has still to post: a backlog that is the forge's own,
// separate from whether the bridge reached it.
func theForgeHoldsPendingChatRequests(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := boardForge(w)
	if err != nil {
		return err
	}
	store := w.dashboards[filepath.Base(root)]
	if store == nil {
		return fmt.Errorf("the fixture forge root %s has no dashboard queue", root)
	}
	count, err := countOfCaptures(captures, 1)
	if err != nil {
		return err
	}
	for index := 1; index <= count; index++ {
		if _, err := store.CreateRequest(fmt.Sprintf("request %d", index)); err != nil {
			return err
		}
	}
	return nil
}

// theForgeBoardCannotBeRead puts a directory where the board file goes: a board
// the bridge cannot read at all, which is the failure the rest of a forge's
// turn has to survive.
func theForgeBoardCannotBeRead(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	root, err := boardForge(w)
	if err != nil {
		return err
	}
	path := filepath.Join(root, "projects", approvalProject, filepath.FromSlash(board.TasksFile))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	return markProjectOpen(root)
}

// statusReportsTheWorkItStillOwes checks the status says what work the bridge
// has still to carry out for the forge, alongside naming it reached: a queue
// behind a forge is a separate, visible thing rather than proof that reaching
// it failed.
func statusReportsTheWorkItStillOwes(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if len(w.configured) != 1 {
		return fmt.Errorf("the status step works with one configured forge root, this scenario has %d", len(w.configured))
	}
	name := filepath.Base(w.configured[0])
	if named := w.forgeNames[w.configured[0]]; named != "" {
		name = named
	}
	ctx, cancel := stepContext()
	defer cancel()
	path := filepath.Join(w.stateDir, bridge.StatusName)
	return waitFor(ctx, "the bridge's status never said what work the forge still owes", func() (bool, error) {
		status, err := readStatus(path)
		if err != nil {
			return false, nil
		}
		if !fixtures.Contains(status.ReachedForges, name) {
			return false, nil
		}
		return owedFor(status.Owed, name), nil
	})
}

// owedFor reports whether the status carries what one forge still owes.
func owedFor(owed []bridge.ForgeOwed, name string) bool {
	for _, forge := range owed {
		if forge.Name == name {
			return true
		}
	}
	return false
}
