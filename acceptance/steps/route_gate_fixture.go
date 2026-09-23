package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/board"
)

// routeGateCommand is this repository's route gate: the tool the suite runs,
// and the tool a forge's own copy is a deployment of.
const routeGateCommand = "route_card.sh"

// theForgeRoot is the one fixture forge root the scenario is working with.
func (w *World) theForgeRoot() (string, error) {
	if len(w.forgeRoots) != 1 {
		return "", fmt.Errorf("the scenario works with %d fixture forge roots, want exactly one here", len(w.forgeRoots))
	}
	return w.forgeRoots[0], nil
}

// forgeHoldsProject gives a fixture forge root a project of its own: the roles
// it serves, and the board a card would land on.
func forgeHoldsProject(ctx context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.forge(ctx, captures[1])
	if err != nil {
		return err
	}
	return setUpProject(store.Root(), captures[2])
}

// setUpProject gives a fixture forge root a project of its own: the roles it
// serves with their panes, the board a card would land on, and the forge's own
// scripts beside it.
func setUpProject(root, project string) error {
	dir := filepath.Join(root, "projects", project)
	for _, where := range []string{root, dir} {
		if err := writeFile(filepath.Join(where, ".swarmforge", "roles.tsv"), strings.Join([]string{
			"master\tmaster\t" + where + "\tfixture-master\tMaster\tcodex\ttask\tforward-only",
			"coder\tcoder\t" + filepath.Join(where, "worktrees", "coder") + "\tfixture-coder\tCoder\tcodex\ttask\tforward-only",
		}, "\n")+"\n"); err != nil {
			return err
		}
		if err := writeFile(filepath.Join(where, ".swarmforge", "board", "tasks.tsv"), ""); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "swarmforge"), 0o755); err != nil {
		return err
	}
	return markProjectOpen(root)
}

// fixtureBoardHolds reports whether a fixture project's board holds a card.
func fixtureBoardHolds(w *World, project, card string) (bool, error) {
	rows, err := fixtureBoardRows(w, project)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if name, _, ok := board.ParseRow(row); ok && name == card {
			return true, nil
		}
	}
	return false, nil
}

// fixtureBoardRows reads one fixture project's board, empty when it holds
// nothing yet.
func fixtureBoardRows(w *World, project string) ([]string, error) {
	root, err := w.theForgeRoot()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "projects", project, filepath.FromSlash(board.TasksFile)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

// forgePromptAsksLess gives a fixture forge a lieutenant prompt whose gate is
// looser than another forge's: the tool must behave the same.
func forgePromptAsksLess(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.theForgeRoot()
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(root, "swarmforge", captureRolePrompt(captures[1])),
		"Ask the operator only about fleet-wide or hard-to-reverse actions.\n")
}

// captureRolePrompt is the prompt file one role answers to, in the forge the
// role serves.
func captureRolePrompt(role string) string {
	return filepath.Join("roles", role+".prompt")
}
