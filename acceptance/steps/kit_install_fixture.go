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

// theForgeHasNoProjectStructure takes the project's own structure away: no
// roles file and no board, which is a wrong path or a wrong forge rather than a
// forge that is merely quiet.
func theForgeHasNoProjectStructure(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	for _, project := range []string{"forgelet-bridge"} {
		for _, file := range []string{
			filepath.Join("roles.tsv"),
			filepath.Join("board", "tasks.tsv"),
		} {
			path := filepath.Join(store.Root(), "projects", project, ".swarmforge", file)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// theForgeHasBeenComposedButNeverStarted takes the forge back to the shape it
// arrives in: its composition and its projects are there, and nothing a start
// writes down is - no roles file, no board, no inbox and no socket anywhere.
// That is the shape a Forgelet forge has when the kit is installed into it
// before anything is started.
func theForgeHasBeenComposedButNeverStarted(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	root := store.Root()
	if err := os.RemoveAll(filepath.Join(root, ".swarmforge")); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(root, "projects"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, "projects", entry.Name(), ".swarmforge")); err != nil {
			return err
		}
	}
	return nil
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
