package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/board"
)

// lieutenantPrompt is the wording a fixture forge wrote for its own gate: this
// forge asks about every card, which is its policy and not the installer's.
const lieutenantPrompt = "Ask the operator before a card is created, every time: this forge is stricter than the pack's own rule.\n"

// forgeCarriesItsOwnLieutenantPrompt gives a fixture forge the prompt its gate
// answers to, which is where its strictness lives.
func forgeCarriesItsOwnLieutenantPrompt(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	// The prompt comes before the project in the scenario, so the fixture
	// declares the forge here rather than reading one that is not there yet.
	store, err := w.forge(context.Background(), captures[1])
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(store.Root(), "swarmforge", "roles", "lieutenant.prompt"), lieutenantPrompt)
}

// projectKeepsANoteWaitingToBePickedUp puts the card's note in the lane it is
// in, the way the forge hands work over: mail the role has not taken up.
func projectKeepsANoteWaitingToBePickedUp(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	projectDir, err := w.pathOf(captures[2], captures[1])
	if err != nil {
		return err
	}
	card, lane, err := firstCard(projectDir)
	if err != nil {
		return err
	}
	worktree, err := roleWorktree(projectDir, lane)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(worktree, ".swarmforge", "handoffs", "inbox", "new", "50_"+card+".handoff"),
		"task: "+card+"\n")
}

// firstCard is the first card a project's board holds, and the lane it is in.
func firstCard(projectDir string) (string, string, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(board.TasksFile)))
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if name, lane, ok := board.ParseRow(line); ok {
			return name, lane, nil
		}
	}
	return "", "", fmt.Errorf("the project %s holds no card to keep a note for", projectDir)
}
