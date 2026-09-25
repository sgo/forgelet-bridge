package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/board"
)

// lostTheClause is what a doorbell reads when it never carried the clause: the
// words its ring says about the gate are replaced by words that say nothing of
// the kind, and the tool still rings. The wording is replaced rather than the
// code that carries it, so the install meets a doorbell that lost its words
// rather than one that cannot run.
var lostTheClause = strings.NewReplacer(
	"the gate is the operator's", "the gate belongs to somebody",
	"Do not approve unless the operator says to.", "Approve it if you think it is right.",
	"the operator's to give", "somebody's to give",
	"Do not answer it", "Answer it yourself",
	"unless the operator says to.", "when you think it is right.",
)

// kitCopy is the kit the next install runs from, when a scenario has given the
// kit itself something: the project's own tools copied into the scenario's
// directory, so what a scenario changes is a copy rather than the tree the
// suite runs from. Its modes come with it, which is what the installer carries.
func (w *World) kitCopy() (string, error) {
	if w.kitDir != "" {
		return w.kitDir, nil
	}
	dir := filepath.Join(w.workDir, "kit")
	if err := copyTree(filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts"), dir); err != nil {
		return "", err
	}
	w.kitDir = dir
	return dir, nil
}

// copyTree copies a directory's files, keeping the mode each one carries.
func copyTree(from, to string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(from, entry.Name()))
		if err != nil {
			return err
		}
		target := filepath.Join(to, entry.Name())
		if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Chmod(target, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// theKitsDoorbellHasLostTheClause gives the scenario a kit whose doorbell never
// carried the clause, which is what a forge's own copy looks like before the
// installer replaces it with the kit's.
func theKitsDoorbellHasLostTheClause(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	dir, err := w.kitCopy()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "doorbell.bb")
	shipped, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lost := lostTheClause.Replace(string(shipped))
	if lost == string(shipped) {
		return fmt.Errorf("the kit's own doorbell carries no clause to lose: %s", path)
	}
	return os.WriteFile(path, []byte(lost), 0o644)
}

// theKitsCopyOfTheDoorbellCarriesItsExecutableBit gives the scenario a kit whose
// doorbell is executable: the .bb the kit ships is run directly as often as the
// wrapper is, so the installer has to carry the mode rather than guess it.
func theKitsCopyOfTheDoorbellCarriesItsExecutableBit(_ context.Context, world any, _ []string) error {
	dir, err := world.(*World).kitCopy()
	if err != nil {
		return err
	}
	for _, file := range []string{"doorbell.bb", "doorbell.sh"} {
		if err := os.Chmod(filepath.Join(dir, file), 0o755); err != nil {
			return err
		}
	}
	return nil
}

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
